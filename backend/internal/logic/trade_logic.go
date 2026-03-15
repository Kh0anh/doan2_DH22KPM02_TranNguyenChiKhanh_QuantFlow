package logic

// trade_logic.go — TradeLogic: business rules for GET /trades + GET /trades/export.
//
// ─── Cursor Encoding ──────────────────────────────────────────────────────────
//
// Cursor is a base64url-encoded string containing "<uuid>|<RFC3339>" that encodes
// the (id, executed_at) of the last record on the previous page:
//
//	encode: base64url("<uuid>|<RFC3339>")
//	decode: split on "|", parse [0] as UUID, parse [1] as time.RFC3339
//
// This compound cursor ensures stable ordering even when multiple trades share
// the same executed_at timestamp.
//
// ─── Default Date Range ───────────────────────────────────────────────────────
//
// When start_date is not provided, the default is 7 days before "now".
// When end_date is not provided, the default is "now" (time of the request).
//
// ─── CSV Streaming ────────────────────────────────────────────────────────────
//
// ExportTradesCSV streams rows directly into the provided http.ResponseWriter
// using encoding/csv (stdlib). No intermediate buffer = O(1) memory regardless
// of dataset size.
//
// Task 2.8.5 — Trade History APIs (GET /trades + GET /trades/export CSV).
// WBS: P2-Backend · 15/03/2026.
// SRS: FR-MONITOR-05; api.yaml §GET /trades, §GET /trades/export.

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/repository"
)

// ─── DTOs ─────────────────────────────────────────────────────────────────────

// TradeListInput carries validated query parameters from TradeHandler.List.
type TradeListInput struct {
	BotID     string
	Symbol    string
	Side      string
	Status    string
	StartDate *time.Time
	EndDate   *time.Time
	Cursor    string // raw cursor string from query param
	Limit     int    // validated: 1-200, default 50
}

// TradeListResult is the response envelope for GET /trades.
type TradeListResult struct {
	Data       []domain.TradeRecord `json:"data"`
	NextCursor *string              `json:"next_cursor"` // nil when has_more=false
	HasMore    bool                 `json:"has_more"`
}

// TradeExportFilter carries filter params for GET /trades/export (no cursor/limit).
type TradeExportFilter struct {
	BotID     string
	Symbol    string
	StartDate *time.Time
	EndDate   *time.Time
}

// ─── TradeLogic ───────────────────────────────────────────────────────────────

// TradeLogic implements business rules for trade history retrieval and CSV export.
type TradeLogic struct {
	tradeRepo repository.TradeRepository
}

// NewTradeLogic constructs a TradeLogic with its repository dependency.
func NewTradeLogic(tradeRepo repository.TradeRepository) *TradeLogic {
	return &TradeLogic{tradeRepo: tradeRepo}
}

// ─── ListTrades ───────────────────────────────────────────────────────────────

// ListTrades returns a cursor-paginated page of trade history records for the
// given user (WBS 2.8.5, api.yaml §GET /trades).
//
// Business rules:
//   - Default date range: last 7 days (start_date) to now (end_date).
//   - Limit is clamped to [1, 200]; default 50.
//   - Cursor encodes both UUID and executed_at (compound cursor).
//   - Uses limit+1 trick for has_more detection without a COUNT query.
//
// Return patterns:
//   - (*TradeListResult, nil) — success; data may be empty slice.
//   - (nil, error)            — 500 internal error.
func (l *TradeLogic) ListTrades(ctx context.Context, userID string, input TradeListInput) (*TradeListResult, error) {
	now := time.Now().UTC()

	// Apply default date range when not provided.
	startDate := input.StartDate
	if startDate == nil {
		sevenDaysAgo := now.Add(-7 * 24 * time.Hour)
		startDate = &sevenDaysAgo
	}
	endDate := input.EndDate
	if endDate == nil {
		endDate = &now
	}

	filter := repository.TradeFilter{
		BotID:     input.BotID,
		Symbol:    input.Symbol,
		Side:      input.Side,
		Status:    input.Status,
		StartDate: startDate,
		EndDate:   endDate,
	}

	// Parse cursor.
	var cursor *repository.TradeCursor
	if input.Cursor != "" {
		c, parseErr := decodeCursor(input.Cursor)
		if parseErr != nil {
			// Invalid cursor — treat as first page (safe degradation).
			cursor = nil
		} else {
			cursor = c
		}
	}

	// Clamp limit.
	limit := input.Limit
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := l.tradeRepo.ListByFilter(ctx, userID, filter, limit, cursor)
	if err != nil {
		return nil, fmt.Errorf("trade_logic: ListTrades: %w", err)
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	records := make([]domain.TradeRecord, len(rows))
	for i, r := range rows {
		records[i] = r.TradeRecord
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		// Parse executed_at from string for cursor encoding.
		t, parseErr := time.Parse("2006-01-02T15:04:05Z", last.ExecutedAtRaw)
		if parseErr == nil {
			encoded := encodeCursor(last.ID, t)
			nextCursor = &encoded
		}
	}

	return &TradeListResult{
		Data:       records,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// ─── ExportTradesCSV ──────────────────────────────────────────────────────────

// ExportTradesCSV streams all trade records matching the filter to w as CSV.
// CSV columns (api.yaml): ID, Bot Name, Symbol, Side, Quantity, Fill Price,
// Fee, Realized PnL, Status, Executed At.
//
// The caller (TradeHandler.Export) is responsible for setting Content-Type
// and Content-Disposition headers before calling this method.
//
// Default date range (same as ListTrades) is applied when dates are absent.
func (l *TradeLogic) ExportTradesCSV(ctx context.Context, userID string, filter TradeExportFilter, w io.Writer) error {
	now := time.Now().UTC()

	startDate := filter.StartDate
	if startDate == nil {
		sevenDaysAgo := now.Add(-7 * 24 * time.Hour)
		startDate = &sevenDaysAgo
	}
	endDate := filter.EndDate
	if endDate == nil {
		endDate = &now
	}

	repoFilter := repository.TradeFilter{
		BotID:     filter.BotID,
		Symbol:    filter.Symbol,
		StartDate: startDate,
		EndDate:   endDate,
	}

	rows, err := l.tradeRepo.ListAllByFilter(ctx, userID, repoFilter)
	if err != nil {
		return fmt.Errorf("trade_logic: ExportTradesCSV: %w", err)
	}

	writer := csv.NewWriter(w)

	// Write CSV header row.
	if err := writer.Write([]string{
		"ID", "Bot Name", "Symbol", "Side",
		"Quantity", "Fill Price", "Fee", "Realized PnL",
		"Status", "Executed At",
	}); err != nil {
		return fmt.Errorf("trade_logic: ExportTradesCSV: write header: %w", err)
	}

	// Write data rows.
	for _, r := range rows {
		record := []string{
			r.ID,
			r.BotName,
			r.Symbol,
			r.Side,
			fmt.Sprintf("%g", r.Quantity),
			fmt.Sprintf("%g", r.FillPrice),
			fmt.Sprintf("%g", r.Fee),
			fmt.Sprintf("%g", r.RealizedPnL),
			r.Status,
			r.ExecutedAt,
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("trade_logic: ExportTradesCSV: write row: %w", err)
		}
	}

	writer.Flush()
	return writer.Error()
}

// ─── Cursor helpers ───────────────────────────────────────────────────────────

// encodeCursor encodes a compound cursor (id + executedAt) as a base64url string.
// Format: base64url("<uuid>|<RFC3339>").
func encodeCursor(id string, executedAt time.Time) string {
	raw := id + "|" + executedAt.UTC().Format(time.RFC3339)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor decodes a base64url cursor string into a *TradeCursor.
// Returns an error if the string is malformed.
func decodeCursor(encoded string) (*repository.TradeCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decodeCursor: base64: %w", err)
	}

	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("decodeCursor: invalid format")
	}

	id := parts[0]
	t, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return nil, fmt.Errorf("decodeCursor: parse time: %w", err)
	}

	return &repository.TradeCursor{
		ID:         id,
		ExecutedAt: t,
	}, nil
}

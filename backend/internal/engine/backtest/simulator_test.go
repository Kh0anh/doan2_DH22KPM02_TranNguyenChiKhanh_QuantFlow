package backtest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/engine/backtest"
	"github.com/kh0anh/quantflow/internal/engine/blockly"
	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Helpers — Construct Blockly 12 JSON programmatically
// ═══════════════════════════════════════════════════════════════════════════

// buildWorkspaceJSON constructs a Blockly 12 serialization JSON.
func buildWorkspaceJSON(blocks ...map[string]interface{}) []byte {
	workspace := map[string]interface{}{
		"blocks": map[string]interface{}{
			"languageVersion": 0,
			"blocks":          blocks,
		},
	}
	data, _ := json.Marshal(workspace)
	return data
}

// makeEventOnCandle creates an event_on_candle root block.
// Body attached via "next" field (Blockly 12 nextStatement format).
func makeEventOnCandle(trigger, timeframe string, body map[string]interface{}) map[string]interface{} {
	block := map[string]interface{}{
		"type": "event_on_candle",
		"id":   "root_event_1",
		"fields": map[string]interface{}{
			"TRIGGER":   trigger,
			"TIMEFRAME": timeframe,
		},
	}
	if body != nil {
		block["next"] = map[string]interface{}{
			"block": body,
		}
	}
	return block
}

// makeTradeSmartOrder creates a trade_smart_order block with all fields.
func makeTradeSmartOrder(side, orderType string, leverage float64, marginType string, price, quantity float64) map[string]interface{} {
	return map[string]interface{}{
		"type": "trade_smart_order",
		"id":   "trade_1",
		"fields": map[string]interface{}{
			"SIDE":        side,
			"ORDER_TYPE":  orderType,
			"LEVERAGE":    leverage,
			"MARGIN_TYPE": marginType,
		},
		"inputs": map[string]interface{}{
			"PRICE": map[string]interface{}{
				"block": map[string]interface{}{
					"type": "math_number",
					"id":   "price_1",
					"fields": map[string]interface{}{
						"NUM": price,
					},
				},
			},
			"QUANTITY": map[string]interface{}{
				"block": map[string]interface{}{
					"type": "math_number",
					"id":   "qty_1",
					"fields": map[string]interface{}{
						"NUM": quantity,
					},
				},
			},
		},
	}
}

// makeTradeClosePosition creates a trade_close_position block.
func makeTradeClosePosition() map[string]interface{} {
	return map[string]interface{}{
		"type": "trade_close_position",
		"id":   "close_1",
	}
}

// chainBlocks links two statement blocks via next → block.
func chainBlocks(first, second map[string]interface{}) map[string]interface{} {
	first["next"] = map[string]interface{}{
		"block": second,
	}
	return first
}

// makeTradeSmartOrderWithShadows creates a trade_smart_order block using
// shadow blocks for PRICE and QUANTITY inputs — this is the ACTUAL format
// produced by Blockly 12 when the user edits the default placeholder values
// without dragging separate blocks into the input slots.
func makeTradeSmartOrderWithShadows(side, orderType string, leverage float64, marginType string, price, quantity float64) map[string]interface{} {
	return map[string]interface{}{
		"type": "trade_smart_order",
		"id":   "trade_shadow_1",
		"fields": map[string]interface{}{
			"SIDE":        side,
			"ORDER_TYPE":  orderType,
			"LEVERAGE":    leverage,
			"MARGIN_TYPE": marginType,
		},
		"inputs": map[string]interface{}{
			"PRICE": map[string]interface{}{
				"shadow": map[string]interface{}{
					"type": "math_number",
					"id":   "shadow_price_1",
					"fields": map[string]interface{}{
						"NUM": price,
					},
				},
			},
			"QUANTITY": map[string]interface{}{
				"shadow": map[string]interface{}{
					"type": "math_number",
					"id":   "shadow_qty_1",
					"fields": map[string]interface{}{
						"NUM": quantity,
					},
				},
			},
		},
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  Stub CandleRepository for tests (implements repository.CandleRepository)
// ═══════════════════════════════════════════════════════════════════════════

type stubCandleRepo struct {
	candles []domain.Candle
}

func (r *stubCandleRepo) FindLatest(_ context.Context, _, _ string) (*domain.Candle, error) {
	if len(r.candles) == 0 {
		return nil, nil
	}
	c := r.candles[len(r.candles)-1]
	return &c, nil
}
func (r *stubCandleRepo) InsertOne(_ context.Context, _ *domain.Candle) error    { return nil }
func (r *stubCandleRepo) InsertBatch(_ context.Context, _ []domain.Candle) error { return nil }
func (r *stubCandleRepo) QueryCandles(_ context.Context, _, _ string, _, _ *time.Time, _ int) ([]domain.Candle, error) {
	return r.candles, nil
}
func (r *stubCandleRepo) QueryLatestClosedCandles(_ context.Context, _, _ string, limit int) ([]domain.Candle, error) {
	if limit > len(r.candles) {
		limit = len(r.candles)
	}
	if limit <= 0 {
		return nil, nil
	}
	return r.candles[:limit], nil
}

// makeSampleCandles generates N sequential 1m candles.
func makeSampleCandles(n int, basePrice float64) []domain.Candle {
	candles := make([]domain.Candle, n)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		p := basePrice + float64(i)*10
		candles[i] = domain.Candle{
			Symbol:     "BTCUSDT",
			Interval:   "1m",
			OpenTime:   start.Add(time.Duration(i) * time.Minute),
			OpenPrice:  fmt.Sprintf("%.2f", p),
			HighPrice:  fmt.Sprintf("%.2f", p+5),
			LowPrice:   fmt.Sprintf("%.2f", p-5),
			ClosePrice: fmt.Sprintf("%.2f", p+2),
			Volume:     "100",
			IsClosed:   true,
		}
	}
	return candles
}

// ═══════════════════════════════════════════════════════════════════════════
//  Tests
// ═══════════════════════════════════════════════════════════════════════════

func TestParseLogicJSON_ResolvesBodyFromNext(t *testing.T) {
	tradeBlock := makeTradeSmartOrder("LONG", "MARKET", 1, "ISOLATED", 0, 100)
	rootBlock := makeEventOnCandle("ON_CANDLE_CLOSE", "1m", tradeBlock)
	logicJSON := buildWorkspaceJSON(rootBlock)

	t.Logf("Logic JSON:\n%s", string(logicJSON))

	root, err := blockly.ParseLogicJSON(logicJSON)
	if err != nil {
		t.Fatalf("ParseLogicJSON failed: %v", err)
	}

	if root.Type != "event_on_candle" {
		t.Fatalf("expected root type event_on_candle, got %s", root.Type)
	}

	trigger := blockly.GetFieldString(root, "TRIGGER")
	if trigger != "ON_CANDLE_CLOSE" {
		t.Fatalf("expected TRIGGER=ON_CANDLE_CLOSE, got %q", trigger)
	}

	// Critical test: body resolution
	body := blockly.GetBodyBlock(root)
	if body == nil {
		t.Fatal("GetBodyBlock returned nil — body NOT resolved from Next")
	}
	if body.Type != "trade_smart_order" {
		t.Fatalf("expected body type trade_smart_order, got %s", body.Type)
	}
	t.Logf("✓ GetBodyBlock resolved body from root.Next: type=%s, id=%s", body.Type, body.ID)

	// Confirm GetInputBlock("DO") returns nil
	doBlock := blockly.GetInputBlock(root, "DO")
	if doBlock != nil {
		t.Fatal("GetInputBlock(root, 'DO') should return nil for nextStatement blocks")
	}
	t.Logf("✓ GetInputBlock(root, 'DO') correctly returns nil")
}

func TestExtractEventMeta(t *testing.T) {
	rootBlock := makeEventOnCandle("ON_CANDLE_CLOSE", "15m", nil)
	logicJSON := buildWorkspaceJSON(rootBlock)

	root, err := blockly.ParseLogicJSON(logicJSON)
	if err != nil {
		t.Fatalf("ParseLogicJSON failed: %v", err)
	}

	trigger, timeframe := blockly.ExtractEventMeta(root)
	if trigger != "ON_CANDLE_CLOSE" {
		t.Fatalf("expected trigger ON_CANDLE_CLOSE, got %q", trigger)
	}
	if timeframe != "15m" {
		t.Fatalf("expected timeframe 15m, got %q", timeframe)
	}
	t.Logf("✓ ExtractEventMeta: trigger=%s, timeframe=%s", trigger, timeframe)
}

func TestSimulator_ProducesOrders(t *testing.T) {
	// Strategy: on candle close → buy 100 USDT worth of BTC MARKET LONG
	tradeBlock := makeTradeSmartOrder("LONG", "MARKET", 1, "ISOLATED", 0, 100)
	rootBlock := makeEventOnCandle("ON_CANDLE_CLOSE", "1m", tradeBlock)
	logicJSON := buildWorkspaceJSON(rootBlock)

	t.Logf("Logic JSON:\n%s", string(logicJSON))

	candles := makeSampleCandles(5, 50000)
	repo := &stubCandleRepo{candles: candles}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	sim := backtest.NewBacktestSimulator(repo, logger)

	cfg := backtest.Config{
		Symbol:         "BTCUSDT",
		Timeframe:      "1m",
		StartTime:      candles[0].OpenTime,
		EndTime:        candles[len(candles)-1].OpenTime.Add(time.Minute),
		InitialCapital: decimal.NewFromFloat(10000),
		FeeRate:        decimal.NewFromFloat(0.0004),
		MaxUnit:        1000,
	}

	var progress int32
	output, err := sim.Run(context.Background(), cfg, logicJSON, &progress)
	if err != nil {
		t.Fatalf("Simulator.Run failed: %v", err)
	}

	totalOrders := 0
	for i, exec := range output.Executions {
		t.Logf("Candle %d: orders_submitted=%d, session_error=%v",
			i, len(exec.OrdersSubmitted), exec.SessionError)
		totalOrders += len(exec.OrdersSubmitted)
	}

	if totalOrders == 0 {
		t.Fatal("FAIL: Simulator produced 0 orders — " +
			"strategy body is not being executed or trade blocks are not creating orders")
	}
	t.Logf("✓ Simulator produced %d total orders across %d sessions", totalOrders, len(output.Executions))
}

func TestOrderMatcher_FillsTrades(t *testing.T) {
	// Strategy: on candle close → buy 100 USDT worth of BTC MARKET LONG → close position
	buyBlock := makeTradeSmartOrder("LONG", "MARKET", 1, "ISOLATED", 0, 100)
	closeBlock := makeTradeClosePosition()
	chainBlocks(buyBlock, closeBlock)

	rootBlock := makeEventOnCandle("ON_CANDLE_CLOSE", "1m", buyBlock)
	logicJSON := buildWorkspaceJSON(rootBlock)

	t.Logf("Logic JSON:\n%s", string(logicJSON))

	candles := makeSampleCandles(10, 50000)
	repo := &stubCandleRepo{candles: candles}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	sim := backtest.NewBacktestSimulator(repo, logger)

	cfg := backtest.Config{
		Symbol:         "BTCUSDT",
		Timeframe:      "1m",
		StartTime:      candles[0].OpenTime,
		EndTime:        candles[len(candles)-1].OpenTime.Add(time.Minute),
		InitialCapital: decimal.NewFromFloat(10000),
		FeeRate:        decimal.NewFromFloat(0.0004),
		MaxUnit:        1000,
	}

	var progress int32
	output, err := sim.Run(context.Background(), cfg, logicJSON, &progress)
	if err != nil {
		t.Fatalf("Simulator.Run failed: %v", err)
	}

	totalOrders := 0
	for i, exec := range output.Executions {
		t.Logf("Candle %d: orders=%d error=%v", i, len(exec.OrdersSubmitted), exec.SessionError)
		for j, order := range exec.OrdersSubmitted {
			t.Logf("  Order %d: side=%s type=%s qty=%s reduceOnly=%v",
				j, order.Side, order.OrderType, order.Quantity.String(), order.IsReduceOnly)
		}
		totalOrders += len(exec.OrdersSubmitted)
	}
	t.Logf("Total orders submitted: %d", totalOrders)

	if totalOrders == 0 {
		t.Fatal("FAIL: No orders submitted by simulator")
	}

	// Run order matcher
	matcher := backtest.NewOrderMatcher(cfg.FeeRate, logger)
	result, err := matcher.Match(context.Background(), output)
	if err != nil {
		t.Fatalf("OrderMatcher.Match failed: %v", err)
	}

	t.Logf("Trades filled: %d", len(result.Trades))
	t.Logf("Final balance: %s", result.FinalBalance.String())

	for i, trade := range result.Trades {
		t.Logf("Trade %d: side=%s entry=%s exit=%s qty=%s pnl=%s",
			i, trade.Side, trade.EntryPrice.String(), trade.ExitPrice.String(),
			trade.Quantity.String(), trade.PnL.String())
	}

	if len(result.Trades) == 0 {
		t.Fatal("FAIL: OrderMatcher produced 0 trades despite orders being submitted")
	}

	t.Logf("✓ Full pipeline produced %d trades", len(result.Trades))
}

// TestShadowBlocks_ProduceTrades verifies the full pipeline works when the
// strategy JSON uses shadow blocks (the ACTUAL format produced by Blockly 12
// when users edit the default placeholder values in input slots).
func TestShadowBlocks_ProduceTrades(t *testing.T) {
	// Build strategy using shadow blocks — exactly as the frontend serializes it
	buyBlock := makeTradeSmartOrderWithShadows("LONG", "MARKET", 1, "ISOLATED", 0, 100)
	closeBlock := makeTradeClosePosition()
	chainBlocks(buyBlock, closeBlock)

	rootBlock := makeEventOnCandle("ON_CANDLE_CLOSE", "1m", buyBlock)
	logicJSON := buildWorkspaceJSON(rootBlock)

	t.Logf("Logic JSON (with shadows):\n%s", string(logicJSON))

	candles := makeSampleCandles(10, 50000)
	repo := &stubCandleRepo{candles: candles}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	sim := backtest.NewBacktestSimulator(repo, logger)

	cfg := backtest.Config{
		Symbol:         "BTCUSDT",
		Timeframe:      "1m",
		StartTime:      candles[0].OpenTime,
		EndTime:        candles[len(candles)-1].OpenTime.Add(time.Minute),
		InitialCapital: decimal.NewFromFloat(10000),
		FeeRate:        decimal.NewFromFloat(0.0004),
		MaxUnit:        1000,
	}

	var progress int32
	output, err := sim.Run(context.Background(), cfg, logicJSON, &progress)
	if err != nil {
		t.Fatalf("Simulator.Run failed: %v", err)
	}

	totalOrders := 0
	for _, exec := range output.Executions {
		totalOrders += len(exec.OrdersSubmitted)
	}
	t.Logf("Total orders submitted (with shadow blocks): %d", totalOrders)

	if totalOrders == 0 {
		t.Fatal("FAIL: Shadow blocks produced 0 orders — GetInputBlock does not fall back to Shadow")
	}

	matcher := backtest.NewOrderMatcher(cfg.FeeRate, logger)
	result, err := matcher.Match(context.Background(), output)
	if err != nil {
		t.Fatalf("OrderMatcher.Match failed: %v", err)
	}

	if len(result.Trades) == 0 {
		t.Fatal("FAIL: Shadow block pipeline produced 0 trades")
	}

	t.Logf("✓ Shadow block pipeline produced %d trades", len(result.Trades))
}

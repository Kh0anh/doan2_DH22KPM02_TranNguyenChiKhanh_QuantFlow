package logic

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/repository"
)

// listStrategiesDefaults mirrors the defaults declared in api.yaml §GET /strategies.
const (
	defaultStrategyPage  = 1
	defaultStrategyLimit = 20
	maxStrategyLimit     = 100

	// eventTriggerBlockType is the canonical Blockly block type name for the
	// Event Trigger block (blockly.md §3.1.1). Every valid strategy must contain
	// at least one block of this type at the top level of blocks.blocks[].
	eventTriggerBlockType = "event_on_candle"
)

// Sentinel errors for POST /strategies — mapped to specific HTTP codes by the handler.
var (
	// ErrMissingEventTrigger is returned when logic_json contains no top-level
	// event_on_candle block. Handler maps this to 400 MISSING_EVENT_TRIGGER.
	ErrMissingEventTrigger = errors.New("strategy must contain an event_on_candle block")

	// ErrInvalidJSONStructure is returned when logic_json cannot be parsed or its
	// structure does not conform to the expected Blockly JSON shape.
	// Handler maps this to 400 INVALID_JSON_STRUCTURE.
	ErrInvalidJSONStructure = errors.New("logic_json has invalid or unexpected structure")
)

// ListStrategiesInput carries the validated query parameters for list strategies.
type ListStrategiesInput struct {
	// Page is 1-based page number. Clamped to ≥ 1 in NewListStrategiesInput.
	Page int
	// Limit is the number of records per page. Clamped to [1, 100].
	Limit int
	// Search is the case-insensitive name filter (ILIKE). Empty = no filter.
	Search string
}

// NewListStrategiesInput constructs a ListStrategiesInput with defaults applied
// for zero values and clamping applied to out-of-range values.
func NewListStrategiesInput(page, limit int, search string) ListStrategiesInput {
	if page < 1 {
		page = defaultStrategyPage
	}
	if limit < 1 {
		limit = defaultStrategyLimit
	} else if limit > maxStrategyLimit {
		limit = maxStrategyLimit
	}
	return ListStrategiesInput{
		Page:   page,
		Limit:  limit,
		Search: strings.TrimSpace(search),
	}
}

// PagePagination is the standard page-based pagination envelope returned by
// GET /strategies, matching api.yaml §PagePagination.
type PagePagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// ListStrategiesOutput is the data returned by StrategyLogic.ListStrategies.
type ListStrategiesOutput struct {
	Data       []domain.StrategySummary `json:"data"`
	Pagination PagePagination           `json:"pagination"`
}

// StrategyLogic encapsulates business rules for strategy management (WBS 2.3.x).
type StrategyLogic struct {
	repo repository.StrategyRepository
}

// NewStrategyLogic constructs a StrategyLogic.
func NewStrategyLogic(repo repository.StrategyRepository) *StrategyLogic {
	return &StrategyLogic{repo: repo}
}

// ListStrategies retrieves a paginated, optionally searched list of strategies
// for the given user (WBS 2.3.1, api.yaml §GET /strategies).
//
// Business rules:
//   - page defaults to 1 when ≤ 0; limit defaults to 20, max 100.
//   - search is matched case-insensitively (ILIKE) against strategy name.
//   - Results are ordered by updated_at DESC (most recently modified first).
//   - version in each StrategySummary reflects the latest version_number from
//     the strategy_versions table at query time.
func (l *StrategyLogic) ListStrategies(ctx context.Context, userID string, input ListStrategiesInput) (*ListStrategiesOutput, error) {
	summaries, total, err := l.repo.ListWithPagination(ctx, userID, input.Search, input.Page, input.Limit)
	if err != nil {
		return nil, err
	}

	// Ensure a non-nil slice so the JSON response encodes as [] not null.
	if summaries == nil {
		summaries = []domain.StrategySummary{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(input.Limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	return &ListStrategiesOutput{
		Data: summaries,
		Pagination: PagePagination{
			Page:       input.Page,
			Limit:      input.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

// CreateStrategyInput is the internal DTO passed from StrategyHandler to StrategyLogic.
type CreateStrategyInput struct {
	// Name is the human-readable strategy name (required, non-blank).
	Name string
	// LogicJSON is the raw Blockly JSON payload exactly as received from the
	// client request body. It is stored as-is into strategy_versions.logic_json.
	LogicJSON json.RawMessage
	// Status is the desired initial status. Defaults to "Draft" when blank or
	// not one of the recognised values (Draft / Valid).
	Status string
}

// CreateStrategy implements the POST /strategies business flow (WBS 2.3.2,
// api.yaml §POST /strategies, SRS FR-DESIGN-11):
//
//  1. Validate name is non-blank.
//  2. Validate logic_json parses correctly and contains an event_on_candle block
//     at the top level of blocks.blocks[] (SRS FR-DESIGN-03).
//  3. Normalise status to "Draft" when omitted or unrecognised.
//  4. Atomically insert strategy + strategy_version (version_number=1).
//  5. Return StrategyCreated DTO.
//
// Return patterns:
//   - (*StrategyCreated, nil)            — success → HTTP 201.
//   - (nil, ErrInvalidJSONStructure)     — malformed JSON → HTTP 400.
//   - (nil, ErrMissingEventTrigger)      — no event block → HTTP 400.
//   - (nil, other)                       — unexpected server error → HTTP 500.
func (l *StrategyLogic) CreateStrategy(ctx context.Context, userID string, input CreateStrategyInput) (*domain.StrategyCreated, error) {
	// 1. Name must be non-blank.
	if strings.TrimSpace(input.Name) == "" {
		return nil, ErrInvalidJSONStructure
	}

	// 2. Validate logic_json structure and presence of event_on_candle block.
	found, err := hasEventTriggerBlock(input.LogicJSON)
	if err != nil {
		return nil, ErrInvalidJSONStructure
	}
	if !found {
		return nil, ErrMissingEventTrigger
	}

	// 3. Normalise status.
	status := input.Status
	if status != domain.StrategyStatusDraft && status != domain.StrategyStatusValid {
		status = domain.StrategyStatusDraft
	}

	// 4. Build entities and persist.
	strategy := &domain.Strategy{
		UserID: userID,
		Name:   strings.TrimSpace(input.Name),
		Status: status,
	}
	version := &domain.StrategyVersion{
		VersionNumber: 1,
		LogicJSON:     []byte(input.LogicJSON),
		Status:        status,
	}

	if err := l.repo.Create(ctx, strategy, version); err != nil {
		return nil, err
	}

	// 5. Project to response DTO.
	return &domain.StrategyCreated{
		ID:        strategy.ID,
		Name:      strategy.Name,
		Version:   1,
		Status:    strategy.Status,
		CreatedAt: strategy.CreatedAt,
	}, nil
}

// hasEventTriggerBlock parses a Blockly JSON payload and reports whether it
// contains at least one top-level block of type "event_on_candle".
//
// Expected Blockly JSON shape (blockly.md §3.1.1):
//
//	{
//	  "blocks": {
//	    "languageVersion": 0,
//	    "blocks": [ { "type": "event_on_candle", ... }, ... ]
//	  }
//	}
//
// Returns:
//   - (true,  nil)  — event block present.
//   - (false, nil)  — valid JSON but no event block found.
//   - (false, err)  — JSON is malformed or missing required keys.
func hasEventTriggerBlock(logicJSON []byte) (bool, error) {
	if len(logicJSON) == 0 {
		return false, ErrInvalidJSONStructure
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(logicJSON, &root); err != nil {
		return false, err
	}

	blocksOuter, ok := root["blocks"]
	if !ok {
		return false, ErrInvalidJSONStructure
	}

	var blocksWrapper map[string]json.RawMessage
	if err := json.Unmarshal(blocksOuter, &blocksWrapper); err != nil {
		return false, ErrInvalidJSONStructure
	}

	blocksArray, ok := blocksWrapper["blocks"]
	if !ok {
		// No blocks key means an empty workspace — no event trigger.
		return false, nil
	}

	var topBlocks []map[string]json.RawMessage
	if err := json.Unmarshal(blocksArray, &topBlocks); err != nil {
		return false, ErrInvalidJSONStructure
	}

	for _, block := range topBlocks {
		rawType, ok := block["type"]
		if !ok {
			continue
		}
		var blockType string
		if err := json.Unmarshal(rawType, &blockType); err != nil {
			continue
		}
		if blockType == eventTriggerBlockType {
			return true, nil
		}
	}

	return false, nil
}

package logic

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

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

	// ErrStrategyNotFound is returned when the requested strategy does not exist
	// or does not belong to the authenticated user. Handler maps this to 404.
	ErrStrategyNotFound = errors.New("strategy not found")

	// ErrStrategyInUse is returned when DELETE is attempted but Running bots
	// reference the strategy. Handler maps this to 409 STRATEGY_IN_USE.
	ErrStrategyInUse = errors.New("strategy is in use by running bots")
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

// GetStrategy retrieves the full detail of a single strategy for the given user
// (WBS 2.3.3, api.yaml §GET /strategies/{id}).
//
// Business rules:
//   - Returns ErrStrategyNotFound when the strategy does not exist or belongs
//     to a different user (ownership enforced at repository layer).
//   - Appends a warning message and populates active_bot_ids when the strategy
//     is referenced by one or more bot_instances with status=Running.
//
// Return patterns:
//   - (*StrategyDetail, nil)         — found, no active bots.
//   - (*StrategyDetail, nil)         — found, warning + active_bot_ids populated.
//   - (nil, ErrStrategyNotFound)     — not found or not owned → HTTP 404.
//   - (nil, other)                   — unexpected server error → HTTP 500.
func (l *StrategyLogic) GetStrategy(ctx context.Context, userID, strategyID string) (*domain.StrategyDetail, error) {
	detail, err := l.repo.FindByID(ctx, strategyID, userID)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, ErrStrategyNotFound
	}

	// Populate warning when active (Running) bots reference this strategy.
	if len(detail.ActiveBotIDs) > 0 {
		msg := "This strategy is being used by running Bot(s). Any changes will only apply to new runs."
		detail.Warning = &msg
	}

	return detail, nil
}

// UpdateStrategyInput is the internal DTO passed from StrategyHandler to StrategyLogic
// for PUT /strategies/{id} (WBS 2.3.4).
// All fields are optional — omitted fields retain their current database values.
type UpdateStrategyInput struct {
	// Name is the new strategy name. Empty string = keep existing.
	Name string
	// LogicJSON is the updated Blockly JSON payload. nil/empty = keep existing.
	// When provided it must contain a valid event_on_candle block.
	LogicJSON json.RawMessage
	// Status is the desired new status (Draft|Valid). Empty string = keep existing.
	Status string
}

// UpdateStrategy implements PUT /strategies/{id} (WBS 2.3.4,
// api.yaml §PUT /strategies/{id}, SRS FR-DESIGN-11).
//
// Business rules:
//  1. Validate logic_json when provided — must contain an event_on_candle block.
//  2. Delegate to repo.Update (atomic: lock → nextVersion → UPDATE → INSERT version).
//  3. Map nil result to ErrStrategyNotFound (ownership check at DB layer).
//  4. Append warning when Running bots reference this strategy (non-blocking).
//
// Return patterns:
//   - (*StrategyUpdated, nil)       — success.
//   - (nil, ErrStrategyNotFound)    — strategy not found or not owned → 404.
//   - (nil, ErrMissingEventTrigger) — no event_on_candle block → 400.
//   - (nil, ErrInvalidJSONStructure)— malformed logic_json → 400.
//   - (nil, other)                  — unexpected server error → 500.
func (l *StrategyLogic) UpdateStrategy(ctx context.Context, userID, strategyID string, input UpdateStrategyInput) (*domain.StrategyUpdated, error) {
	// Validate logic_json if provided.
	if len(input.LogicJSON) > 0 {
		found, err := hasEventTriggerBlock(input.LogicJSON)
		if err != nil {
			return nil, ErrInvalidJSONStructure
		}
		if !found {
			return nil, ErrMissingEventTrigger
		}
	}

	// Normalise status — only Draft/Valid are allowed; anything else is cleared
	// so the repository carries forward the current DB value.
	if input.Status != domain.StrategyStatusDraft && input.Status != domain.StrategyStatusValid {
		input.Status = ""
	}

	result, err := l.repo.Update(ctx, strategyID, userID,
		strings.TrimSpace(input.Name),
		input.Status,
		[]byte(input.LogicJSON),
	)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrStrategyNotFound
	}

	// Check for Running bots and attach warning (non-blocking).
	// FindByID is cheap: it only does two indexed queries and carries the
	// active_bot_ids already built by the repository (WBS 2.3.3 reuse).
	if detail, ferr := l.repo.FindByID(ctx, strategyID, userID); ferr == nil && detail != nil && len(detail.ActiveBotIDs) > 0 {
		msg := "This strategy is being used by running Bot(s). Any changes will only apply to new runs."
		result.Warning = &msg
	}

	return result, nil
}

// DeleteStrategy implements DELETE /strategies/{id} (WBS 2.3.5,
// api.yaml §DELETE /strategies/{id}, SRS FR-DESIGN-11).
//
// Business rules:
//  1. Check for Running bot_instances linked to strategy — block with 409 if found.
//  2. Delete strategy (CASCADE removes strategy_versions rows).
//  3. 404 when strategy does not exist or belongs to another user.
//
// Return patterns:
//   - (nil, nil)                    — deleted successfully → HTTP 200.
//   - (activeBotIDs, ErrStrategyInUse) — Running bots block deletion → HTTP 409.
//   - (nil, ErrStrategyNotFound)    — not found or not owned → HTTP 404.
//   - (nil, other)                  — unexpected server error → HTTP 500.
func (l *StrategyLogic) DeleteStrategy(ctx context.Context, userID, strategyID string) ([]string, error) {
	botIDs, err := l.repo.DeleteByID(ctx, strategyID, userID)
	if err != nil {
		// DeleteByID returns errStrategyNotFoundRepo (unexported) — match by message
		// to avoid coupling the logic layer to a repo-internal sentinel.
		if err.Error() == "strategy not found" {
			return nil, ErrStrategyNotFound
		}
		return nil, err
	}
	if len(botIDs) > 0 {
		return botIDs, ErrStrategyInUse
	}
	return nil, nil
}

// ImportStrategyInput is the internal DTO passed from StrategyHandler to
// StrategyLogic for POST /strategies/import (WBS 2.3.6).
type ImportStrategyInput struct {
	// Name is the human-readable strategy name (required, non-blank).
	Name string
	// LogicJSON is the raw Blockly JSON payload from the imported file.
	// Must conform to the BlocklyLogicJson schema and contain an event_on_candle block.
	LogicJSON json.RawMessage
}

// ImportStrategy implements POST /strategies/import (WBS 2.3.6,
// api.yaml §POST /strategies/import, SRS FR-DESIGN-13).
//
// Business rules:
//  1. Validate name is non-blank.
//  2. Validate logic_json structure and event_on_candle presence.
//  3. Fixed status = "Valid" — import implies a well-formed, ready-to-use strategy.
//  4. Atomically persist strategy + version_number=1 via repo.Create (reused).
//
// All validation failures collapse to ErrInvalidJSONStructure so the handler
// maps everything to a single 400 INVALID_JSON_STRUCTURE per api.yaml spec.
//
// Return patterns:
//   - (*StrategyCreated, nil)         — success → HTTP 201.
//   - (nil, ErrInvalidJSONStructure)  — malformed JSON or missing required keys → HTTP 400.
//   - (nil, ErrMissingEventTrigger)   — no event_on_candle block → HTTP 400 (same code).
//   - (nil, other)                    — unexpected server error → HTTP 500.
func (l *StrategyLogic) ImportStrategy(ctx context.Context, userID string, input ImportStrategyInput) (*domain.StrategyCreated, error) {
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

	// 3. Fixed status = "Valid" for all imported strategies.
	const importStatus = domain.StrategyStatusValid

	// 4. Build entities and persist.
	strategy := &domain.Strategy{
		UserID: userID,
		Name:   strings.TrimSpace(input.Name),
		Status: importStatus,
	}
	version := &domain.StrategyVersion{
		VersionNumber: 1,
		LogicJSON:     []byte(input.LogicJSON),
		Status:        importStatus,
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

// ExportStrategy implements GET /strategies/{id}/export (WBS 2.3.7,
// api.yaml §GET /strategies/{id}/export, SRS FR-DESIGN-12).
//
// Business rules:
//  1. Fetch strategy detail via repo.FindByID (reused — returns latest logic_json).
//  2. Map StrategyDetail → domain.StrategyExport.
//  3. Set ExportedAt = time.Now().UTC() at call time (not a DB field).
//
// Return patterns:
//   - (*StrategyExport, nil)      — success → HTTP 200 with Content-Disposition header.
//   - (nil, ErrStrategyNotFound)  — not found or not owned → HTTP 404.
//   - (nil, other)                — unexpected server error → HTTP 500.
func (l *StrategyLogic) ExportStrategy(ctx context.Context, userID, strategyID string) (*domain.StrategyExport, error) {
	detail, err := l.repo.FindByID(ctx, strategyID, userID)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, ErrStrategyNotFound
	}

	return &domain.StrategyExport{
		Name:       detail.Name,
		LogicJSON:  detail.LogicJSON,
		Version:    detail.Version,
		ExportedAt: time.Now().UTC(),
	}, nil
}

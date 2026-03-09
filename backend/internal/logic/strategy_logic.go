package logic

import (
	"context"
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

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/kh0anh/quantflow/internal/domain"
	"gorm.io/gorm"
)

// ApiKeyRepository defines the data-access contract for the api_keys table.
// Methods are kept minimal — only what WBS 2.2.1 requires.
// GET (2.2.2) and DELETE (2.2.3) methods will be added in their respective tasks.
type ApiKeyRepository interface {
	// FindByUserID returns the API key record for the given user UUID, or
	// (nil, nil) when no configuration exists yet.
	FindByUserID(ctx context.Context, userID string) (*domain.APIKey, error)

	// Upsert persists an APIKey record. When apiKey.ID is non-empty GORM issues
	// an UPDATE (overwrite existing row); an empty ID triggers INSERT and GORM
	// back-fills the generated UUID from the PostgreSQL gen_random_uuid() default.
	Upsert(ctx context.Context, apiKey *domain.APIKey) error
}

type apiKeyRepository struct {
	db *gorm.DB
}

// NewApiKeyRepository constructs a GORM-backed ApiKeyRepository.
func NewApiKeyRepository(db *gorm.DB) ApiKeyRepository {
	return &apiKeyRepository{db: db}
}

// FindByUserID retrieves the API key record for the given user UUID.
// Returns (nil, nil) when no record is found (caller must handle this case).
func (r *apiKeyRepository) FindByUserID(ctx context.Context, userID string) (*domain.APIKey, error) {
	var apiKey domain.APIKey
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&apiKey).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("api_key_repo: FindByUserID: %w", err)
	}
	return &apiKey, nil
}

// Upsert persists the APIKey entity using GORM db.Save().
//   - Non-empty ID  → UPDATE all columns + autoUpdateTime.
//   - Empty ID      → INSERT; PostgreSQL fills ID via gen_random_uuid() default.
func (r *apiKeyRepository) Upsert(ctx context.Context, apiKey *domain.APIKey) error {
	if err := r.db.WithContext(ctx).Save(apiKey).Error; err != nil {
		return fmt.Errorf("api_key_repo: Upsert: %w", err)
	}
	return nil
}

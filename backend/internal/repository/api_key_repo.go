package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/kh0anh/quantflow/internal/domain"
	"gorm.io/gorm"
)

// ApiKeyRepository defines the data-access contract for the api_keys table.
type ApiKeyRepository interface {
	// FindByUserID returns the API key record for the given user UUID, or
	// (nil, nil) when no configuration exists yet.
	FindByUserID(ctx context.Context, userID string) (*domain.APIKey, error)

	// Upsert persists an APIKey record. When apiKey.ID is non-empty GORM issues
	// an UPDATE (overwrite existing row); an empty ID triggers INSERT and GORM
	// back-fills the generated UUID from the PostgreSQL gen_random_uuid() default.
	Upsert(ctx context.Context, apiKey *domain.APIKey) error

	// HasRunningBotsByAPIKeyID reports whether any bot_instance linked to the
	// given api_key_id is currently in Running status. Used by DeleteApiKey to
	// enforce the 409 constraint (WBS 2.2.3, api.yaml §DELETE /exchange/api-keys).
	// A direct COUNT on bot_instances is used here to avoid pulling in the full
	// BotRepository infrastructure (WBS 2.7.x) ahead of schedule.
	HasRunningBotsByAPIKeyID(ctx context.Context, apiKeyID string) (bool, error)

	// DeleteByUserID removes the API key record for the given user.
	// No-op when no record exists (idempotent — api.yaml specifies no 404).
	DeleteByUserID(ctx context.Context, userID string) error
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

// HasRunningBotsByAPIKeyID counts bot_instance rows linked to the given API key
// that are currently in Running status. Returns true when count > 0.
func (r *apiKeyRepository) HasRunningBotsByAPIKeyID(ctx context.Context, apiKeyID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("bot_instances").
		Where("api_key_id = ? AND status = ?", apiKeyID, "Running").
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("api_key_repo: HasRunningBotsByAPIKeyID: %w", err)
	}
	return count > 0, nil
}

// DeleteByUserID hard-deletes the API key record for the given user UUID.
// Idempotent: no error when no record exists (RowsAffected may be 0).
func (r *apiKeyRepository) DeleteByUserID(ctx context.Context, userID string) error {
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&domain.APIKey{}).Error
	if err != nil {
		return fmt.Errorf("api_key_repo: DeleteByUserID: %w", err)
	}
	return nil
}

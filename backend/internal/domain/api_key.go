package domain

import (
	"strings"
	"time"
)

// API key status constants — matches the allowed values documented in
// api_keys.status (VARCHAR 20). Never use raw strings in business logic.
const (
	APIKeyStatusConnected = "Connected"
	APIKeyStatusRevoked   = "Revoked"
)

// APIKey maps to the `api_keys` table in PostgreSQL (Database Schema §2).
//
// Security contract (SRS FR-CORE-01, NFR-SEC-01):
//   - SecretKeyEncrypted stores only the AES-256-GCM ciphertext; plain-text
//     secret is NEVER persisted and NEVER logged.
//   - json:"-" tags on sensitive fields prevent accidental marshaling.
type APIKey struct {
	ID                 string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"-"`
	UserID             string    `gorm:"type:uuid;not null"                             json:"-"`
	Exchange           string    `gorm:"type:varchar(50);not null"                      json:"exchange"`
	ApiKey             string    `gorm:"type:varchar(255);not null"                     json:"-"`
	SecretKeyEncrypted string    `gorm:"type:varchar(512);not null"                     json:"-"`
	Status             string    `gorm:"type:varchar(20);not null;default:'Active'"     json:"status"`
	CreatedAt          time.Time `gorm:"not null;autoCreateTime"                        json:"created_at"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime"                                 json:"updated_at"`
}

// TableName overrides the default GORM table name.
func (APIKey) TableName() string { return "api_keys" }

// ApiKeyInfo is the read-only response DTO returned by /exchange/api-keys
// endpoints. It deliberately excludes ApiKey (plain), SecretKeyEncrypted, and
// UserID in compliance with the Write-Only secret key policy (api.yaml §Exchange).
type ApiKeyInfo struct {
	ID           string    `json:"id"`
	Exchange     string    `json:"exchange"`
	ApiKeyMasked string    `json:"api_key_masked"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ToInfo projects an APIKey entity into its public-safe DTO.
// The api_key is masked; secret_key is never included (Write-Only).
func (k *APIKey) ToInfo() ApiKeyInfo {
	return ApiKeyInfo{
		ID:           k.ID,
		Exchange:     k.Exchange,
		ApiKeyMasked: MaskAPIKey(k.ApiKey),
		Status:       k.Status,
		CreatedAt:    k.CreatedAt,
		UpdatedAt:    k.UpdatedAt,
	}
}

// MaskAPIKey returns the key with all but the final 4 characters replaced by
// asterisks, matching the masking convention in api.yaml:
//
//	"vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A"
//	→ "****************************Eh8A"
func MaskAPIKey(key string) string {
	const visibleSuffix = 4
	if len(key) <= visibleSuffix {
		return strings.Repeat("*", len(key))
	}
	return strings.Repeat("*", len(key)-visibleSuffix) + key[len(key)-visibleSuffix:]
}

package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/exchange"
	"github.com/kh0anh/quantflow/internal/repository"
	pkgcrypto "github.com/kh0anh/quantflow/pkg/crypto"
)

// Sentinel errors exposed to ApiKeyHandler for precise HTTP response mapping.
var (
	// ErrInvalidKeyFormat is returned when api_key or secret_key is blank after
	// trimming. HTTP handler maps this to 400 INVALID_KEY_FORMAT.
	ErrInvalidKeyFormat = errors.New("api_key and secret_key must not be empty")

	// ErrExchangeValidationFailed is returned when Binance rejects the provided
	// key pair — wrong key, IP whitelist mismatch, or missing Futures Trading
	// permission. HTTP handler maps this to 422 EXCHANGE_VALIDATION_FAILED.
	ErrExchangeValidationFailed = errors.New("binance rejected the api key or insufficient permissions")
)

// SaveApiKeyInput is the internal DTO passed from ApiKeyHandler to ApiKeyLogic.
type SaveApiKeyInput struct {
	// Exchange is the name of the CEX (defaults to "Binance" when blank).
	Exchange string
	// ApiKey is the plain-text Binance Access Key (public, non-secret).
	ApiKey string
	// SecretKey is the plain-text Binance Secret Key. It lives in RAM only
	// during Binance ping-verify and AES-256-GCM encryption; never persisted
	// as plain-text and never logged (SRS FR-CORE-01, NFR-SEC-01).
	SecretKey string
}

// ApiKeyLogic orchestrates exchange key management business rules (WBS 2.2.1).
type ApiKeyLogic struct {
	repo   repository.ApiKeyRepository
	aesKey []byte // 32-byte AES-256 key produced by pkgcrypto.DeriveKey
}

// NewApiKeyLogic constructs an ApiKeyLogic.
// aesKey must be exactly 32 bytes — use pkgcrypto.DeriveKey(cfg.AESKey).
func NewApiKeyLogic(repo repository.ApiKeyRepository, aesKey []byte) *ApiKeyLogic {
	return &ApiKeyLogic{repo: repo, aesKey: aesKey}
}

// SaveApiKey implements the full POST /exchange/api-keys business flow
// described in WBS 2.2.1 and api.yaml §/exchange/api-keys:
//
//  1. Validate format — api_key and secret_key must be non-empty.
//  2. Ping-verify via Binance Futures — NewGetAccountService().Do(ctx).
//     Any error (wrong key / IP block / missing Futures permission) → ErrExchangeValidationFailed.
//  3. AES-256-GCM encrypt the secret_key before any DB write.
//  4. Upsert: overwrite the existing record for this user, or insert a new one.
//
// Return patterns:
//   - (*APIKey, nil)                      — success; caller builds 201 response.
//   - (nil, ErrInvalidKeyFormat)          — → HTTP 400.
//   - (nil, ErrExchangeValidationFailed)  — → HTTP 422.
//   - (nil, other)                        — unexpected server error → HTTP 500.
func (l *ApiKeyLogic) SaveApiKey(ctx context.Context, userID string, input SaveApiKeyInput) (*domain.APIKey, error) {
	// 1. Format guard (handler already trims, but enforce the contract here too).
	if strings.TrimSpace(input.ApiKey) == "" || strings.TrimSpace(input.SecretKey) == "" {
		return nil, ErrInvalidKeyFormat
	}

	// 2. Binance ping-verify.
	//    go-binance NewGetAccountService() performs a signed HMAC-SHA256 request;
	//    any API-level rejection is surfaced as a non-nil error.
	//    The SecretKey is in RAM only during this call — never written to logs.
	client := exchange.NewFuturesClient(input.ApiKey, input.SecretKey)
	if _, err := client.NewGetAccountService().Do(ctx); err != nil {
		return nil, ErrExchangeValidationFailed
	}

	// 3. AES-256-GCM encrypt the secret_key before persistence (NFR-SEC-01).
	encrypted, err := pkgcrypto.Encrypt([]byte(input.SecretKey), l.aesKey)
	if err != nil {
		return nil, fmt.Errorf("api_key_logic: SaveApiKey: encrypt secret: %w", err)
	}

	// 4. Upsert — one api_key record per user (api.yaml: "ghi đè cặp Key cũ").
	existing, err := l.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("api_key_logic: SaveApiKey: %w", err)
	}

	var record *domain.APIKey
	if existing != nil {
		// Overwrite all mutable fields on the existing record.
		existing.Exchange = input.Exchange
		existing.ApiKey = input.ApiKey
		existing.SecretKeyEncrypted = encrypted
		existing.Status = domain.APIKeyStatusConnected
		record = existing
	} else {
		record = &domain.APIKey{
			UserID:             userID,
			Exchange:           input.Exchange,
			ApiKey:             input.ApiKey,
			SecretKeyEncrypted: encrypted,
			Status:             domain.APIKeyStatusConnected,
		}
	}

	if err := l.repo.Upsert(ctx, record); err != nil {
		return nil, fmt.Errorf("api_key_logic: SaveApiKey: %w", err)
	}
	return record, nil
}

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

	// ErrActiveBotsExist is returned when one or more bot_instances linked to the
	// user's API key are still in Running status. HTTP handler maps this to
	// 409 ACTIVE_BOTS_EXIST (api.yaml §DELETE /exchange/api-keys, WBS 2.2.3).
	ErrActiveBotsExist = errors.New("active bots are using this api key")

	// ErrAPIKeyNotConfigured is returned when the user has no api_key record in
	// the database yet. Callers map this to a user-facing error (e.g. 400) or
	// use it as a bot-start guard (WBS 2.2.4, 2.7.1).
	ErrAPIKeyNotConfigured = errors.New("no exchange api key configured for this user")
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

// ApiKeyLogic orchestrates exchange key management business rules (WBS 2.2.1–2.2.5).
type ApiKeyLogic struct {
	repo    repository.ApiKeyRepository
	aesKey  []byte                        // 32-byte AES-256 key from pkgcrypto.DeriveKey
	limiter *exchange.ExchangeRateLimiter // singleton shared across all BinanceProxy instances
	baseURL string                        // Binance Futures REST base URL override (empty = library default)
}

// NewApiKeyLogic constructs an ApiKeyLogic.
//   - aesKey must be exactly 32 bytes — use pkgcrypto.DeriveKey(cfg.AESKey).
//   - limiter is the singleton ExchangeRateLimiter created in router.go and
//     shared by every BinanceProxy built via BuildProxy (WBS 2.2.5).
//   - baseURL overrides the Binance Futures REST base URL (e.g. testnet).
//     Pass an empty string to use the go-binance library default.
func NewApiKeyLogic(repo repository.ApiKeyRepository, aesKey []byte, limiter *exchange.ExchangeRateLimiter, baseURL string) *ApiKeyLogic {
	return &ApiKeyLogic{repo: repo, aesKey: aesKey, limiter: limiter, baseURL: baseURL}
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
	client := exchange.NewFuturesClient(input.ApiKey, input.SecretKey, l.baseURL)
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

// GetApiKey retrieves the current API key configuration for the given user and
// returns its public-safe representation (WBS 2.2.2).
//
// The SecretKeyEncrypted field is NEVER accessed or returned — only the masked
// ApiKey and status are surfaced via domain.ApiKeyInfo (NFR-SEC-01).
//
// Return patterns:
//   - (*ApiKeyInfo, nil) — configured; caller renders 200 with data payload.
//   - (nil, nil)         — not yet configured; caller renders 200 with data:null.
//   - (nil, err)         — unexpected DB error → HTTP 500.
func (l *ApiKeyLogic) GetApiKey(ctx context.Context, userID string) (*domain.ApiKeyInfo, error) {
	record, err := l.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("api_key_logic: GetApiKey: %w", err)
	}
	if record == nil {
		return nil, nil
	}
	info := record.ToInfo()
	return &info, nil
}

// DeleteApiKey enforces the running-bot constraint and removes the API key
// configuration for the given user (WBS 2.2.3, api.yaml §DELETE /exchange/api-keys).
//
// Flow:
//  1. FindByUserID — if no record exists, return nil (idempotent; no 404).
//  2. HasRunningBotsByAPIKeyID — any Running bot blocks deletion → ErrActiveBotsExist.
//  3. DeleteByUserID — hard-delete the record including secret_key_encrypted.
//
// Return patterns:
//   - nil                     — success (or no record existed).
//   - ErrActiveBotsExist      — → HTTP 409 ACTIVE_BOTS_EXIST.
//   - other error             — unexpected DB error → HTTP 500.
func (l *ApiKeyLogic) DeleteApiKey(ctx context.Context, userID string) error {
	record, err := l.repo.FindByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("api_key_logic: DeleteApiKey: %w", err)
	}
	if record == nil {
		// No configuration exists — deletion is idempotent; nothing to do.
		return nil
	}

	// Guard: reject deletion when any linked bot is still Running.
	hasRunning, err := l.repo.HasRunningBotsByAPIKeyID(ctx, record.ID)
	if err != nil {
		return fmt.Errorf("api_key_logic: DeleteApiKey: %w", err)
	}
	if hasRunning {
		return ErrActiveBotsExist
	}

	if err := l.repo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("api_key_logic: DeleteApiKey: %w", err)
	}
	return nil
}

// BuildProxy constructs a BinanceProxy for the given user by loading their
// encrypted API key from the database and decrypting it in-memory
// (WBS 2.2.4, SRS FR-CORE-01 — Secure API Proxy pattern).
//
// The proxy is the sole gateway through which Bot and Blockly engine code
// communicates with Binance. The Secret Key is decrypted per-call, passed to
// NewBinanceProxy (where it is immediately zeroed), and never assigned to any
// persistent variable or written to logs (SRS NFR-SEC-01, FR-CORE-01).
//
// Return patterns:
//   - (*exchange.BinanceProxy, nil)  — success; caller owns the proxy lifetime.
//   - (nil, ErrAPIKeyNotConfigured)  — user has not saved API keys yet.
//   - (nil, other)                   — DB read or AES-256-GCM decryption error.
func (l *ApiKeyLogic) BuildProxy(ctx context.Context, userID string) (*exchange.BinanceProxy, error) {
	record, err := l.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("api_key_logic: BuildProxy: %w", err)
	}
	if record == nil {
		return nil, ErrAPIKeyNotConfigured
	}

	proxy, err := exchange.NewBinanceProxy(record.ApiKey, record.SecretKeyEncrypted, l.baseURL, l.aesKey, l.limiter)
	if err != nil {
		return nil, fmt.Errorf("api_key_logic: BuildProxy: %w", err)
	}
	return proxy, nil
}

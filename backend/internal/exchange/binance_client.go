package exchange

import (
	"github.com/adshao/go-binance/v2/futures"
)

// NewFuturesClient constructs a go-binance Futures client authenticated with
// the given API Key and Secret Key.
//
// The client is initialized in-memory and handles HMAC-SHA256 request signing
// internally — the Secret Key is never written to logs or stored beyond the
// lifetime of this client instance (SRS FR-CORE-01, NFR-SEC-01).
//
// Usage pattern (Secure API Proxy, WBS 2.2.4):
//
//	client := exchange.NewFuturesClient(apiKey, secretKey)
//	defer client = nil  // discard after use to minimise plaintext window
//
// The returned *futures.Client is NOT safe to cache across requests; callers
// should create a fresh client each time using keys decrypted at call-time
// from the AES-256-GCM encrypted store (pkg/crypto/aes.go).
//
// WBS 1.1.3: dependency initialisation scaffold.
// Full implementation (order placement, position queries, etc.) → WBS 2.2.4.
func NewFuturesClient(apiKey, secretKey string) *futures.Client {
	return futures.NewClient(apiKey, secretKey)
}

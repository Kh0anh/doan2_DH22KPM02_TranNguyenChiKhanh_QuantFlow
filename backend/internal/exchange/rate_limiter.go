package exchange

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Binance Futures IP rate limit constants (SRS FR-CORE-04, WBS 2.2.5).
const (
	// binanceWeightLimit is the default Binance Futures IP weight cap per 1-minute window.
	binanceWeightLimit = 2400

	// throttleThreshold triggers a rate reduction when X-MBX-USED-WEIGHT-1M > 80% of limit.
	throttleThreshold = 1920

	// recoverThreshold restores the normal rate when X-MBX-USED-WEIGHT-1M < 50% of limit.
	// The dead-band between 1200 and 1920 is intentional — hysteresis prevents oscillation.
	recoverThreshold = 1200

	// defaultBurst is the Token Bucket burst capacity for absorbing short synchronised
	// spikes from multiple bots without blocking (NFR-PERF-03: 5 bots in parallel).
	defaultBurst = 20
)

var (
	// normalRate ≈ 40 req/s — conservative default; 40 × 60 = 2400/min = Binance limit.
	normalRate = rate.Every(25 * time.Millisecond)

	// throttledRate ≈ 10 req/s — engaged when weight > 80%, prevents HTTP 429 / IP ban.
	throttledRate = rate.Every(100 * time.Millisecond)
)

// ExchangeRateLimiter is a goroutine-safe singleton Token Bucket that serialises
// all outbound Binance Futures REST requests across every running bot and goroutine
// (SRS FR-CORE-04 — Rate Limiter, WBS 2.2.5).
//
// Design:
//   - Initial rate: 40 req/s (rate.Every(25ms))  — safely fills a 2400/min window.
//   - Burst:        20                            — handles 5-bot synchronised spikes.
//   - Adaptive:     X-MBX-USED-WEIGHT-1M header  — dynamic back-pressure.
//
// Adaptive rules (Adapt):
//   - usedWeight > 1920 (>80%) → throttle: 10 req/s  — backs off before Binance retaliates.
//   - usedWeight < 1200 (<50%) → restore:  40 req/s  — back to normal operation.
//   - 1200 ≤ usedWeight ≤ 1920 → keep current rate  (hysteresis, prevents thrashing).
//
// All BinanceProxy methods are rate-gated automatically via the weightInterceptor
// http.RoundTripper — no per-method changes are required.
type ExchangeRateLimiter struct {
	limiter *rate.Limiter
	mu      sync.Mutex // guards concurrent SetLimit calls
}

// NewExchangeRateLimiter creates the singleton rate limiter at normal rate.
// Call exactly once at application startup (router.go) and share the returned
// pointer across all BinanceProxy instances.
func NewExchangeRateLimiter() *ExchangeRateLimiter {
	return &ExchangeRateLimiter{
		limiter: rate.NewLimiter(normalRate, defaultBurst),
	}
}

// Wait blocks the caller until a token is available in the bucket or ctx is
// cancelled (e.g. bot stopped, HTTP request timeout).
// Returns ctx.Err() wrapped when the context is done.
func (l *ExchangeRateLimiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}

// Adapt adjusts the emission rate based on the X-MBX-USED-WEIGHT-1M value read
// from a Binance response. Called automatically by weightInterceptor after every
// outbound REST call — callers do not need to invoke this directly.
func (l *ExchangeRateLimiter) Adapt(usedWeight int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch {
	case usedWeight > throttleThreshold:
		// Approaching the 2400/min limit — back off to 10 req/s to avoid HTTP 429.
		l.limiter.SetLimit(throttledRate)
	case usedWeight < recoverThreshold:
		// Weight comfortably low — restore full throughput.
		l.limiter.SetLimit(normalRate)
		// Dead-band [1200, 1920]: keep current rate to prevent oscillation.
	}
}

// ---------------------------------------------------------------------------
// weightInterceptor — http.RoundTripper
// ---------------------------------------------------------------------------

// weightInterceptor is an http.RoundTripper middleware injected into the
// futures.Client.HTTPClient of every BinanceProxy (WBS 2.2.5). It:
//  1. Calls ExchangeRateLimiter.Wait before the outbound request — Token Bucket gate.
//  2. Reads the X-MBX-USED-WEIGHT-1M header from every response and calls Adapt.
//
// By intercepting at the transport layer, all BinanceProxy proxy methods are
// rate-limited automatically without any per-method changes.
type weightInterceptor struct {
	inner   http.RoundTripper
	limiter *ExchangeRateLimiter
}

// RoundTrip implements http.RoundTripper.
func (t *weightInterceptor) RoundTrip(req *http.Request) (*http.Response, error) {
	// Block until a token is available (or ctx is cancelled).
	if err := t.limiter.Wait(req.Context()); err != nil {
		return nil, fmt.Errorf("exchange: rate limiter: %w", err)
	}

	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	// Adaptive feedback: adjust rate based on Binance's usage header.
	// Header name: X-MBX-USED-WEIGHT-1M (Binance Futures, 1-minute rolling window).
	if raw := resp.Header.Get("X-MBX-USED-WEIGHT-1M"); raw != "" {
		if w, parseErr := strconv.Atoi(raw); parseErr == nil {
			t.limiter.Adapt(w)
		}
	}

	return resp, nil
}

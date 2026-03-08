package exchange

import (
	"context"
	"fmt"
	"net/http"

	"github.com/adshao/go-binance/v2/futures"
	pkgcrypto "github.com/kh0anh/quantflow/pkg/crypto"
)

// NewFuturesClient constructs a go-binance Futures client authenticated with
// the given API Key and Secret Key.
//
// Used internally for ping-verify during POST /exchange/api-keys (WBS 2.2.1).
// For all other Binance calls, use NewBinanceProxy (WBS 2.2.4).
func NewFuturesClient(apiKey, secretKey string) *futures.Client {
	return futures.NewClient(apiKey, secretKey)
}

// ---------------------------------------------------------------------------
// Secure API Proxy — WBS 2.2.4
// ---------------------------------------------------------------------------

// PlaceOrderParams carries typed parameters for a Futures order request.
// Used by the Blockly Trading engine (WBS 2.5.6, SRS FR-DESIGN-09) and
// SmartOrder pre-flight logic.
type PlaceOrderParams struct {
	// Symbol is the Binance Futures trading pair, e.g. "BTCUSDT".
	Symbol string
	// Side is the order direction: futures.SideTypeBuy or futures.SideTypeSell.
	Side futures.SideType
	// PositionSide controls hedge-mode direction: LONG, SHORT, or BOTH (one-way).
	// Leave empty for one-way (non-hedge) mode — the field is omitted from the
	// request when blank to avoid Binance validation errors.
	PositionSide futures.PositionSideType
	// OrderType is the execution type: futures.OrderTypeMarket or futures.OrderTypeLimit.
	OrderType futures.OrderType
	// Quantity is the order size as a string (e.g. "0.001") to preserve precision.
	Quantity string
	// Price is the limit price as a string; leave empty for MARKET orders.
	Price string
}

// BinanceProxy is the sole gateway between QuantFlow and the Binance Futures
// REST API (SRS FR-CORE-01 — Secure API Proxy pattern, WBS 2.2.4).
//
// The underlying *futures.Client is intentionally private — callers cannot
// extract or log the Secret Key. HMAC-SHA256 request signing is handled
// internally by the go-binance SDK (github.com/adshao/go-binance/v2 v2.8.10).
//
// Lifecycle: create one proxy per request/bot-session via
// ApiKeyLogic.BuildProxy(); discard after use. The Secret Key is decrypted
// into a local []byte, passed to the SDK, then zeroed immediately — it is
// never assigned to a persistent variable and never written to any log
// (SRS NFR-SEC-01, FR-CORE-01).
type BinanceProxy struct {
	client *futures.Client // private — prevents callers from extracting the key
}

// NewBinanceProxy constructs a BinanceProxy by decrypting the AES-256-GCM
// encrypted secret key in-memory, initialising the go-binance FuturesClient,
// and immediately zeroing the plain-text byte slice to minimise the in-RAM
// exposure window (SRS FR-CORE-01, NFR-SEC-01).
//
// The provided limiter is injected into the client's HTTP transport via
// weightInterceptor — every Binance REST call made through this proxy is
// automatically rate-gated and contributes to the adaptive feedback loop
// (SRS FR-CORE-04, WBS 2.2.5).
//
// Parameters:
//   - apiKey           — plain-text Binance Access Key (non-secret).
//   - encryptedSecret  — AES-256-GCM ciphertext from the api_keys table.
//   - aesKey           — 32-byte key from pkgcrypto.DeriveKey(cfg.AESKey).
//   - limiter          — singleton ExchangeRateLimiter (shared across all proxies).
func NewBinanceProxy(apiKey, encryptedSecret string, aesKey []byte, limiter *ExchangeRateLimiter) (*BinanceProxy, error) {
	// Decrypt — the secret exists as plain-text only within this stack frame.
	plaintext, err := pkgcrypto.Decrypt(encryptedSecret, aesKey)
	if err != nil {
		return nil, fmt.Errorf("exchange: NewBinanceProxy: decrypt secret: %w", err)
	}

	// Initialise the SDK client. go-binance copies the secret internally for
	// HMAC-SHA256 signing; we never touch the signing logic directly.
	client := futures.NewClient(apiKey, string(plaintext))

	// Zero-out the decrypted bytes immediately after the SDK has consumed them.
	for i := range plaintext {
		plaintext[i] = 0
	}

	// Inject the rate-limiting transport. All 9 proxy methods are gated
	// automatically — no per-method changes required (WBS 2.2.5).
	client.HTTPClient = &http.Client{
		Transport: &weightInterceptor{
			inner:   http.DefaultTransport,
			limiter: limiter,
		},
	}

	return &BinanceProxy{client: client}, nil
}

// ---------------------------------------------------------------------------
// Proxy methods — one per Binance Futures REST operation required by the
// Blockly engine (WBS 2.5.6) and Bot lifecycle (WBS 2.7.x).
// ---------------------------------------------------------------------------

// GetAccount returns Futures account information (balances, commissions, positions).
// Used for ping-verify on POST /exchange/api-keys and general account info
// (WBS 2.2.1, FR-DESIGN-08).
func (p *BinanceProxy) GetAccount(ctx context.Context) (*futures.Account, error) {
	res, err := p.client.NewGetAccountService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("exchange: GetAccount: %w", err)
	}
	return res, nil
}

// GetBalance returns asset balances for all Futures wallet assets.
// Mapped to the Blockly get_balance block (WBS 2.5.6, SRS FR-DESIGN-08).
func (p *BinanceProxy) GetBalance(ctx context.Context) ([]*futures.Balance, error) {
	res, err := p.client.NewGetBalanceService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("exchange: GetBalance: %w", err)
	}
	return res, nil
}

// GetPositionRisk returns open position risk data for the given symbol.
// Mapped to the Blockly get_position_info block (WBS 2.5.6, SRS FR-DESIGN-08).
func (p *BinanceProxy) GetPositionRisk(ctx context.Context, symbol string) ([]*futures.PositionRisk, error) {
	res, err := p.client.NewGetPositionRiskService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("exchange: GetPositionRisk(%s): %w", symbol, err)
	}
	return res, nil
}

// GetOpenOrders returns all open orders for the given symbol.
// Mapped to the Blockly get_order_info block (WBS 2.5.6, SRS FR-DESIGN-08).
func (p *BinanceProxy) GetOpenOrders(ctx context.Context, symbol string) ([]*futures.Order, error) {
	res, err := p.client.NewListOpenOrdersService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("exchange: GetOpenOrders(%s): %w", symbol, err)
	}
	return res, nil
}

// PlaceOrder submits a new Futures order to Binance.
// Mapped to the Blockly place_order / SmartOrder block (WBS 2.5.6, FR-DESIGN-09).
// HMAC-SHA256 signing is handled transparently by the go-binance SDK.
//
// PositionSide in PlaceOrderParams is omitted from the request when blank,
// which is correct for one-way (non-hedge) account mode.
func (p *BinanceProxy) PlaceOrder(ctx context.Context, params PlaceOrderParams) (*futures.CreateOrderResponse, error) {
	svc := p.client.NewCreateOrderService().
		Symbol(params.Symbol).
		Side(params.Side).
		Type(params.OrderType).
		Quantity(params.Quantity)

	if params.PositionSide != "" {
		svc = svc.PositionSide(params.PositionSide)
	}
	if params.OrderType == futures.OrderTypeLimit && params.Price != "" {
		svc = svc.Price(params.Price).TimeInForce(futures.TimeInForceTypeGTC)
	}

	res, err := svc.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("exchange: PlaceOrder(%s %s %s): %w",
			params.Symbol, params.Side, params.OrderType, err)
	}
	return res, nil
}

// ChangeLeverage sets the leverage multiplier for the given symbol.
// Called as SmartOrder pre-flight when the strategy leverage differs from
// the current account setting (WBS 2.5.6, SRS FR-DESIGN-09).
func (p *BinanceProxy) ChangeLeverage(ctx context.Context, symbol string, leverage int) (*futures.SymbolLeverage, error) {
	res, err := p.client.NewChangeLeverageService().
		Symbol(symbol).
		Leverage(leverage).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("exchange: ChangeLeverage(%s, %d): %w", symbol, leverage, err)
	}
	return res, nil
}

// ChangeMarginType switches the margin mode (ISOLATED / CROSSED) for the symbol.
// Called as SmartOrder pre-flight when the strategy margin type differs
// (WBS 2.5.6, SRS FR-DESIGN-09).
func (p *BinanceProxy) ChangeMarginType(ctx context.Context, symbol string, marginType futures.MarginType) error {
	err := p.client.NewChangeMarginTypeService().
		Symbol(symbol).
		MarginType(marginType).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("exchange: ChangeMarginType(%s, %s): %w", symbol, marginType, err)
	}
	return nil
}

// ClosePosition closes the current position for the given symbol by placing a
// reduce-only MARKET order. Callers must supply the full position quantity
// (obtained via GetPositionRisk) so the order dimensions are exact.
// Mapped to the Blockly close_position block (WBS 2.5.6, SRS FR-DESIGN-10).
func (p *BinanceProxy) ClosePosition(ctx context.Context, symbol string, side futures.SideType, quantity string) (*futures.CreateOrderResponse, error) {
	res, err := p.client.NewCreateOrderService().
		Symbol(symbol).
		Side(side).
		Type(futures.OrderTypeMarket).
		Quantity(quantity).
		ReduceOnly(true).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("exchange: ClosePosition(%s): %w", symbol, err)
	}
	return res, nil
}

// CancelAllOrders cancels every open order for the given symbol.
// Mapped to the Blockly cancel_all_orders block (WBS 2.5.6, SRS FR-DESIGN-10).
func (p *BinanceProxy) CancelAllOrders(ctx context.Context, symbol string) error {
	err := p.client.NewCancelAllOpenOrdersService().
		Symbol(symbol).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("exchange: CancelAllOrders(%s): %w", symbol, err)
	}
	return nil
}

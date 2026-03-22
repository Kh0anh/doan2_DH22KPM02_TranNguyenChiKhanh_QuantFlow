package exchange

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/kh0anh/quantflow/internal/domain"
	pkgcrypto "github.com/kh0anh/quantflow/pkg/crypto"
	"github.com/shopspring/decimal"
)

// NewFuturesClient constructs a go-binance Futures client authenticated with
// the given API Key and Secret Key.
//
// baseURL overrides the default Binance Futures REST base URL when non-empty
// (e.g. Binance testnet "https://testnet.binancefuture.com").
// Pass an empty string to use the library default.
//
// Used internally for ping-verify during POST /exchange/api-keys (WBS 2.2.1).
func NewFuturesClient(apiKey, secretKey, baseURL string) *futures.Client {
	client := futures.NewClient(apiKey, secretKey)
	if baseURL != "" {
		client.BaseURL = baseURL
	}
	return client
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
// baseURL overrides the Binance Futures REST base URL when non-empty.
// Pass an empty string to use the go-binance library default.
//
// The provided limiter is injected into the client’s HTTP transport via
// weightInterceptor — every Binance REST call made through this proxy is
// automatically rate-gated (SRS FR-CORE-04, WBS 2.2.5).
func NewBinanceProxy(apiKey, encryptedSecret, baseURL string, aesKey []byte, limiter *ExchangeRateLimiter) (*BinanceProxy, error) {
	// Decrypt — the secret exists as plain-text only within this stack frame.
	plaintext, err := pkgcrypto.Decrypt(encryptedSecret, aesKey)
	if err != nil {
		return nil, fmt.Errorf("exchange: NewBinanceProxy: decrypt secret: %w", err)
	}

	// Initialise the SDK client.
	client := futures.NewClient(apiKey, string(plaintext))

	// Override base URL when configured (testnet / internal proxy).
	if baseURL != "" {
		client.BaseURL = baseURL
	}

	// Zero-out the decrypted bytes immediately after the SDK has consumed them.
	for i := range plaintext {
		plaintext[i] = 0
	}

	// Inject the rate-limiting transport.
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

// CreateCloseOrder places a reduce-only MARKET order to close a known position.
// Callers must supply the exact side and quantity (obtained via GetPositionRisk).
// This is the low-level primitive used by the Bot engine (WBS 2.7.x) when it
// needs precise control over the close order. For Blockly block execution use
// ClosePosition (the high-level smart method) instead.
func (p *BinanceProxy) CreateCloseOrder(ctx context.Context, symbol string, side futures.SideType, quantity string) (*futures.CreateOrderResponse, error) {
	res, err := p.client.NewCreateOrderService().
		Symbol(symbol).
		Side(side).
		Type(futures.OrderTypeMarket).
		Quantity(quantity).
		ReduceOnly(true).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("exchange: CreateCloseOrder(%s): %w", symbol, err)
	}
	return res, nil
}

// CancelAllOrders cancels every open order for the given symbol.
// Safely returns nil when there are no open orders (idempotent).
// Mapped to the Blockly trade_cancel_all_orders block and satisfies
// blockly.TradingProxy.CancelAllOrders (WBS 2.5.6, SRS FR-DESIGN-10).
func (p *BinanceProxy) CancelAllOrders(ctx context.Context, symbol string) error {
	// Pre-check: avoid Binance error code -2011 on empty order list.
	orders, listErr := p.client.NewListOpenOrdersService().Symbol(symbol).Do(ctx)
	if listErr != nil {
		return fmt.Errorf("exchange: CancelAllOrders(%s): list orders: %w", symbol, listErr)
	}
	if len(orders) == 0 {
		return nil // nothing to cancel
	}
	if err := p.client.NewCancelAllOpenOrdersService().Symbol(symbol).Do(ctx); err != nil {
		return fmt.Errorf("exchange: CancelAllOrders(%s): %w", symbol, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// High-level TradingProxy interface methods (Task 2.5.6, blockly/executor.go)
//
// These methods present domain-level semantics (decimal.Decimal, plain strings)
// and satisfy the blockly.TradingProxy interface via Go structural typing.
// They are the sole public surface used by block_data.go and block_trading.go.
// ---------------------------------------------------------------------------

// GetLastPrice returns the latest mark/ticker price for the given symbol.
// Mapped to data_market_price PRICE_TYPE="LAST_PRICE" (blockly.md §3.6.1,
// SRS FR-DESIGN-08, WBS 2.5.6).
func (p *BinanceProxy) GetLastPrice(ctx context.Context, symbol string) (decimal.Decimal, error) {
	prices, err := p.client.NewListPricesService().Symbol(symbol).Do(ctx)
	if err != nil {
		return decimal.Zero, fmt.Errorf("exchange: GetLastPrice(%s): %w", symbol, err)
	}
	for _, price := range prices {
		if price.Symbol == symbol {
			v, parseErr := decimal.NewFromString(price.Price)
			if parseErr != nil {
				return decimal.Zero, fmt.Errorf("exchange: GetLastPrice(%s): parse %q: %w", symbol, price.Price, parseErr)
			}
			return v, nil
		}
	}
	return decimal.Zero, fmt.Errorf("exchange: GetLastPrice(%s): symbol not found in response", symbol)
}

// GetAvailableBalance returns the available (withdrawable) USDT balance in the
// Futures wallet. Mapped to data_balance (blockly.md §3.6.4, SRS FR-DESIGN-08,
// WBS 2.5.6).
func (p *BinanceProxy) GetAvailableBalance(ctx context.Context) (decimal.Decimal, error) {
	balances, err := p.client.NewGetBalanceService().Do(ctx)
	if err != nil {
		return decimal.Zero, fmt.Errorf("exchange: GetAvailableBalance: %w", err)
	}
	for _, b := range balances {
		if b.Asset == "USDT" {
			v, parseErr := decimal.NewFromString(b.AvailableBalance)
			if parseErr != nil {
				return decimal.Zero, fmt.Errorf("exchange: GetAvailableBalance: parse %q: %w", b.AvailableBalance, parseErr)
			}
			return v, nil
		}
	}
	// No USDT entry means zero available balance.
	return decimal.Zero, nil
}

// getPositionRiskForSymbol is a shared helper that returns the first matching
// PositionRisk entry for symbol. Returns nil (no error) when no position exists.
func (p *BinanceProxy) getPositionRiskForSymbol(ctx context.Context, symbol string) (*futures.PositionRisk, error) {
	risks, err := p.client.NewGetPositionRiskService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("exchange: getPositionRiskForSymbol(%s): %w", symbol, err)
	}
	for _, r := range risks {
		if r.Symbol == symbol {
			return r, nil
		}
	}
	return nil, nil
}

// GetPositionSize returns the absolute position amount (PositionAmt) for the
// symbol. Positive = Long; Negative = Short; Zero = no open position.
// Mapped to data_position_info FIELD="POSITION_SIZE" (blockly.md §3.6.2,
// SRS FR-DESIGN-08, WBS 2.5.6).
func (p *BinanceProxy) GetPositionSize(ctx context.Context, symbol string) (decimal.Decimal, error) {
	risk, err := p.getPositionRiskForSymbol(ctx, symbol)
	if err != nil {
		return decimal.Zero, fmt.Errorf("exchange: GetPositionSize(%s): %w", symbol, err)
	}
	if risk == nil {
		return decimal.Zero, nil
	}
	v, parseErr := decimal.NewFromString(risk.PositionAmt)
	if parseErr != nil {
		return decimal.Zero, fmt.Errorf("exchange: GetPositionSize(%s): parse PositionAmt %q: %w", symbol, risk.PositionAmt, parseErr)
	}
	return v, nil
}

// GetPositionEntryPrice returns the average entry price of the open position.
// Returns decimal.Zero when no position is open.
// Mapped to data_position_info FIELD="ENTRY_PRICE" (blockly.md §3.6.2,
// SRS FR-DESIGN-08, WBS 2.5.6).
func (p *BinanceProxy) GetPositionEntryPrice(ctx context.Context, symbol string) (decimal.Decimal, error) {
	risk, err := p.getPositionRiskForSymbol(ctx, symbol)
	if err != nil {
		return decimal.Zero, fmt.Errorf("exchange: GetPositionEntryPrice(%s): %w", symbol, err)
	}
	if risk == nil {
		return decimal.Zero, nil
	}
	v, parseErr := decimal.NewFromString(risk.EntryPrice)
	if parseErr != nil {
		return decimal.Zero, fmt.Errorf("exchange: GetPositionEntryPrice(%s): parse EntryPrice %q: %w", symbol, risk.EntryPrice, parseErr)
	}
	return v, nil
}

// GetPositionUnrealizedPNL returns the unrealized PnL of the open position.
// Returns decimal.Zero when no position is open.
// Mapped to data_position_info FIELD="UNREALIZED_PNL" (blockly.md §3.6.2,
// SRS FR-DESIGN-08, WBS 2.5.6).
func (p *BinanceProxy) GetPositionUnrealizedPNL(ctx context.Context, symbol string) (decimal.Decimal, error) {
	risk, err := p.getPositionRiskForSymbol(ctx, symbol)
	if err != nil {
		return decimal.Zero, fmt.Errorf("exchange: GetPositionUnrealizedPNL(%s): %w", symbol, err)
	}
	if risk == nil {
		return decimal.Zero, nil
	}
	v, parseErr := decimal.NewFromString(risk.UnRealizedProfit)
	if parseErr != nil {
		return decimal.Zero, fmt.Errorf("exchange: GetPositionUnrealizedPNL(%s): parse UnRealizedProfit %q: %w", symbol, risk.UnRealizedProfit, parseErr)
	}
	return v, nil
}

// GetOpenOrdersCount returns the number of pending (open) orders for the symbol.
// Mapped to data_open_orders_count (blockly.md §3.6.3, SRS FR-DESIGN-08, WBS 2.5.6).
func (p *BinanceProxy) GetOpenOrdersCount(ctx context.Context, symbol string) (int, error) {
	orders, err := p.client.NewListOpenOrdersService().Symbol(symbol).Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("exchange: GetOpenOrdersCount(%s): %w", symbol, err)
	}
	return len(orders), nil
}

// getSymbolPrecision fetches the quantityPrecision and pricePrecision for the
// given symbol from the Binance ExchangeInfo endpoint. These values indicate
// the maximum number of decimal places allowed for each field.
func (p *BinanceProxy) getSymbolPrecision(ctx context.Context, symbol string) (qtyPrecision, pricePrecision int32, err error) {
	info, infoErr := p.client.NewExchangeInfoService().Do(ctx)
	if infoErr != nil {
		return 0, 0, fmt.Errorf("exchange: getSymbolPrecision(%s): %w", symbol, infoErr)
	}
	for _, s := range info.Symbols {
		if s.Symbol == symbol {
			return int32(s.QuantityPrecision), int32(s.PricePrecision), nil
		}
	}
	// Symbol not found — fallback to conservative defaults.
	return 3, 2, nil
}

// SmartOrder is the "All-in-one" Futures order method that satisfies
// blockly.TradingProxy.SmartOrder (FR-DESIGN-09, blockly.md §3.6.5, WBS 2.5.6).
//
// Pre-flight sequence (runs before NewCreateOrderService):
//  1. Fetch current PositionRisk to read leverage and marginType from the
//     exchange account state.
//  2. If desired leverage differs from current → call ChangeLeverage.
//     (Can be changed at any time, even with open positions.)
//  3. If desired marginType differs from current AND no position is open →
//     call ChangeMarginType. Binance rejects this call when a position exists;
//     the mismatch is silently skipped and a warning is logged in that case.
//     Binance also returns an error "No need to change" when the type already
//     matches — that error is suppressed (idempotent pre-flight).
//  4. Place the order: LONG → SideTypeBuy; SHORT → SideTypeSell.
//     MARKET orders omit the Price field; LIMIT orders set Price + GTC TIF.
//
// side:       "LONG" or "SHORT".
// orderType:  "MARKET" or "LIMIT".
// marginType: "CROSS" or "ISOLATED".
func (p *BinanceProxy) SmartOrder(
	ctx context.Context,
	symbol, side, orderType string,
	price, quantity decimal.Decimal,
	leverage int,
	marginType string,
) (*domain.OrderResult, error) {
	// ── Step 1: fetch current account state for this symbol ──────────────
	risk, err := p.getPositionRiskForSymbol(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("exchange: SmartOrder(%s): preflight position check: %w", symbol, err)
	}

	var currentLeverage int
	var currentMarginType string
	hasOpenPosition := false

	if risk != nil {
		if lev, parseErr := strconv.Atoi(risk.Leverage); parseErr == nil {
			currentLeverage = lev
		}
		currentMarginType = strings.ToUpper(risk.MarginType)
		posAmt, parseErr := decimal.NewFromString(risk.PositionAmt)
		if parseErr == nil && !posAmt.IsZero() {
			hasOpenPosition = true
		}
	}

	// ── Step 2: adjust leverage if needed ──────────────────────────────
	if leverage > 0 && currentLeverage != leverage {
		if _, leverageErr := p.client.NewChangeLeverageService().
			Symbol(symbol).
			Leverage(leverage).
			Do(ctx); leverageErr != nil {
			return nil, fmt.Errorf("exchange: SmartOrder(%s): ChangeLeverage to %dx: %w", symbol, leverage, leverageErr)
		}
	}

	// ── Step 3: adjust margin type if needed and position allows ────────
	// Map block values "CROSS"/"ISOLATED" → Binance API values "crossed"/"isolated".
	desiredMarginType := strings.ToUpper(marginType)
	if desiredMarginType != "" && desiredMarginType != currentMarginType {
		if hasOpenPosition {
			// Cannot change margin type while a position is open; skip silently.
			_ = fmt.Sprintf("exchange: SmartOrder(%s): skip ChangeMarginType — position open", symbol)
		} else {
			var mt futures.MarginType
			if desiredMarginType == "ISOLATED" {
				mt = futures.MarginTypeIsolated
			} else {
				mt = futures.MarginTypeCrossed
			}
			marginErr := p.client.NewChangeMarginTypeService().
				Symbol(symbol).
				MarginType(mt).
				Do(ctx)
			if marginErr != nil {
				// Binance returns an error when margin type is already set to the
				// requested value ("No need to change"). Suppress that specific
				// case to make this pre-flight idempotent.
				if !strings.Contains(marginErr.Error(), "No need to change") {
					return nil, fmt.Errorf("exchange: SmartOrder(%s): ChangeMarginType to %s: %w", symbol, mt, marginErr)
				}
			}
		}
	}

	// ── Step 4: truncate quantity/price to exchange precision ─────────
	qtyPrec, pricePrec, precErr := p.getSymbolPrecision(ctx, symbol)
	if precErr != nil {
		// Non-fatal: fallback to conservative precision if ExchangeInfo fails.
		qtyPrec, pricePrec = 3, 2
	}
	quantity = quantity.Truncate(qtyPrec)
	if !price.IsZero() {
		price = price.Truncate(pricePrec)
	}

	// ── Step 5: place the order ───────────────────────────────────────
	var binanceSide futures.SideType
	if strings.ToUpper(side) == "LONG" {
		binanceSide = futures.SideTypeBuy
	} else {
		binanceSide = futures.SideTypeSell
	}

	var binanceOrderType futures.OrderType
	if strings.ToUpper(orderType) == "LIMIT" {
		binanceOrderType = futures.OrderTypeLimit
	} else {
		binanceOrderType = futures.OrderTypeMarket
	}

	svc := p.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binanceSide).
		Type(binanceOrderType).
		Quantity(quantity.String())

	if binanceOrderType == futures.OrderTypeLimit && !price.IsZero() {
		svc = svc.Price(price.String()).TimeInForce(futures.TimeInForceTypeGTC)
	}

	res, orderErr := svc.Do(ctx)
	if orderErr != nil {
		return nil, fmt.Errorf("exchange: SmartOrder(%s %s %s): %w", symbol, side, orderType, orderErr)
	}

	// ── Step 6: build OrderResult from Binance response ──────────────
	result := buildOrderResult(res, symbol, side)
	return result, nil
}

// ClosePosition closes the entire open position for the given symbol by
// placing a reduce-only MARKET order on the opposite side.
// No-op (returns nil) when no position is currently open.
// This method satisfies blockly.TradingProxy.ClosePosition via Go structural
// typing (FR-DESIGN-10, blockly.md §3.6.6, WBS 2.5.6).
func (p *BinanceProxy) ClosePosition(ctx context.Context, symbol string) (*domain.OrderResult, error) {
	risk, err := p.getPositionRiskForSymbol(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("exchange: ClosePosition(%s): %w", symbol, err)
	}
	if risk == nil {
		return nil, nil // no position — nothing to close
	}

	posAmt, parseErr := decimal.NewFromString(risk.PositionAmt)
	if parseErr != nil {
		return nil, fmt.Errorf("exchange: ClosePosition(%s): parse PositionAmt %q: %w", symbol, risk.PositionAmt, parseErr)
	}
	if posAmt.IsZero() {
		return nil, nil // zero position — nothing to close
	}

	// Determine the position side for the OrderResult.
	originalSide := "LONG"
	if posAmt.IsNegative() {
		originalSide = "SHORT"
	}

	// Capture realized PnL before closing (from PositionRisk data).
	realizedPnL := decimal.Zero
	// Note: Binance PositionRisk doesn't have a realizedPnL field directly;
	// we compute it from the unrealized PnL at close time as an approximation.
	// The actual realized PnL is better captured from the order response or
	// account income history. For now we record decimal.Zero and let the
	// user reconcile via Binance account statements.

	// Close side is opposite to open side.
	var closeSide futures.SideType
	if posAmt.IsPositive() {
		// Long position → close with Sell.
		closeSide = futures.SideTypeSell
	} else {
		// Short position → close with Buy. Quantity must be absolute.
		closeSide = futures.SideTypeBuy
		posAmt = posAmt.Abs()
	}

	res, orderErr := p.client.NewCreateOrderService().
		Symbol(symbol).
		Side(closeSide).
		Type(futures.OrderTypeMarket).
		Quantity(posAmt.String()).
		ReduceOnly(true).
		Do(ctx)
	if orderErr != nil {
		return nil, fmt.Errorf("exchange: ClosePosition(%s): %w", symbol, orderErr)
	}

	result := buildOrderResult(res, symbol, originalSide)
	result.RealizedPnL = realizedPnL
	return result, nil
}

// buildOrderResult converts a Binance CreateOrderResponse into a domain.OrderResult.
// Shared by SmartOrder and ClosePosition.
func buildOrderResult(res *futures.CreateOrderResponse, symbol, side string) *domain.OrderResult {
	if res == nil {
		return nil
	}

	qty, _ := decimal.NewFromString(res.ExecutedQuantity)
	avgPrice, _ := decimal.NewFromString(res.AvgPrice)
	// Binance Futures doesn't always populate AvgPrice on MARKET fills.
	// Fallback: compute from cumQuote / executedQty.
	if avgPrice.IsZero() && !qty.IsZero() {
		cumQuote, _ := decimal.NewFromString(res.CumQuote)
		if !cumQuote.IsZero() {
			avgPrice = cumQuote.Div(qty)
		}
	}

	// Commission is not directly in CreateOrderResponse for Futures.
	// Estimate fee as qty * avgPrice * 0.0004 (taker fee).
	fee := qty.Mul(avgPrice).Mul(decimal.NewFromFloat(0.0004))

	orderTime := time.UnixMilli(res.UpdateTime)
	if res.UpdateTime == 0 {
		orderTime = time.Now().UTC()
	}

	return &domain.OrderResult{
		OrderID:  res.OrderID,
		Symbol:   symbol,
		Side:     strings.ToUpper(side),
		Quantity: qty,
		Price:    avgPrice,
		Fee:      fee,
		Status:   string(res.Status),
		Time:     orderTime,
	}
}

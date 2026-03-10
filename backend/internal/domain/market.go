package domain

// MarketSymbol carries the 24-hour ticker summary for a single watched
// trading pair, served by GET /market/symbols (WBS 2.4.3, api.yaml §MarketSymbol).
//
// Field names and JSON tags match the API schema exactly so the handler can
// serialise the slice directly without an intermediate mapping layer.
type MarketSymbol struct {
	// Symbol is the Binance Futures trading pair, e.g. "BTCUSDT".
	Symbol string `json:"symbol"`

	// LastPrice is the most recent trade price (string to preserve exchange precision).
	LastPrice string `json:"last_price"`

	// PriceChangePercent is the 24-hour price change expressed as a percentage string
	// (e.g. "2.35" means +2.35 %). Negative values carry a leading minus sign.
	PriceChangePercent string `json:"price_change_percent"`

	// Volume24h is the 24-hour quote-asset volume (USDT) — the financially meaningful
	// measure for Futures pairs (quoteVolume from Binance, not base-asset volume).
	Volume24h string `json:"volume_24h"`
}

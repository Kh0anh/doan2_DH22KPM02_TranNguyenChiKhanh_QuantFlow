package domain

import "time"

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

// CandleOHLCV is the JSON-serialisable representation of one candle bar
// returned by GET /market/candles (WBS 2.4.4, api.yaml §Candle).
//
// Prices are returned as float64 (parsed from the DB decimal strings) so that
// Lightweight Charts on the Frontend can consume them without further conversion.
type CandleOHLCV struct {
	OpenTime time.Time `json:"open_time"`
	Open     float64   `json:"open"`
	High     float64   `json:"high"`
	Low      float64   `json:"low"`
	Close    float64   `json:"close"`
	Volume   float64   `json:"volume"`
	IsClosed bool      `json:"is_closed"`
}

// TradeMarker is a single trade execution point that is overlaid on the candle
// chart to show where a Bot placed an order (WBS 2.4.4, api.yaml §TradeMarker).
//
// Markers are sourced from the trade_history table JOIN bot_instances by
// TradeMarkerRepository (repository/trade_marker_repo.go). The slice is empty
// until the Bot engine (WBS 2.7.x) and Trade History (WBS 2.8.5) produce data.
type TradeMarker struct {
	Time    time.Time `json:"time"`
	Price   float64   `json:"price"`
	Side    string    `json:"side"`
	BotName string    `json:"bot_name"`
	BotID   string    `json:"bot_id"`
}

// CandleChartData is the top-level response payload for GET /market/candles
// (WBS 2.4.4, api.yaml §CandleData).
type CandleChartData struct {
	Symbol    string        `json:"symbol"`
	Timeframe string        `json:"timeframe"`
	Candles   []CandleOHLCV `json:"candles"`
	Markers   []TradeMarker `json:"markers"`
}

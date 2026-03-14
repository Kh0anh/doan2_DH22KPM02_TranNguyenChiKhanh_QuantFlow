package websocket

// channel_market_ticker.go — market_ticker WebSocket channel handler.
//
// MarketTickerChannel connects upstream Binance Futures WebSocket streams
// (24h ticker + multi-timeframe kline OHLCV) to downstream WebSocket clients
// subscribed to the market_ticker channel.
//
// For each watched symbol it starts:
//   - 1 goroutine  → futures.WsMarketTickerServe  → pushes "market_ticker" events
//   - 6 goroutines → futures.WsKlineServe          → pushes "market_candle" events
//     (one per standard timeframe: 1m, 5m, 15m, 1h, 4h, 1d)
//
// Closed-candle rule (WBS 2.8.2: "is_closed=true then INSERT DB + push"):
//   - event.Kline.IsFinal == true  → CandleRepository.InsertOne first, then push.
//   - event.Kline.IsFinal == false → push only (in-progress candle, ephemeral tick).
//
// Concurrency: one goroutine per (symbol, stream-type) pair.
// All goroutines respect ctx cancellation for graceful SIGTERM shutdown.
//
// Task 2.8.2 — Channel market_ticker (ticker + candle OHLCV events).
// WBS: P2-Backend · 15/03/2026.
// Referenced by: websocket.md §3.1, WBS 2.4.1 (KlineSyncService), WBS 2.8.1.

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/repository"
)

// watchedTimeframes lists the six standard Binance Futures kline intervals that
// MarketTickerChannel streams for every watched symbol. Matches the domain
// CandleInterval* constants defined in domain/candle.go and DB schema §9.
var watchedTimeframes = []string{
	domain.CandleInterval1m,
	domain.CandleInterval5m,
	domain.CandleInterval15m,
	domain.CandleInterval1h,
	domain.CandleInterval4h,
	domain.CandleInterval1d,
}

// ─── Push payload types ───────────────────────────────────────────────────────

// marketTickerPush is the outer WS envelope for the "market_ticker" event
// (websocket.md §2.3, §3.1 — Event: market_ticker).
type marketTickerPush struct {
	Event   string          `json:"event"`
	Channel string          `json:"channel"`
	Data    tickerEventData `json:"data"`
}

// tickerEventData is the data payload for the "market_ticker" push event
// (websocket.md §3.1 — Event: market_ticker).
type tickerEventData struct {
	Symbol             string  `json:"symbol"`
	LastPrice          float64 `json:"last_price"`
	PriceChangePercent float64 `json:"price_change_percent"`
	High24h            float64 `json:"high_24h"`
	Low24h             float64 `json:"low_24h"`
	Volume24h          float64 `json:"volume_24h"`
	Timestamp          string  `json:"timestamp"`
}

// marketCandlePush is the outer WS envelope for the "market_candle" event
// (websocket.md §2.3, §3.1 — Event: market_candle).
type marketCandlePush struct {
	Event   string          `json:"event"`
	Channel string          `json:"channel"`
	Data    candleEventData `json:"data"`
}

// candleEventData is the data payload for the "market_candle" push event
// (websocket.md §3.1 — Event: market_candle).
type candleEventData struct {
	Symbol    string        `json:"symbol"`
	Timeframe string        `json:"timeframe"`
	Candle    candlePayload `json:"candle"`
}

// candlePayload carries the per-candle OHLCV fields (websocket.md §3.1).
type candlePayload struct {
	OpenTime string  `json:"open_time"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	Volume   float64 `json:"volume"`
	IsClosed bool    `json:"is_closed"`
}

// ─── MarketTickerChannel ──────────────────────────────────────────────────────

// MarketTickerChannel manages per-symbol Binance Futures WebSocket streams and
// fans out ticker and candle OHLCV events to subscribed WebSocket clients.
//
// It is designed to be started once on server startup via StartWatchedSymbols.
// All goroutines exit cleanly when ctx is cancelled (SIGTERM graceful shutdown).
type MarketTickerChannel struct {
	candleRepo repository.CandleRepository
	manager    *WSManager
	logger     *slog.Logger
}

// NewMarketTickerChannel constructs a MarketTickerChannel.
//
//   - candleRepo: used to persist closed candles (IsFinal == true) before push.
//   - manager:    singleton WSManager used to fan-out push events to subscribers.
//   - logger:     slog.Logger; slog.Default() is used when nil.
func NewMarketTickerChannel(
	candleRepo repository.CandleRepository,
	manager *WSManager,
	logger *slog.Logger,
) *MarketTickerChannel {
	if logger == nil {
		logger = slog.Default()
	}
	return &MarketTickerChannel{
		candleRepo: candleRepo,
		manager:    manager,
		logger:     logger.With(slog.String("component", "market_ticker_channel")),
	}
}

// StartWatchedSymbols launches all Binance WS streams for the provided symbol
// list. Called once on server startup, driven by the WATCHED_SYMBOLS env var
// (tech_stack.md §2.2.1, WBS 2.4.1).
//
// For each symbol it starts:
//   - 1 goroutine for the 24h ticker stream (runTickerStream).
//   - 6 goroutines for kline streams, one per standard timeframe (runKlineStream).
//
// Errors from individual stream starts are logged but never abort the loop —
// one bad symbol must never prevent valid symbols from being monitored
// (SRS NFR-REL, NFR-PERF-03).
func (ch *MarketTickerChannel) StartWatchedSymbols(ctx context.Context, symbols []string) {
	for _, sym := range symbols {
		go ch.runTickerStream(ctx, sym)

		for _, interval := range watchedTimeframes {
			go ch.runKlineStream(ctx, sym, interval)
		}
	}
}

// ─── Internal stream runners ──────────────────────────────────────────────────

// runTickerStream subscribes to the Binance Futures 24h ticker stream for
// symbol, builds a "market_ticker" push payload on every event, and fans it
// out via WSManager.PushToSubscribersForSymbol to all subscribed clients.
//
// The goroutine exits when ctx is cancelled or the SDK closes the connection.
func (ch *MarketTickerChannel) runTickerStream(ctx context.Context, symbol string) {
	wsHandler := func(event *futures.WsMarketTickerEvent) {
		data := tickerEventData{
			Symbol:             event.Symbol,
			LastPrice:          parseDecimalString(event.ClosePrice),
			PriceChangePercent: parseDecimalString(event.PriceChangePercent),
			High24h:            parseDecimalString(event.HighPrice),
			Low24h:             parseDecimalString(event.LowPrice),
			Volume24h:          parseDecimalString(event.QuoteVolume),
			Timestamp:          time.UnixMilli(event.Time).UTC().Format(time.RFC3339),
		}

		push := marketTickerPush{
			Event:   "market_ticker",
			Channel: "market_ticker",
			Data:    data,
		}

		payload, err := json.Marshal(push)
		if err != nil {
			ch.logger.Error("market_ticker: marshal ticker event",
				slog.String("symbol", symbol),
				slog.Any("error", err),
			)
			return
		}

		ch.manager.PushToSubscribersForSymbol(symbol, payload)
	}

	errHandler := func(err error) {
		ch.logger.Warn("market_ticker: ticker stream error",
			slog.String("symbol", symbol),
			slog.Any("error", err),
		)
	}

	doneC, sdkStopC, err := futures.WsMarketTickerServe(symbol, wsHandler, errHandler)
	if err != nil {
		ch.logger.Error("market_ticker: WsMarketTickerServe failed",
			slog.String("symbol", symbol),
			slog.Any("error", err),
		)
		return
	}

	select {
	case <-ctx.Done():
		sdkStopC <- struct{}{}
		<-doneC
	case <-doneC:
		ch.logger.Info("market_ticker: ticker stream closed by remote",
			slog.String("symbol", symbol),
		)
	}
}

// runKlineStream subscribes to the Binance Futures kline stream for the given
// (symbol, interval) pair and fans out "market_candle" push events.
//
// Closed-candle rule (WBS 2.8.2: "is_closed=true then INSERT DB + push"):
//   - IsFinal == true  → CandleRepository.InsertOne first, then push.
//     ON CONFLICT DO NOTHING (unique constraint on symbol+interval+open_time)
//     ensures idempotency alongside KlineSyncService streams (Task 2.4.1).
//   - IsFinal == false → push only (in-progress candle; ephemeral tick data).
//
// A failed INSERT is non-fatal — logged and the push continues so clients
// still receive the candle update. The GapFillerWorker (Task 2.4.2) recovers
// any missing rows on next startup.
func (ch *MarketTickerChannel) runKlineStream(ctx context.Context, symbol, interval string) {
	wsHandler := func(event *futures.WsKlineEvent) {
		k := event.Kline

		if k.IsFinal {
			candle := &domain.Candle{
				Symbol:     event.Symbol,
				Interval:   k.Interval,
				OpenTime:   time.UnixMilli(k.StartTime).UTC(),
				OpenPrice:  k.Open,
				HighPrice:  k.High,
				LowPrice:   k.Low,
				ClosePrice: k.Close,
				Volume:     k.Volume,
				IsClosed:   true,
			}
			if err := ch.candleRepo.InsertOne(ctx, candle); err != nil {
				ch.logger.Warn("market_ticker: InsertOne failed",
					slog.String("symbol", symbol),
					slog.String("interval", interval),
					slog.Any("error", err),
				)
			}
		}

		push := marketCandlePush{
			Event:   "market_candle",
			Channel: "market_ticker",
			Data: candleEventData{
				Symbol:    event.Symbol,
				Timeframe: k.Interval,
				Candle: candlePayload{
					OpenTime: time.UnixMilli(k.StartTime).UTC().Format(time.RFC3339),
					Open:     parseDecimalString(k.Open),
					High:     parseDecimalString(k.High),
					Low:      parseDecimalString(k.Low),
					Close:    parseDecimalString(k.Close),
					Volume:   parseDecimalString(k.Volume),
					IsClosed: k.IsFinal,
				},
			},
		}

		payload, err := json.Marshal(push)
		if err != nil {
			ch.logger.Error("market_ticker: marshal candle event",
				slog.String("symbol", symbol),
				slog.String("interval", interval),
				slog.Any("error", err),
			)
			return
		}

		ch.manager.PushToSubscribersForSymbol(event.Symbol, payload)
	}

	errHandler := func(err error) {
		ch.logger.Warn("market_ticker: kline stream error",
			slog.String("symbol", symbol),
			slog.String("interval", interval),
			slog.Any("error", err),
		)
	}

	doneC, sdkStopC, err := futures.WsKlineServe(symbol, interval, wsHandler, errHandler)
	if err != nil {
		ch.logger.Error("market_ticker: WsKlineServe failed",
			slog.String("symbol", symbol),
			slog.String("interval", interval),
			slog.Any("error", err),
		)
		return
	}

	select {
	case <-ctx.Done():
		sdkStopC <- struct{}{}
		<-doneC
	case <-doneC:
		ch.logger.Info("market_ticker: kline stream closed by remote",
			slog.String("symbol", symbol),
			slog.String("interval", interval),
		)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// parseDecimalString converts a Binance decimal string (e.g. "64520.50000000")
// to float64 for JSON serialisation.
//
// Returns 0 on parse failure. Under normal Binance WS operation every numeric
// field is a valid decimal string; a zero fallback is safer than panicking and
// is clearly distinguishable in the client UI.
func parseDecimalString(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

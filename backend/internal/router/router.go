package router

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/kh0anh/quantflow/config"
	"github.com/kh0anh/quantflow/internal/engine/bot"
	"github.com/kh0anh/quantflow/internal/exchange"
	"github.com/kh0anh/quantflow/internal/handler"
	"github.com/kh0anh/quantflow/internal/logic"
	appMiddleware "github.com/kh0anh/quantflow/internal/middleware"
	"github.com/kh0anh/quantflow/internal/repository"
	pkgcrypto "github.com/kh0anh/quantflow/pkg/crypto"
	"github.com/kh0anh/quantflow/pkg/response"
	"gorm.io/gorm"
)

// Setup constructs and returns the root HTTP handler with all middleware and
// route groups registered. Each route group is stubbed with a TODO comment
// referencing the WBS task that will implement it (Phase 2 tasks).
func Setup(ctx context.Context, db *gorm.DB, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true, // Required for HttpOnly Cookie (JWT)
		MaxAge:           300,  // Preflight cache: 5 minutes
	}))

	// --- Dependency wiring (Clean Architecture: handler → logic → repository) ---
	userRepo := repository.NewUserRepository(db)
	bruteForce := logic.NewBruteForceStore()
	authLogic := logic.NewAuthLogic(userRepo, bruteForce)
	authHandler := handler.NewAuthHandler(authLogic, cfg)
	// WBS 2.1.6: account profile management
	accountLogic := logic.NewAccountLogic(userRepo)
	accountHandler := handler.NewAccountHandler(accountLogic, cfg)

	r.Route("/api/v1", func(r chi.Router) {

		// Health check — consumed by Docker HEALTHCHECK and monitoring
		r.Get("/health", healthHandler)

		// /auth — mixed-auth: /login is public, /logout|/me|/refresh require JWT.
		// All four endpoints share the same /auth prefix so they are consolidated
		// into a single r.Route to avoid Chi's duplicate-Mount panic (Chi's
		// r.Group shares the parent Mux, making a second r.Route("/auth") illegal).
		// WBS 2.1.1: POST /auth/login    (public)
		// WBS 2.1.2: POST /auth/logout   (JWT required)
		// WBS 2.1.3: GET  /auth/me       (JWT required)
		// WBS 2.1.4: POST /auth/refresh  (JWT required)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", authHandler.Login)

			r.Group(func(r chi.Router) {
				r.Use(appMiddleware.JWTAuth(cfg))
				r.Post("/logout", authHandler.Logout)
				r.Get("/me", authHandler.Me)
				r.Post("/refresh", authHandler.Refresh)
			})
		})

		// --- Protected routes — JWT cookie required (WBS 2.1.5) ---
		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.JWTAuth(cfg))

			// TODO(dev): Mount account handler — PUT /account/profile (WBS 2.1.6)
			r.Route("/account", func(r chi.Router) {
				r.Put("/profile", accountHandler.UpdateProfile)
			})

			// WBS 2.2.1: POST   /exchange/api-keys (AES-256-GCM + Binance ping-verify) ✓
			// WBS 2.2.2: GET    /exchange/api-keys (masked api_key, write-only secret)  ✓
			// WBS 2.2.3: DELETE /exchange/api-keys (running-bot 409 constraint)         ✓
			// WBS 2.2.5: Token Bucket rate limiter — singleton shared by all BinanceProxy instances ✓
			exchangeLimiter := exchange.NewExchangeRateLimiter()
			apiKeyRepo := repository.NewApiKeyRepository(db)
			apiKeyLogic := logic.NewApiKeyLogic(apiKeyRepo, pkgcrypto.DeriveKey(cfg.AESKey), exchangeLimiter)
			apiKeyHandler := handler.NewApiKeyHandler(apiKeyLogic)
			r.Route("/exchange", func(r chi.Router) {
				r.Post("/api-keys", apiKeyHandler.Save)
				r.Get("/api-keys", apiKeyHandler.Get)
				r.Delete("/api-keys", apiKeyHandler.Delete)
			})

			// WBS 2.3.1: GET    /strategies (page pagination + ILIKE search) ✓
			// WBS 2.3.2: POST   /strategies (event_on_candle validation + version 1) ✓
			// WBS 2.3.3: GET    /strategies/{id} (detail + active Bot warning) ✓
			// WBS 2.3.4: PUT    /strategies/{id} (auto version_number++) ✓
			// WBS 2.3.5: DELETE /strategies/{id} (check Running Bot then 409) ✓
			// WBS 2.3.6: POST   /strategies/import (Validate JSON Schema) ✓
			// WBS 2.3.7: GET    /strategies/{id}/export (Content-Disposition) ✓
			strategyRepo := repository.NewStrategyRepository(db)
			strategyLogic := logic.NewStrategyLogic(strategyRepo)
			strategyHandler := handler.NewStrategyHandler(strategyLogic)
			r.Route("/strategies", func(r chi.Router) {
				r.Get("/", strategyHandler.List)
				r.Post("/", strategyHandler.Create)
				r.Post("/import", strategyHandler.Import)
				r.Get("/{id}", strategyHandler.Get)
				r.Get("/{id}/export", strategyHandler.Export)
				r.Put("/{id}", strategyHandler.Update)
				r.Delete("/{id}", strategyHandler.Delete)
			})

			// WBS 2.4.1: Hybrid Data Sync — Binance WS + REST fallback, INSERT candles ✓
			// CandleRepository and KlineSyncService are wired here and will be injected
			// into MarketLogic (WBS 2.4.3-2.4.4) and BacktestLogic (WBS 2.6.1).
			candleRepo := repository.NewCandleRepository(db)
			klineSyncSvc := exchange.NewKlineSyncService(candleRepo, exchangeLimiter)
			// WBS 2.4.1: subscribe to all watched symbols from WATCHED_SYMBOLS env on startup.
			go klineSyncSvc.StartWatchedSymbols(ctx, cfg.WatchedSymbols)

			// WBS 2.4.2: background worker that periodically detects and backfills
			// missing candle ranges caused by WS disconnections or server restarts.
			// Shares the same CandleRepository and ExchangeRateLimiter to ensure
			// idempotent inserts and respect the Binance IP weight cap.
			gapFiller := exchange.NewGapFillerWorker(candleRepo, exchangeLimiter, cfg.WatchedSymbols)
			go gapFiller.Run(ctx)

			// WBS 2.7.1: Initialize Bot Manager and Bot Event Listener (two-phase init)
			// for Live Trade bot orchestration. Task 2.7.5 wires the CRUD handlers below.
			varRepo := repository.NewBotLifecycleVarRepository(db)
			logRepo := repository.NewBotLogRepository(db)
			botManager := bot.NewBotManager(db, slog.Default(), varRepo, logRepo, nil)
			botListener := bot.NewBotEventListener(ctx, botManager, slog.Default())
			botManager.SetListener(botListener)

			// WBS 2.6.5: POST /backtests (async launch), GET /backtests/{id} (poll),
			// POST /backtests/{id}/cancel. Results held in-memory (no DB persistence).
			backtestLogic := logic.NewBacktestLogic(strategyRepo, candleRepo, nil)
			backtestHandler := handler.NewBacktestHandler(backtestLogic)
			r.Route("/backtests", func(r chi.Router) {
				r.Post("/", backtestHandler.Create)
				r.Get("/{id}", backtestHandler.Get)
				r.Post("/{id}/cancel", backtestHandler.Cancel)
			})

			// WBS 2.7.5: Bot CRUD APIs — GET/POST/DELETE /bots, GET /bots/{id} ✓
			// POST creates Bot and runs immediately (Running status)
			botRepo := repository.NewBotRepository(db)
			botLogic := logic.NewBotLogic(
				botRepo,
				strategyRepo,
				apiKeyRepo,
				candleRepo,
				botManager,
				pkgcrypto.DeriveKey(cfg.AESKey),
				exchangeLimiter,
			)
			botHandler := handler.NewBotHandler(botLogic)
			r.Route("/bots", func(r chi.Router) {
				r.Get("/", botHandler.List)
				r.Post("/", botHandler.Create)
				r.Get("/{id}", botHandler.Get)
				r.Delete("/{id}", botHandler.Delete)
				// WBS 2.7.6: Bot Control APIs — Start/Stop (close_position flag) ✓
				r.Post("/{id}/start", botHandler.Start)
				r.Post("/{id}/stop", botHandler.Stop)
			})

			// WBS 2.4.3: GET /market/symbols (24hr ticker — list + price + volume) ✓
			// WBS 2.4.4: GET /market/candles (on-demand sync + trade markers)       ✓
			marketLogic := logic.NewMarketLogic(
				exchangeLimiter,
				candleRepo,
				repository.NewTradeMarkerRepository(db),
			)
			marketHandler := handler.NewMarketHandler(marketLogic, cfg.WatchedSymbols)
			r.Route("/market", func(r chi.Router) {
				r.Get("/symbols", marketHandler.ListSymbols)
				r.Get("/candles", marketHandler.GetCandles)
			})

			// TODO(dev): Mount trade history handlers — GET /trades, GET /trades/export (WBS 2.8.5)
			r.Route("/trades", func(r chi.Router) {
			})

			// TODO(dev): Mount WebSocket upgrade handler — GET /ws (WBS 2.8.1)
			// r.Get("/ws", wsHandler.ServeWS)
		})
	})

	return r
}

// healthHandler responds to Docker HEALTHCHECK and uptime probes.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": "1.0.0",
	})
}

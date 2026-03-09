package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/kh0anh/quantflow/config"
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
func Setup(db *gorm.DB, cfg *config.Config) http.Handler {
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

		// --- Public auth routes (no JWT required) ---
		// WBS 2.1.1: POST /auth/login (implemented)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", authHandler.Login)
		})

		// --- Protected routes — JWT cookie required (WBS 2.1.5) ---
		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.JWTAuth(cfg))

			// Auth protected endpoints
			// WBS 2.1.2: POST /auth/logout  (implemented)
			// WBS 2.1.3: GET  /auth/me      (implemented)
			// WBS 2.1.4: POST /auth/refresh (implemented)
			r.Route("/auth", func(r chi.Router) {
				r.Post("/logout", authHandler.Logout)
				r.Get("/me", authHandler.Me)
				r.Post("/refresh", authHandler.Refresh)
			})

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

			// WBS 2.3.1: GET  /strategies (page pagination + ILIKE search) ✓
			// WBS 2.3.2: POST /strategies (event_on_candle validation + version 1) ✓
			// WBS 2.3.3: GET  /strategies/{id} (detail + active Bot warning) ✓
			// WBS 2.3.4: PUT  /strategies/{id} (auto version_number++) ✓
			strategyRepo := repository.NewStrategyRepository(db)
			strategyLogic := logic.NewStrategyLogic(strategyRepo)
			strategyHandler := handler.NewStrategyHandler(strategyLogic)
			r.Route("/strategies", func(r chi.Router) {
				r.Get("/", strategyHandler.List)
				r.Post("/", strategyHandler.Create)
				r.Get("/{id}", strategyHandler.Get)
				r.Put("/{id}", strategyHandler.Update)
			})

			// TODO(dev): Mount backtest handlers — POST/GET/cancel /backtests (WBS 2.6.5)
			r.Route("/backtests", func(r chi.Router) {
			})

			// TODO(dev): Mount bot handlers — CRUD + control + logs /bots (WBS 2.7.5-2.7.7)
			r.Route("/bots", func(r chi.Router) {
			})

			// TODO(dev): Mount market data handlers — GET /market/symbols, GET /market/candles (WBS 2.4.3-2.4.4)
			r.Route("/market", func(r chi.Router) {
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

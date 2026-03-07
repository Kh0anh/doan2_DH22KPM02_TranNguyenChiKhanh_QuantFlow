package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/kh0anh/quantflow/config"
	"github.com/kh0anh/quantflow/pkg/response"
	"gorm.io/gorm"
)

// Setup constructs and returns the root HTTP handler with all middleware and
// route groups registered. Each route group is stubbed with a TODO comment
// referencing the WBS task that will implement it (Phase 2 tasks).
func Setup(db *gorm.DB, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true, // Required for HttpOnly Cookie (JWT)
		MaxAge:           300,  // Preflight cache: 5 minutes
	}))

	r.Route("/api/v1", func(r chi.Router) {

		// Health check — consumed by Docker HEALTHCHECK and monitoring
		r.Get("/health", healthHandler)

		// TODO(dev): Mount auth handlers — POST /auth/login, POST /auth/logout, GET /auth/me, POST /auth/refresh (WBS 2.1.1-2.1.4)
		r.Route("/auth", func(r chi.Router) {
		})

		// TODO(dev): Mount account handler — PUT /account/profile (WBS 2.1.6)
		r.Route("/account", func(r chi.Router) {
		})

		// TODO(dev): Mount exchange API-key handlers — POST/GET/DELETE /exchange/api-keys (WBS 2.2.1-2.2.3)
		r.Route("/exchange", func(r chi.Router) {
		})

		// TODO(dev): Mount strategy handlers — CRUD + import/export /strategies (WBS 2.3.1-2.3.7)
		r.Route("/strategies", func(r chi.Router) {
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

	return r
}

// healthHandler responds to Docker HEALTHCHECK and uptime probes.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": "1.0.0",
	})
}

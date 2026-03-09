package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/kh0anh/quantflow/config"
	"github.com/kh0anh/quantflow/database"
	"github.com/kh0anh/quantflow/internal/router"
	"github.com/kh0anh/quantflow/pkg/logger"
)

// main initialises configuration, opens the database connection, wires the
// HTTP handler, and starts the server with graceful shutdown on SIGINT/SIGTERM.
func main() {
	cfg := config.Load()
	logger.Setup(cfg)

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Fatal("database connection failed", "component", "server", "error", err)
	}

	// Listen for OS signal before wiring the router so that the server-lifetime
	// context is available for long-running background workers (e.g. kline WS
	// streams started by StartWatchedSymbols — WBS 2.4.1).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	handler := router.Setup(ctx, db, cfg)

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine so the main goroutine can wait for signal
	go func() {
		slog.Info("QuantFlow backend listening", "component", "server", "port", cfg.ServerPort, "env", cfg.GoEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("ListenAndServe error", "component", "server", "error", err)
		}
	}()

	<-ctx.Done()
	stop()
	slog.Info("shutdown signal received, draining connections", "component", "server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "component", "server", "error", err)
	} else {
		slog.Info("server stopped cleanly", "component", "server")
	}
}

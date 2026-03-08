package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/kh0anh/quantflow/config"
	"github.com/kh0anh/quantflow/database"
	"github.com/kh0anh/quantflow/internal/router"
)

// main is the root entry point for `go run .` during local development.
// The canonical Dockerfile build target remains cmd/server/main.go.
func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("[server] Database connection failed: %v", err)
	}

	handler := router.Setup(db, cfg)

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("[server] QuantFlow backend listening on :%s (env=%s)", cfg.ServerPort, cfg.GoEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[server] ListenAndServe error: %v", err)
		}
	}()

	<-ctx.Done()
	stop()
	log.Println("[server] Shutdown signal received, draining connections...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[server] Graceful shutdown failed: %v", err)
	} else {
		log.Println("[server] Server stopped cleanly.")
	}
}

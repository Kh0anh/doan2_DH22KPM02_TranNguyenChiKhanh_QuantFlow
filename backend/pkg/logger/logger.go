package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/kh0anh/quantflow/config"
)

// Setup configures the default slog logger based on runtime environment.
// Must be called once, immediately after config.Load(), before any other
// package emits log output.
//
// Handler selection (tech_stack.md §2.2):
//   - GO_ENV == "development" → TextHandler (stdout) — human-readable output.
//   - GO_ENV != "development" → JSONHandler (stdout) — structured JSON for
//     Docker json-file log driver (docker-compose.yml).
//
// Log level is read from cfg.LogLevel (LOG_LEVEL env var, default "info").
func Setup(cfg *config.Config) {
	level := parseLevel(cfg.LogLevel)
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.GoEnv == "development" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// Fatal logs the message at Error level and terminates the process with
// exit code 1. Use in place of log.Fatal / log.Fatalf where the program
// cannot safely continue (e.g. missing required secrets — NFR-SEC-02).
func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

// parseLevel converts a LOG_LEVEL string to the corresponding slog.Level.
// Unrecognised values default to slog.LevelInfo.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

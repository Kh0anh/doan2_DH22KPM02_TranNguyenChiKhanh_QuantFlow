package config

import (
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	// Server
	ServerPort string
	GoEnv      string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Security
	JWTSecret string
	AESKey    string

	// CORS
	CORSAllowedOrigins []string

	// Market Watch — comma-separated list of Binance Futures symbol pairs
	// the backend subscribes to on startup (WBS 2.4.1).
	// Example: BTCUSDT,ETHUSDT,SOLUSDT,BNBUSDT
	WatchedSymbols []string

	// Exchange
	BinanceBaseURL string // BINANCE_API_BASE — override Binance Futures REST base URL (testnet/proxy)

	// Logging
	LogLevel string // LOG_LEVEL env var: debug | info | warn | error (default: info)
}

// Load reads configuration from environment variables.
// If a .env file exists, it is loaded first (dev convenience).
// Missing values fall back to the same defaults defined in docker-compose.yml.
func Load() *Config {
	// Best-effort: load .env in development; ignore error in production/Docker.
	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, reading from environment", "component", "config")
	}

	cfg := &Config{
		GoEnv:      getEnv("GO_ENV", "development"),
		ServerPort: getEnv("SERVER_PORT", "8080"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "quantflow_user"),
		DBPassword: getEnv("DB_PASSWORD", "quantflow_secure_password_2026"),
		DBName:     getEnv("DB_NAME", "quantflow_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		JWTSecret: getEnv("JWT_SECRET", ""),
		AESKey:    getEnv("AES_KEY", ""),

		LogLevel: getEnv("LOG_LEVEL", "info"),

		BinanceBaseURL: getEnv("BINANCE_API_BASE", ""), // empty = use go-binance library default
	}

	rawOrigins := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost,http://localhost:3000")
	cfg.CORSAllowedOrigins = splitTrim(rawOrigins, ",")

	rawSymbols := getEnv("WATCHED_SYMBOLS", "BTCUSDT,ETHUSDT,SOLUSDT,BNBUSDT")
	cfg.WatchedSymbols = splitTrim(rawSymbols, ",")

	// Guard: secrets must not be empty in non-development environments
	if cfg.GoEnv != "development" {
		if cfg.JWTSecret == "" {
			slog.Error("JWT_SECRET must be set in non-development environments", "component", "config")
			os.Exit(1)
		}
		if cfg.AESKey == "" {
			slog.Error("AES_KEY must be set in non-development environments", "component", "config")
			os.Exit(1)
		}
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

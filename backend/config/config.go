package config

import (
	"log"
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
}

// Load reads configuration from environment variables.
// If a .env file exists, it is loaded first (dev convenience).
// Missing values fall back to the same defaults defined in docker-compose.yml.
func Load() *Config {
	// Best-effort: load .env in development; ignore error in production/Docker.
	if err := godotenv.Load(); err != nil {
		log.Println("[config] No .env file found, reading from environment")
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
	}

	rawOrigins := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost,http://localhost:3000")
	cfg.CORSAllowedOrigins = splitTrim(rawOrigins, ",")

	// Guard: secrets must not be empty in non-development environments
	if cfg.GoEnv != "development" {
		if cfg.JWTSecret == "" {
			log.Fatal("[config] JWT_SECRET must be set in non-development environments")
		}
		if cfg.AESKey == "" {
			log.Fatal("[config] AES_KEY must be set in non-development environments")
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

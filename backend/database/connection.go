package database

import (
	"fmt"
	"log"
	"time"

	"github.com/kh0anh/quantflow/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens a PostgreSQL connection using GORM with the pgx v5 driver.
// Connection pool parameters are tuned for the 5-parallel-bot workload (NFR-PERF).
func Connect(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	logLevel := logger.Silent
	if cfg.GoEnv == "development" {
		logLevel = logger.Warn
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logLevel),
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		return nil, fmt.Errorf("database: failed to open connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database: failed to get sql.DB: %w", err)
	}

	// Connection pool settings — supports 5 concurrent bots + API handlers
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	// Verify connectivity immediately so startup fails fast on misconfiguration
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("database: ping failed: %w", err)
	}

	log.Printf("[database] Connected to PostgreSQL at %s:%s/%s", cfg.DBHost, cfg.DBPort, cfg.DBName)
	return db, nil
}

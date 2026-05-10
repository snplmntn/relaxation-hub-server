package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig holds database pool configuration
type PoolConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// DefaultPoolConfig returns sensible defaults for production use
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConns:          25,
		MinConns:          5,
		MaxConnLifetime:   1 * time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
		HealthCheckPeriod: 1 * time.Minute,
	}
}

// LoadPoolConfigFromEnv loads pool configuration from environment variables
func LoadPoolConfigFromEnv() PoolConfig {
	cfg := DefaultPoolConfig()

	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxConns = int32(n)
		}
	}
	if v := os.Getenv("DB_MIN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.MinConns = int32(n)
		}
	}
	if v := os.Getenv("DB_MAX_CONN_LIFETIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.MaxConnLifetime = d
		}
	}
	if v := os.Getenv("DB_MAX_CONN_IDLE_TIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.MaxConnIdleTime = d
		}
	}

	return cfg
}

func InitDB(connString string) (*pgxpool.Pool, error) {
	if connString == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	// Parse the connection string to get a config we can modify
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection string: %v", err)
	}

	// Apply pool configuration from environment
	envConfig := LoadPoolConfigFromEnv()
	poolConfig.MaxConns = envConfig.MaxConns
	poolConfig.MinConns = envConfig.MinConns
	poolConfig.MaxConnLifetime = envConfig.MaxConnLifetime
	poolConfig.MaxConnIdleTime = envConfig.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = envConfig.HealthCheckPeriod

	slog.Info("Database pool config",
		"max_conns", poolConfig.MaxConns,
		"min_conns", poolConfig.MinConns,
		"max_conn_lifetime", poolConfig.MaxConnLifetime,
		"max_conn_idle_time", poolConfig.MaxConnIdleTime)

	// Create pool with configured settings
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to the database: %v", err)
	}

	err = pool.Ping(context.Background())
	if err != nil {
		return nil, fmt.Errorf("database ping failed: %v", err)
	}

	fmt.Println("Database connection successful.")
	return pool, nil
}

func CloseDB(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
		fmt.Println("Database connection closed.")
	}
}

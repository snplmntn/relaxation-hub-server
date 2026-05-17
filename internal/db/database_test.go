package db

import (
	"testing"
	"time"
)

func TestDefaultPoolConfig(t *testing.T) {
	cfg := DefaultPoolConfig()

	if cfg.MaxConns != 10 {
		t.Fatalf("MaxConns = %d, want 10", cfg.MaxConns)
	}
	if cfg.MinConns != 0 {
		t.Fatalf("MinConns = %d, want 0", cfg.MinConns)
	}
	if cfg.MaxConnLifetime != time.Hour {
		t.Fatalf("MaxConnLifetime = %s, want 1h", cfg.MaxConnLifetime)
	}
	if cfg.MaxConnIdleTime != 5*time.Minute {
		t.Fatalf("MaxConnIdleTime = %s, want 5m", cfg.MaxConnIdleTime)
	}
	if cfg.HealthCheckPeriod != time.Minute {
		t.Fatalf("HealthCheckPeriod = %s, want 1m", cfg.HealthCheckPeriod)
	}
	if cfg.MinConns > cfg.MaxConns {
		t.Fatalf("MinConns = %d, MaxConns = %d, want MinConns <= MaxConns", cfg.MinConns, cfg.MaxConns)
	}
}

func TestLoadPoolConfigFromEnv(t *testing.T) {
	t.Run("applies valid overrides", func(t *testing.T) {
		t.Setenv("DB_MAX_CONNS", "12")
		t.Setenv("DB_MIN_CONNS", "4")
		t.Setenv("DB_MAX_CONN_LIFETIME", "2h")
		t.Setenv("DB_MAX_CONN_IDLE_TIME", "7m")

		cfg := LoadPoolConfigFromEnv()

		if cfg.MaxConns != 12 {
			t.Fatalf("MaxConns = %d, want 12", cfg.MaxConns)
		}
		if cfg.MinConns != 4 {
			t.Fatalf("MinConns = %d, want 4", cfg.MinConns)
		}
		if cfg.MaxConnLifetime != 2*time.Hour {
			t.Fatalf("MaxConnLifetime = %s, want 2h", cfg.MaxConnLifetime)
		}
		if cfg.MaxConnIdleTime != 7*time.Minute {
			t.Fatalf("MaxConnIdleTime = %s, want 7m", cfg.MaxConnIdleTime)
		}
		if cfg.HealthCheckPeriod != time.Minute {
			t.Fatalf("HealthCheckPeriod = %s, want 1m", cfg.HealthCheckPeriod)
		}
		if cfg.MinConns > cfg.MaxConns {
			t.Fatalf("MinConns = %d, MaxConns = %d, want MinConns <= MaxConns", cfg.MinConns, cfg.MaxConns)
		}
	})

	t.Run("ignores invalid overrides", func(t *testing.T) {
		t.Setenv("DB_MAX_CONNS", "not-a-number")
		t.Setenv("DB_MIN_CONNS", "-1")
		t.Setenv("DB_MAX_CONN_LIFETIME", "bad-duration")
		t.Setenv("DB_MAX_CONN_IDLE_TIME", "also-bad")

		cfg := LoadPoolConfigFromEnv()
		want := DefaultPoolConfig()

		if cfg != want {
			t.Fatalf("LoadPoolConfigFromEnv() = %+v, want %+v", cfg, want)
		}
	})

	t.Run("clamps min conns to max conns", func(t *testing.T) {
		t.Setenv("DB_MAX_CONNS", "3")
		t.Setenv("DB_MIN_CONNS", "9")

		cfg := LoadPoolConfigFromEnv()

		if cfg.MaxConns != 3 {
			t.Fatalf("MaxConns = %d, want 3", cfg.MaxConns)
		}
		if cfg.MinConns != 3 {
			t.Fatalf("MinConns = %d, want 3", cfg.MinConns)
		}
		if cfg.MinConns > cfg.MaxConns {
			t.Fatalf("MinConns = %d, MaxConns = %d, want MinConns <= MaxConns", cfg.MinConns, cfg.MaxConns)
		}
	})
}

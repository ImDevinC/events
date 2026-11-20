package config_test

import (
	"testing"
	"time"

	"events/internal/config"
)

// TestLoadUsesEnvironmentValues verifies that configuration comes from environment variables.
func TestLoadUsesEnvironmentValues(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DATABASE_URL", "postgres://tester:secret@localhost:5432/events")
	t.Setenv("APP_TIMEZONE", "America/New_York")
	t.Setenv("URL_FETCH_TIMEOUT", "3s")
	t.Setenv("HTTP_READ_TIMEOUT", "4s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "5s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "6s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "7s")
	t.Setenv("DATABASE_HEALTH_TIMEOUT", "2s")
	t.Setenv("DATABASE_STARTUP_TIMEOUT", "8s")
	t.Setenv("DATABASE_CONN_MAX_LIFETIME", "25m")
	t.Setenv("DATABASE_MAX_OPEN_CONNS", "8")
	t.Setenv("DATABASE_MAX_IDLE_CONNS", "2")

	settings, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned an error: %v", err)
	}
	if settings.Address != ":9090" || settings.DatabaseURL != "postgres://tester:secret@localhost:5432/events" || settings.Timezone != "America/New_York" {
		t.Fatalf("Load() returned incorrect settings: %#v", settings)
	}
	if settings.FetchTimeout != 3*time.Second || settings.HTTPReadTimeout != 4*time.Second || settings.HTTPWriteTimeout != 5*time.Second || settings.HTTPIdleTimeout != 6*time.Second || settings.ShutdownTimeout != 7*time.Second || settings.HealthTimeout != 2*time.Second || settings.StartupTimeout != 8*time.Second || settings.DatabaseConnLifetime != 25*time.Minute {
		t.Fatalf("Load() returned incorrect timeouts: %#v", settings)
	}
	if settings.DatabaseMaxOpen != 8 || settings.DatabaseMaxIdle != 2 {
		t.Fatalf("Load() returned incorrect database limits: %#v", settings)
	}
}

// TestLoadRejectsInvalidDatabaseLimits verifies that invalid pool sizes fail configuration.
func TestLoadRejectsInvalidDatabaseLimits(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("DATABASE_MAX_OPEN_CONNS", "2")
	t.Setenv("DATABASE_MAX_IDLE_CONNS", "3")
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() accepted an idle connection limit above the open limit")
	}
}

func setValidEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_URL", "postgres://events:events@localhost:5432/events")
	t.Setenv("APP_TIMEZONE", "UTC")
	t.Setenv("URL_FETCH_TIMEOUT", "8s")
	t.Setenv("HTTP_READ_TIMEOUT", "10s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "20s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "60s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "10s")
	t.Setenv("DATABASE_HEALTH_TIMEOUT", "2s")
	t.Setenv("DATABASE_STARTUP_TIMEOUT", "10s")
	t.Setenv("DATABASE_CONN_MAX_LIFETIME", "30m")
	t.Setenv("DATABASE_MAX_OPEN_CONNS", "12")
	t.Setenv("DATABASE_MAX_IDLE_CONNS", "4")
}

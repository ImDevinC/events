package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config contains the runtime settings for the events service.
type Config struct {
	Address              string        // Address is the HTTP listen address.
	DatabaseURL          string        // DatabaseURL is the PostgreSQL connection URL.
	FetchTimeout         time.Duration // FetchTimeout limits event page requests.
	HTTPReadTimeout      time.Duration // HTTPReadTimeout limits incoming HTTP requests.
	HTTPWriteTimeout     time.Duration // HTTPWriteTimeout limits HTTP responses.
	HTTPIdleTimeout      time.Duration // HTTPIdleTimeout limits keep-alive connections.
	ShutdownTimeout      time.Duration // ShutdownTimeout limits graceful server shutdown.
	HealthTimeout        time.Duration // HealthTimeout limits database health checks.
	StartupTimeout       time.Duration // StartupTimeout limits database startup and migrations.
	DatabaseConnLifetime time.Duration // DatabaseConnLifetime limits the age of a pooled connection.
	DatabaseMaxOpen      int           // DatabaseMaxOpen is the maximum number of open database connections.
	DatabaseMaxIdle      int           // DatabaseMaxIdle is the maximum number of idle database connections.
	Timezone             string        // Timezone is the timezone used for event dates.
}

// Load reads the runtime settings from environment variables.
func Load() (Config, error) {
	config := Config{
		Address:              environment("HTTP_ADDR", ":8080"),
		DatabaseURL:          environment("DATABASE_URL", "postgres://events:events@localhost:5432/events?sslmode=disable"),
		Timezone:             environment("APP_TIMEZONE", "UTC"),
		FetchTimeout:         durationEnvironment("URL_FETCH_TIMEOUT", 8*time.Second),
		HTTPReadTimeout:      durationEnvironment("HTTP_READ_TIMEOUT", 10*time.Second),
		HTTPWriteTimeout:     durationEnvironment("HTTP_WRITE_TIMEOUT", 20*time.Second),
		HTTPIdleTimeout:      durationEnvironment("HTTP_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:      durationEnvironment("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
		HealthTimeout:        durationEnvironment("DATABASE_HEALTH_TIMEOUT", 2*time.Second),
		StartupTimeout:       durationEnvironment("DATABASE_STARTUP_TIMEOUT", 10*time.Second),
		DatabaseConnLifetime: durationEnvironment("DATABASE_CONN_MAX_LIFETIME", 30*time.Minute),
		DatabaseMaxOpen:      intEnvironment("DATABASE_MAX_OPEN_CONNS", 12),
		DatabaseMaxIdle:      intEnvironment("DATABASE_MAX_IDLE_CONNS", 4),
	}
	if config.Address == "" {
		return Config{}, errors.New("HTTP_ADDR cannot be empty")
	}
	if config.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL cannot be empty")
	}
	if config.FetchTimeout <= 0 || config.HTTPReadTimeout <= 0 || config.HTTPWriteTimeout <= 0 || config.HTTPIdleTimeout <= 0 || config.ShutdownTimeout <= 0 || config.HealthTimeout <= 0 || config.StartupTimeout <= 0 || config.DatabaseConnLifetime <= 0 {
		return Config{}, errors.New("HTTP and URL timeout values must be greater than zero")
	}
	if config.DatabaseMaxOpen < 1 || config.DatabaseMaxIdle < 0 || config.DatabaseMaxIdle > config.DatabaseMaxOpen {
		return Config{}, errors.New("database connection limits are invalid")
	}
	if _, err := time.LoadLocation(config.Timezone); err != nil {
		return Config{}, fmt.Errorf("APP_TIMEZONE: %w", err)
	}
	return config, nil
}

func environment(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func durationEnvironment(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return duration
}

func intEnvironment(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

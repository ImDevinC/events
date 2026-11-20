package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"events/internal/config"
	"events/internal/metadata"
	"events/internal/store"
	"events/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("events stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	settings, err := config.Load()
	if err != nil {
		return err
	}
	location, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return err
	}
	startupContext, cancelStartup := context.WithTimeout(context.Background(), settings.StartupTimeout)
	repository, err := store.Open(startupContext, settings.DatabaseURL, settings.DatabaseMaxOpen, settings.DatabaseMaxIdle, settings.DatabaseConnLifetime)
	cancelStartup()
	if err != nil {
		return err
	}
	defer repository.Close()

	migrationContext, cancelMigration := context.WithTimeout(context.Background(), settings.StartupTimeout)
	if err := repository.Migrate(migrationContext); err != nil {
		cancelMigration()
		return err
	}
	cancelMigration()

	handler, err := web.NewHandler(repository, metadata.NewFetcher(settings.FetchTimeout), location, settings.HealthTimeout)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              settings.Address,
		Handler:           handler,
		ReadHeaderTimeout: settings.HTTPReadTimeout,
		ReadTimeout:       settings.HTTPReadTimeout,
		WriteTimeout:      settings.HTTPWriteTimeout,
		IdleTimeout:       settings.HTTPIdleTimeout,
	}
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdownContext.Done()
		ctx, cancel := context.WithTimeout(context.Background(), settings.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	}()

	slog.Info("events server started", "address", settings.Address, "timezone", settings.Timezone)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

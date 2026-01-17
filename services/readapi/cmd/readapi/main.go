package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sylvain/realtoragent/services/readapi/internal/config"
	"github.com/sylvain/realtoragent/services/readapi/internal/db"
	"github.com/sylvain/realtoragent/services/readapi/internal/httpapi"
	"github.com/sylvain/realtoragent/services/readapi/internal/listings"
	"github.com/sylvain/realtoragent/services/readapi/internal/listings/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	var dbClient *db.DB
	var listingsService *listings.Service
	var dbPing func(context.Context) error

	if cfg.DBEnabled {
		connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := db.New(connectCtx, cfg)
		if err != nil {
			log.Fatalf("db connect failed: %v", err)
		}
		dbClient = client
		dbPing = dbClient.Ping
		listingsRepo := postgres.NewListingsRepository(dbClient)
		listingsService = listings.NewService(listingsRepo)
	}

	handler := httpapi.NewHandler(listingsService, cfg.DBEnabled, dbPing)
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpapi.NewRouter(handler, cfg.CORSAllowOrigin),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("readapi starting on %s (log_level=%s)", server.Addr, cfg.LogLevel)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case sig := <-sigCh:
		log.Printf("readapi received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("readapi shutdown error: %v", err)
		}
		if dbClient != nil {
			dbClient.Close()
		}
	case err := <-errCh:
		log.Fatalf("readapi server error: %v", err)
	}
}

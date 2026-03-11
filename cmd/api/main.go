package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	v0Client "github.com/hatchet-dev/hatchet/pkg/client"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/rs/cors"

	"github.com/stroppy-io/hatchet-workflow/internal/auth"
	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
	apiDomain "github.com/stroppy-io/hatchet-workflow/internal/domain/api"
	"github.com/stroppy-io/hatchet-workflow/internal/infrastructure/postgres"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/api/apiconnect"
	"github.com/stroppy-io/hatchet-workflow/internal/store"
)

func main() {
	logger.NewFromEnv()

	ctx := context.Background()

	// Postgres
	pgPort, _ := strconv.Atoi(envOrDefault("PG_PORT", "5432"))
	if pgPort == 0 {
		pgPort = 5432
	}
	pgCfg := &postgres.Config{
		Host:     envOrDefault("PG_HOST", "localhost"),
		Port:     pgPort,
		Username: envOrDefault("PG_USER", "stroppy"),
		Password: envOrDefault("PG_PASSWORD", "stroppy"),
		Database: envOrDefault("PG_DATABASE", "stroppy_api"),
	}
	pool, err := postgres.NewPool(ctx, pgCfg)
	if err != nil {
		log.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	if err := store.Migrate(ctx, pool); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// JWT
	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret: envOrDefault("JWT_SECRET", "stroppy-dev-secret-change-me"),
	})

	// Hatchet client
	hatchetToken := os.Getenv("HATCHET_CLIENT_TOKEN")
	hatchetClient, err := hatchetLib.NewClient(
		v0Client.WithLogger(logger.Zerolog()),
		v0Client.WithToken(hatchetToken),
	)
	if err != nil {
		log.Fatalf("Failed to create hatchet client: %v", err)
	}

	// API handler
	handler := apiDomain.NewHandler(pool, jwtSvc, hatchetClient)
	path, svcHandler := apiconnect.NewStroppyAPIHandler(handler)

	mux := http.NewServeMux()
	mux.Handle(path, apiDomain.AuthMiddleware(svcHandler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// Serve embedded SPA frontend (fallback for all non-API routes).
	mux.Handle("/", spaHandler())

	addr := envOrDefault("API_ADDR", ":8090")
	allowedOrigins := envOrDefault("API_CORS_ORIGINS", "*")

	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{allowedOrigins},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Content-Type",
			"Connect-Protocol-Version",
			"Connect-Timeout-Ms",
			"Grpc-Timeout",
			"X-Grpc-Web",
			"X-User-Agent",
			"Authorization",
		},
		ExposedHeaders: []string{
			"Grpc-Status",
			"Grpc-Message",
			"Grpc-Status-Details-Bin",
		},
		MaxAge: 7200,
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           corsHandler.Handler(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("API server listening on %s", addr)
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Fatalf("API server error: %v", err)
	case sig := <-sigCh:
		log.Printf("Received %s, shutting down", sig)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Graceful shutdown failed: %v", err)
	}
	log.Println("API server stopped")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
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

	// Seed default user from config (skip if not configured).
	if defUser := os.Getenv("DEFAULT_USER"); defUser != "" {
		defPass := envOrDefault("DEFAULT_PASSWORD", "admin")
		defRole := envOrDefault("DEFAULT_ROLE", "admin")
		if err := store.EnsureDefaultUser(ctx, pool, defUser, defPass, defRole); err != nil {
			log.Fatalf("Failed to ensure default user: %v", err)
		}
		log.Printf("Default user ensured: %s (role=%s)", defUser, defRole)
	}

	// Seed built-in workloads and topology templates.
	if err := store.SeedBuiltins(ctx, pool); err != nil {
		log.Fatalf("Failed to seed builtins: %v", err)
	}

	// Seed default settings from environment variables.
	hatchetPort, _ := strconv.Atoi(envOrDefault("HATCHET_PORT", "7077"))
	defaultSettings := &settings.Settings{
		HatchetConnection: &settings.HatchetConnection{
			Host:  envOrDefault("HATCHET_HOST", "localhost"),
			Port:  uint32(hatchetPort),
			Token: os.Getenv("HATCHET_CLIENT_TOKEN"),
		},
		Docker: &settings.DockerSettings{
			NetworkName:     envOrDefault("DOCKER_NETWORK_NAME", "stroppy-net"),
			EdgeWorkerImage: envOrDefault("EDGE_WORKER_IMAGE", "stroppy/edge-worker:latest"),
			NetworkCidr:     strPtr(envOrDefault("DOCKER_NETWORK_CIDR", "172.28.0.0/16")),
			NetworkPrefix:   uint32Ptr(24),
		},
		PreferredTarget: deployment.Target_TARGET_DOCKER,
	}
	if err := store.SeedDefaultSettings(ctx, pool, defaultSettings); err != nil {
		log.Fatalf("Failed to seed default settings: %v", err)
	}
	log.Println("Default settings ensured")

	// JWT
	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret: envOrDefault("JWT_SECRET", "stroppy-dev-secret-change-me"),
	})

	// Hatchet client
	hatchetToken := os.Getenv("HATCHET_CLIENT_TOKEN")
	hatchetOpts := []v0Client.ClientOpt{
		v0Client.WithLogger(logger.Zerolog()),
		v0Client.WithToken(hatchetToken),
	}
	if hostPort := strings.TrimSpace(os.Getenv("HATCHET_CLIENT_HOST_PORT")); hostPort != "" {
		parts := strings.Split(hostPort, ":")
		if len(parts) == 2 {
			if port, err := strconv.Atoi(parts[1]); err == nil {
				hatchetOpts = append(hatchetOpts, v0Client.WithHostPort(parts[0], port))
			}
		}
	} else if hatchetHost := strings.TrimSpace(envOrDefault("HATCHET_HOST", "")); hatchetHost != "" {
		hatchetOpts = append(hatchetOpts, v0Client.WithHostPort(hatchetHost, hatchetPort))
	}

	hatchetV0, err := v0Client.New(hatchetOpts...)
	if err != nil {
		log.Fatalf("Failed to create hatchet v0 client: %v", err)
	}

	hatchetClient, err := hatchetLib.NewClient(hatchetOpts...)
	if err != nil {
		log.Fatalf("Failed to create hatchet client: %v", err)
	}

	// API handler
	handler := apiDomain.NewHandler(pool, jwtSvc, hatchetClient, hatchetV0)
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

func strPtr(s string) *string    { return &s }
func uint32Ptr(v uint32) *uint32 { return &v }

package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/web"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/api"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
)

var (
	configFile string
	dbPath     string
)

func main() {
	root := &cobra.Command{
		Use:           "stroppy-cloud",
		Short:         "Database testing orchestrator",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVarP(&configFile, "config", "c", "run.json", "path to run config JSON")
	root.PersistentFlags().StringVar(&dbPath, "db", "", "PostgreSQL DSN (e.g. postgres://stroppy:stroppy@localhost:5432/stroppy?sslmode=disable)")

	root.AddCommand(
		serveCmd(),
		runCmd(),
		validateCmd(),
		dryRunCmd(),
		agentCmd(),
		cloudCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func serveCmd() *cobra.Command {
	var addr string
	var jwtSecret string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server (agent API + external API + WS)",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbDSN := envOrDefault("DATABASE_URL", dbPath)
			if dbDSN == "" {
				dbDSN = "postgres://stroppy:stroppy@localhost:5432/stroppy?sslmode=disable"
			}

			jwtSec := envOrDefault("JWT_SECRET", jwtSecret)
			if jwtSec == "" {
				jwtSec = "stroppy-dev-secret"
			}

			monitoringURL := os.Getenv("MONITORING_URL")     // empty = monitoring disabled
			monitoringToken := os.Getenv("MONITORING_TOKEN") // bearer token for vmauth
			grafanaURL := os.Getenv("GRAFANA_URL")           // empty = grafana disabled
			listenAddr := envOrDefault("LISTEN_ADDR", addr)

			ctx := context.Background()
			pool, err := postgres.Open(ctx, dbDSN)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer pool.Close()

			logger, _ := zap.NewDevelopment()
			app := api.New(api.Config{Pool: pool, Logger: logger})
			srv := api.NewServer(app, logger, pool, jwtSec, monitoringURL, monitoringToken, grafanaURL, listenAddr)
			srv.CleanupOrphanedRuns()

			// Embed SPA into the server.
			spaFS, err := fs.Sub(web.Dist, "dist")
			if err == nil {
				srv.SetSPA(spaFS)
				logger.Info("SPA embedded and served at /")
			}

			httpSrv := &http.Server{Addr: listenAddr, Handler: srv.Router()}

			go func() {
				logger.Info("server listening", zap.String("addr", listenAddr))
				if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Fatal("server error", zap.Error(err))
				}
			}()

			sigCtx := signalCtx()
			<-sigCtx.Done()
			logger.Info("shutting down")
			return httpSrv.Shutdown(context.Background())
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "listen address")
	cmd.Flags().StringVar(&jwtSecret, "jwt-secret", "", "JWT signing secret (default: stroppy-dev-secret)")
	return cmd
}

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Execute a full test run from config (local, no server)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := api.LoadConfig(configFile)
			if err != nil {
				return err
			}

			dbDSN := envOrDefault("DATABASE_URL", dbPath)
			if dbDSN == "" {
				return fmt.Errorf("--db flag or DATABASE_URL env is required (PostgreSQL DSN)")
			}

			ctx := signalCtx()
			pool, err := postgres.Open(ctx, dbDSN)
			if err != nil {
				return err
			}
			defer pool.Close()

			logger, _ := zap.NewDevelopment()
			app := api.New(api.Config{Pool: pool, Logger: logger})

			// CLI runs use an empty tenant ID.
			return app.Start(ctx, "", cfg)
		},
	}
}

func validateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate a run config without executing",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := api.LoadConfig(configFile)
			if err != nil {
				return err
			}

			dbDSN := envOrDefault("DATABASE_URL", dbPath)
			if dbDSN == "" {
				return fmt.Errorf("--db flag or DATABASE_URL env is required (PostgreSQL DSN)")
			}

			ctx := context.Background()
			pool, err := postgres.Open(ctx, dbDSN)
			if err != nil {
				return err
			}
			defer pool.Close()

			logger, _ := zap.NewDevelopment()
			app := api.New(api.Config{Pool: pool, Logger: logger})

			if err := app.Validate(cfg); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
			fmt.Println("config is valid")
			return nil
		},
	}
}

func dryRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dry-run",
		Short: "Print the execution DAG as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := api.LoadConfig(configFile)
			if err != nil {
				return err
			}

			dbDSN := envOrDefault("DATABASE_URL", dbPath)
			if dbDSN == "" {
				return fmt.Errorf("--db flag or DATABASE_URL env is required (PostgreSQL DSN)")
			}

			ctx := context.Background()
			pool, err := postgres.Open(ctx, dbDSN)
			if err != nil {
				return err
			}
			defer pool.Close()

			logger, _ := zap.NewDevelopment()
			app := api.New(api.Config{Pool: pool, Logger: logger})

			data, err := app.DryRun(cfg)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	}
}

func agentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run in agent mode on a target machine (polls server for commands)",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverAddr := os.Getenv("STROPPY_SERVER_ADDR")

			machineID := os.Getenv("STROPPY_MACHINE_ID")
			if machineID == "" {
				h, err := os.Hostname()
				if err != nil {
					return fmt.Errorf("determine machine ID: %w", err)
				}
				machineID = h
			}

			srv := agent.NewAgentServer(serverAddr, machineID, 0)

			// Set auth token if provided (generated by server at agent provisioning).
			if agentToken := os.Getenv("STROPPY_AGENT_TOKEN"); agentToken != "" {
				srv.SetToken(agentToken)
			}

			// Register with the server so it knows we exist.
			if err := srv.Register(); err != nil {
				log.Printf("WARNING: agent registration failed (will continue): %v", err)
			}

			ctx := signalCtx()

			// Run poll loop -- blocks until ctx is cancelled.
			go func() {
				if err := srv.Run(ctx); err != nil {
					log.Printf("agent poll loop error: %v", err)
				}
			}()

			<-ctx.Done()
			log.Println("agent shutting down -- killing managed processes")
			srv.Executor().Shutdown()
			return nil
		},
	}
	return cmd
}

func signalCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()
	return ctx
}

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/api"
)

var (
	configFile string
	dataDir    string
)

func main() {
	root := &cobra.Command{
		Use:   "stroppy-cloud",
		Short: "Database testing orchestrator",
	}

	root.PersistentFlags().StringVarP(&configFile, "config", "c", "run.json", "path to run config JSON")
	root.PersistentFlags().StringVar(&dataDir, "data-dir", "", "badger data directory (empty = in-memory)")

	root.AddCommand(
		serveCmd(),
		runCmd(),
		resumeCmd(),
		validateCmd(),
		dryRunCmd(),
		agentCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	var addr string
	var victoriaURL string
	var apiKey string
	var settingsFile string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server (agent API + external API + WS)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to persistent storage in serve mode.
			if dataDir == "" {
				dataDir = "./data"
			}

			app, err := newApp()
			if err != nil {
				return err
			}
			defer app.Close()

			logger, _ := zap.NewDevelopment()
			srv := api.NewServer(app, logger, victoriaURL, apiKey, settingsFile)

			httpSrv := &http.Server{Addr: addr, Handler: srv.Router()}

			go func() {
				logger.Info("server listening", zap.String("addr", addr))
				if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Fatal("server error", zap.Error(err))
				}
			}()

			ctx := signalCtx()
			<-ctx.Done()
			logger.Info("shutting down")
			return httpSrv.Shutdown(context.Background())
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "listen address")
	cmd.Flags().StringVar(&victoriaURL, "victoria-url", "", "VictoriaMetrics URL (e.g. http://localhost:8428)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication (empty = auth disabled)")
	cmd.Flags().StringVar(&settingsFile, "settings-file", "settings.json", "path to settings JSON file for persistence")
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

			app, err := newApp()
			if err != nil {
				return err
			}
			defer app.Close()

			ctx := signalCtx()
			return app.Start(ctx, cfg)
		},
	}
}

func resumeCmd() *cobra.Command {
	var runID string
	cmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume an interrupted run",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := api.LoadConfig(configFile)
			if err != nil {
				return err
			}

			if runID == "" {
				runID = cfg.ID
			}

			app, err := newApp()
			if err != nil {
				return err
			}
			defer app.Close()

			ctx := signalCtx()
			return app.Resume(ctx, runID, cfg)
		},
	}
	cmd.Flags().StringVar(&runID, "run-id", "", "run ID to resume (defaults to config ID)")
	return cmd
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

			app, err := newApp()
			if err != nil {
				return err
			}
			defer app.Close()

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

			app, err := newApp()
			if err != nil {
				return err
			}
			defer app.Close()

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
	var port int
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run in agent mode on a target machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read configuration from env vars with flag overrides.
			serverAddr := os.Getenv("STROPPY_SERVER_ADDR")

			if envPort := os.Getenv("STROPPY_AGENT_PORT"); envPort != "" {
				if p, err := strconv.Atoi(envPort); err == nil {
					port = p
				}
			}

			machineID := os.Getenv("STROPPY_MACHINE_ID")
			if machineID == "" {
				h, err := os.Hostname()
				if err != nil {
					return fmt.Errorf("determine machine ID: %w", err)
				}
				machineID = h
			}

			srv := agent.NewAgentServer(serverAddr, machineID, port)

			// Best-effort registration with the orchestration server.
			if err := srv.Register(); err != nil {
				log.Printf("WARNING: agent registration failed (will continue): %v", err)
			}

			addr := fmt.Sprintf(":%d", port)
			httpSrv := &http.Server{Addr: addr, Handler: srv.Router()}

			go func() {
				log.Printf("agent listening on %s (machine_id=%s)", addr, machineID)
				if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("agent server error: %v", err)
				}
			}()

			ctx := signalCtx()
			<-ctx.Done()
			log.Println("agent shutting down")
			return httpSrv.Shutdown(context.Background())
		},
	}
	cmd.Flags().IntVar(&port, "port", agent.DefaultAgentPort, "agent listen port")
	return cmd
}

func newApp() (*api.App, error) {
	logger, _ := zap.NewDevelopment()
	return api.New(api.Config{
		DataDir: dataDir,
		Logger:  logger,
		Client:  nil, // buildDeps defaults to HTTPClient when nil
		Sink:    nil, // wired by NewServer when in serve mode
	})
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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

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
		Use:   "stroppy-cloud",
		Short: "Database testing orchestrator",
	}

	root.PersistentFlags().StringVarP(&configFile, "config", "c", "run.json", "path to run config JSON")
	root.PersistentFlags().StringVar(&dbPath, "db", "", "PostgreSQL DSN (e.g. postgres://stroppy:stroppy@localhost:5432/stroppy?sslmode=disable)")

	root.AddCommand(
		serveCmd(),
		runCmd(),
		validateCmd(),
		dryRunCmd(),
		agentCmd(),
		compareCmd(),
		uploadCmd(),
		waitCmd(),
	)

	if err := root.Execute(); err != nil {
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

func compareCmd() *cobra.Command {
	var serverURL, runA, runB, format string
	var threshold float64
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare metrics between two runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Build URL
			u := fmt.Sprintf("%s/api/v1/compare?a=%s&b=%s", serverURL, url.QueryEscape(runA), url.QueryEscape(runB))
			if threshold > 0 {
				u += fmt.Sprintf("&threshold=%.1f", threshold)
			}

			resp, err := http.Get(u)
			if err != nil {
				return fmt.Errorf("compare request: %w", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != 200 {
				return fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
			}

			if format == "json" {
				fmt.Println(string(body))
				return nil
			}

			// Parse and print table
			var result struct {
				RunA    string `json:"run_a"`
				RunB    string `json:"run_b"`
				Metrics []struct {
					Key        string  `json:"key"`
					Name       string  `json:"name"`
					Unit       string  `json:"unit"`
					AvgA       float64 `json:"avg_a"`
					AvgB       float64 `json:"avg_b"`
					DiffAvgPct float64 `json:"diff_avg_pct"`
					Verdict    string  `json:"verdict"`
				} `json:"metrics"`
				Summary struct {
					Better int `json:"better"`
					Worse  int `json:"worse"`
					Same   int `json:"same"`
				} `json:"summary"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}

			if format == "junit" {
				// JUnit XML output
				fmt.Println(`<?xml version="1.0" encoding="UTF-8"?>`)
				fmt.Printf("<testsuite name=\"stroppy-compare\" tests=\"%d\" failures=\"%d\">\n",
					len(result.Metrics), result.Summary.Worse)
				for _, m := range result.Metrics {
					fmt.Printf("  <testcase name=\"%s\" classname=\"stroppy.%s\">\n", m.Name, m.Key)
					if m.Verdict == "worse" {
						fmt.Printf("    <failure message=\"%s regressed by %.1f%%\">avg_a=%.2f avg_b=%.2f diff=%.1f%%</failure>\n",
							m.Name, m.DiffAvgPct, m.AvgA, m.AvgB, m.DiffAvgPct)
					}
					fmt.Println("  </testcase>")
				}
				fmt.Println("</testsuite>")
				return nil
			}

			// Table format (default)
			fmt.Printf("Compare: %s vs %s\n\n", result.RunA, result.RunB)
			fmt.Printf("%-35s %12s %12s %10s %8s\n", "METRIC", "RUN A", "RUN B", "DIFF %", "VERDICT")
			fmt.Println(strings.Repeat("-", 82))
			for _, m := range result.Metrics {
				verdict := m.Verdict
				switch verdict {
				case "better":
					verdict = "better"
				case "worse":
					verdict = "worse"
				case "same":
					verdict = "= same"
				}
				fmt.Printf("%-35s %12.2f %12.2f %+9.1f%% %8s\n",
					m.Name, m.AvgA, m.AvgB, m.DiffAvgPct, verdict)
			}
			fmt.Printf("\nSummary: %d better, %d worse, %d same\n",
				result.Summary.Better, result.Summary.Worse, result.Summary.Same)

			// Exit with non-zero if regressions exceed limit
			if result.Summary.Worse > 2 {
				return fmt.Errorf("performance regression: %d metrics worse", result.Summary.Worse)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&serverURL, "server", "http://localhost:8080", "server URL")
	cmd.Flags().StringVar(&runA, "run-a", "", "first run ID (baseline)")
	cmd.Flags().StringVar(&runB, "run-b", "", "second run ID")
	cmd.Flags().StringVar(&format, "format", "table", "output format: table, json, junit")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "custom threshold percentage (0 = server default)")
	cmd.MarkFlagRequired("run-a")
	cmd.MarkFlagRequired("run-b")
	return cmd
}

func uploadCmd() *cobra.Command {
	var serverURL, filePath string
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload a .deb or .rpm package to the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("open file: %w", err)
			}
			defer f.Close()

			ext := filepath.Ext(filePath)
			var endpoint string
			switch ext {
			case ".deb":
				endpoint = "/api/v1/upload/deb"
			case ".rpm":
				endpoint = "/api/v1/upload/rpm"
			default:
				return fmt.Errorf("unsupported file type %q (must be .deb or .rpm)", ext)
			}

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, err := writer.CreateFormFile("file", filepath.Base(filePath))
			if err != nil {
				return err
			}
			if _, err := io.Copy(part, f); err != nil {
				return err
			}
			writer.Close()

			resp, err := http.Post(serverURL+endpoint, writer.FormDataContentType(), body)
			if err != nil {
				return fmt.Errorf("upload request: %w", err)
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != 200 {
				return fmt.Errorf("upload failed %d: %s", resp.StatusCode, string(respBody))
			}

			var result struct {
				Filename string `json:"filename"`
				URL      string `json:"url"`
				Size     string `json:"size"`
			}
			json.Unmarshal(respBody, &result)
			fmt.Printf("Uploaded: %s\n  URL: %s\n  Size: %s bytes\n", result.Filename, result.URL, result.Size)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverURL, "server", "http://localhost:8080", "server URL")
	cmd.Flags().StringVar(&filePath, "file", "", "path to .deb or .rpm file")
	cmd.MarkFlagRequired("file")
	return cmd
}

func waitCmd() *cobra.Command {
	var serverURL, runID string
	var timeout, interval time.Duration
	cmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait for a run to complete",
		RunE: func(cmd *cobra.Command, args []string) error {
			deadline := time.Now().Add(timeout)
			for {
				if time.Now().After(deadline) {
					return fmt.Errorf("timeout after %s", timeout)
				}

				resp, err := http.Get(fmt.Sprintf("%s/api/v1/run/%s/status", serverURL, url.PathEscape(runID)))
				if err != nil {
					log.Printf("status check failed: %v (retrying)", err)
					time.Sleep(interval)
					continue
				}

				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				var snap struct {
					Nodes []struct {
						ID     string `json:"id"`
						Status string `json:"status"`
						Error  string `json:"error,omitempty"`
					} `json:"nodes"`
				}
				if err := json.Unmarshal(body, &snap); err != nil {
					log.Printf("parse error: %v (retrying)", err)
					time.Sleep(interval)
					continue
				}

				pending, done, failed := 0, 0, 0
				for _, n := range snap.Nodes {
					switch n.Status {
					case "done":
						done++
					case "failed":
						failed++
					default:
						pending++
					}
				}

				total := len(snap.Nodes)
				fmt.Printf("\r[%d/%d] done=%d failed=%d pending=%d", done+failed, total, done, failed, pending)

				if failed > 0 {
					fmt.Println()
					for _, n := range snap.Nodes {
						if n.Status == "failed" {
							fmt.Printf("  FAILED: %s: %s\n", n.ID, n.Error)
						}
					}
					return fmt.Errorf("run failed: %d nodes failed", failed)
				}

				if pending == 0 && total > 0 {
					fmt.Printf("\nRun %s completed successfully (%d nodes)\n", runID, total)
					return nil
				}

				time.Sleep(interval)
			}
		},
	}
	cmd.Flags().StringVar(&serverURL, "server", "http://localhost:8080", "server URL")
	cmd.Flags().StringVar(&runID, "run-id", "", "run ID to wait for")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "max wait time")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "poll interval")
	cmd.MarkFlagRequired("run-id")
	return cmd
}

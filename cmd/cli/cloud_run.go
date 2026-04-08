package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func cloudRunCmd() *cobra.Command {
	var configPath, runID, debFile, presetID string
	var wait, skipValidation bool
	var timeout, interval time.Duration
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start a run on the remote server",
		Long: `Start a run on the remote server from a JSON config file.

If --deb is provided, a temporary package is created with the .deb file
and its ID is injected into the config as package_id before launching.

If --preset-id is provided, the preset topology is used (server resolves it).

The config is validated before launch unless --skip-validation is set.

Examples:
  stroppy-cloud cloud run -c run.json
  stroppy-cloud cloud run -c run.json --wait
  stroppy-cloud cloud run -c run.json --preset-id <id>
  stroppy-cloud cloud run -c run.json --deb ./custom-pg.deb --wait`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			// Validate first (unless skipped).
			if !skipValidation {
				fmt.Print("Validating config... ")
				data, err := os.ReadFile(configPath)
				if err != nil {
					return fmt.Errorf("read config: %w", err)
				}
				var cfg map[string]any
				if err := json.Unmarshal(data, &cfg); err != nil {
					return fmt.Errorf("parse config: %w", err)
				}
				if presetID != "" {
					cfg["preset_id"] = presetID
				}
				valData, _ := json.Marshal(cfg)
				body, status, err := c.doJSON("POST", "/api/v1/validate", strings.NewReader(string(valData)))
				if err != nil {
					return fmt.Errorf("validation request: %w", err)
				}
				if status != 200 {
					return fmt.Errorf("validation failed: %s", string(body))
				}
				fmt.Println("OK")
			}

			id, err := submitRun(c, configPath, runID, debFile)
			if err != nil {
				return err
			}

			fmt.Printf("Run started: %s\n", id)

			if !wait {
				return nil
			}

			return waitForRun(c, id, timeout, interval)
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "path to run config JSON")
	cmd.Flags().StringVar(&runID, "id", "", "custom run ID (auto-generated if omitted)")
	cmd.Flags().StringVar(&presetID, "preset-id", "", "topology preset ID (server resolves topology)")
	cmd.Flags().StringVar(&debFile, "deb", "", "path to .deb file (creates package and injects package_id)")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for run to complete")
	cmd.Flags().BoolVar(&skipValidation, "skip-validation", false, "skip server-side validation")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "max wait time (with --wait)")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "poll interval (with --wait)")
	cmd.MarkFlagRequired("config")
	return cmd
}

// submitRun reads a config file, optionally creates a package from a .deb,
// and POSTs to the server. Returns the run ID.
func submitRun(c *cloudHTTPClient, configPath, runID, debPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse config: %w", err)
	}

	if runID != "" {
		cfg["id"] = runID
	}

	// If --deb provided: extract db kind/version from config, create package, inject package_id.
	if debPath != "" {
		dbKind, dbVersion := extractDBInfo(cfg)
		if dbKind == "" {
			return "", fmt.Errorf("cannot determine database.kind from config (needed for --deb)")
		}
		name := fmt.Sprintf("cli-%s-%s", filepath.Base(debPath), time.Now().Format("20060102-150405"))
		pkgID, err := createPackageWithDeb(c, name, dbKind, dbVersion, debPath)
		if err != nil {
			return "", fmt.Errorf("create package from deb: %w", err)
		}
		cfg["package_id"] = pkgID
	}

	data, _ = json.Marshal(cfg)
	body, status, err := c.doJSON("POST", "/api/v1/run", strings.NewReader(string(data)))
	if err != nil {
		return "", fmt.Errorf("start run: %w", err)
	}
	if status != 202 {
		return "", fmt.Errorf("server error %d: %s", status, string(body))
	}

	var result struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	return result.RunID, nil
}

// extractDBInfo pulls database.kind and database.version from a parsed run config.
func extractDBInfo(cfg map[string]any) (kind, version string) {
	db, ok := cfg["database"].(map[string]any)
	if !ok {
		return "", ""
	}
	kind, _ = db["kind"].(string)
	version, _ = db["version"].(string)
	return kind, version
}

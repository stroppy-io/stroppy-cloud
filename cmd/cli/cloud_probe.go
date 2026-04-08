package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func cloudProbeCmd() *cobra.Command {
	var script, driverType string
	var poolSize, scaleFactor int

	cmd := &cobra.Command{
		Use:   "probe",
		Short: "Probe a stroppy script for metadata (steps, env vars, SQL structure)",
		Long: `Runs stroppy probe on the server side and returns script metadata
including available steps, environment variables, SQL sections, and driver defaults.

Examples:
  stroppy-cloud cloud probe --script tpcc/procs
  stroppy-cloud cloud probe --script tpcc/tx --driver postgres
  stroppy-cloud cloud probe --script tpcb/procs --driver mysql --pool-size 200`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			reqBody := map[string]any{
				"script": script,
			}
			if driverType != "" {
				reqBody["driver_type"] = driverType
			}
			if poolSize > 0 {
				reqBody["pool_size"] = poolSize
			}
			if scaleFactor > 0 {
				reqBody["scale_factor"] = scaleFactor
			}

			data, _ := json.Marshal(reqBody)
			body, status, err := c.doJSON("POST", "/api/v1/probe", strings.NewReader(string(data)))
			if err != nil {
				return fmt.Errorf("probe request: %w", err)
			}
			if status != 200 {
				return fmt.Errorf("probe failed (%d): %s", status, string(body))
			}

			// Pretty-print JSON.
			var pretty json.RawMessage
			if err := json.Unmarshal(body, &pretty); err == nil {
				out, _ := json.MarshalIndent(pretty, "", "  ")
				fmt.Println(string(out))
			} else {
				fmt.Println(string(body))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&script, "script", "", "script name (e.g. tpcc/procs, tpcb/tx)")
	cmd.Flags().StringVar(&driverType, "driver", "", "driver type (postgres, mysql, picodata)")
	cmd.Flags().IntVar(&poolSize, "pool-size", 0, "pool size for probe")
	cmd.Flags().IntVar(&scaleFactor, "scale-factor", 0, "scale factor for probe")
	cmd.MarkFlagRequired("script")
	return cmd
}

func cloudPresetsCmd() *cobra.Command {
	var dbKind string

	cmd := &cobra.Command{
		Use:   "presets",
		Short: "List topology presets",
		Long: `List available topology presets for the current tenant.

Examples:
  stroppy-cloud cloud presets
  stroppy-cloud cloud presets --kind postgres`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			path := "/api/v1/presets"
			if dbKind != "" {
				path += "?db_kind=" + dbKind
			}

			body, status, err := c.doJSON("GET", path, nil)
			if err != nil {
				return fmt.Errorf("list presets: %w", err)
			}
			if status != 200 {
				return fmt.Errorf("server error %d: %s", status, string(body))
			}

			var presets []map[string]any
			if err := json.Unmarshal(body, &presets); err != nil {
				fmt.Println(string(body))
				return nil
			}

			fmt.Printf("%-36s %-20s %-10s %s\n", "ID", "NAME", "DB", "BUILTIN")
			fmt.Println(strings.Repeat("-", 80))
			for _, p := range presets {
				id, _ := p["id"].(string)
				name, _ := p["name"].(string)
				kind, _ := p["db_kind"].(string)
				builtin, _ := p["is_builtin"].(bool)
				builtinStr := ""
				if builtin {
					builtinStr = "yes"
				}
				fmt.Printf("%-36s %-20s %-10s %s\n", id, name, kind, builtinStr)
			}
			fmt.Printf("\n%d presets\n", len(presets))

			return nil
		},
	}

	cmd.Flags().StringVar(&dbKind, "kind", "", "filter by database kind (postgres, mysql, picodata)")
	return cmd
}

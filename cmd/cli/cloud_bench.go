package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

func cloudBenchCmd() *cobra.Command {
	var baselineConfig, candidateConfig string
	var baselineDeb, candidateDeb string
	var runA, runB string
	var outputFiles []string
	var threshold float64
	var timeout, interval time.Duration

	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Run two configs (or wait for two runs), then compare results",
		Long: `Launch baseline and candidate runs in parallel, wait for both to complete,
then compare their metrics. Alternatively, pass existing run IDs to skip launching.

A comparison table is always printed to the console. Use -o to save results
to files — the format is determined by extension:
  .md    → markdown
  .json  → json
  .xml   → junit XML

Examples:
  # Launch two configs and compare
  stroppy-cloud cloud bench --baseline pg16.json --candidate pg17.json

  # With custom .deb packages
  stroppy-cloud cloud bench \
    --baseline run.json --baseline-deb ./pg16-custom.deb \
    --candidate run.json --candidate-deb ./pg17-custom.deb

  # Save results to multiple formats
  stroppy-cloud cloud bench --run-a run-123 --run-b run-456 \
    -o results.md -o results.json -o results.xml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			idA := runA
			idB := runB

			// Launch runs if configs provided.
			if baselineConfig != "" || candidateConfig != "" {
				if baselineConfig == "" || candidateConfig == "" {
					return fmt.Errorf("both --baseline and --candidate are required when launching runs")
				}

				var wg sync.WaitGroup
				var errA, errB error

				wg.Add(2)
				go func() {
					defer wg.Done()
					idA, errA = submitRun(c, baselineConfig, "", baselineDeb)
					if errA == nil {
						fmt.Printf("Baseline started: %s\n", idA)
					}
				}()
				go func() {
					defer wg.Done()
					idB, errB = submitRun(c, candidateConfig, "", candidateDeb)
					if errB == nil {
						fmt.Printf("Candidate started: %s\n", idB)
					}
				}()
				wg.Wait()

				if errA != nil {
					return fmt.Errorf("baseline launch failed: %w", errA)
				}
				if errB != nil {
					return fmt.Errorf("candidate launch failed: %w", errB)
				}
			}

			if idA == "" || idB == "" {
				return fmt.Errorf("provide either --baseline/--candidate configs or --run-a/--run-b IDs")
			}

			// Wait for both runs in parallel.
			fmt.Println("\nWaiting for both runs to complete...")
			var wg sync.WaitGroup
			var errA, errB error

			wg.Add(2)
			go func() {
				defer wg.Done()
				errA = waitForRun(c, idA, timeout, interval)
			}()
			go func() {
				defer wg.Done()
				errB = waitForRun(c, idB, timeout, interval)
			}()
			wg.Wait()

			fmt.Println() // newline after progress output
			if errA != nil {
				return fmt.Errorf("baseline run failed: %w", errA)
			}
			if errB != nil {
				return fmt.Errorf("candidate run failed: %w", errB)
			}

			// Compare.
			fmt.Println("Comparing results...")
			return runCompare(c, idA, idB, threshold, outputFiles)
		},
	}

	cmd.Flags().StringVar(&baselineConfig, "baseline", "", "path to baseline run config JSON")
	cmd.Flags().StringVar(&candidateConfig, "candidate", "", "path to candidate run config JSON")
	cmd.Flags().StringVar(&baselineDeb, "baseline-deb", "", "path to .deb for baseline run")
	cmd.Flags().StringVar(&candidateDeb, "candidate-deb", "", "path to .deb for candidate run")
	cmd.Flags().StringVar(&runA, "run-a", "", "existing baseline run ID (skip launch)")
	cmd.Flags().StringVar(&runB, "run-b", "", "existing candidate run ID (skip launch)")
	cmd.Flags().StringArrayVarP(&outputFiles, "output", "o", nil, "output file (format from extension: .md .json .xml); repeatable")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "custom threshold percentage (0 = server default)")
	cmd.Flags().DurationVar(&timeout, "timeout", 60*time.Minute, "max wait time per run")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "poll interval")

	return cmd
}

type compareResult struct {
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

// runCompare fetches the comparison, prints a table to stdout,
// and writes each -o file in the format matching its extension.
func runCompare(c *cloudHTTPClient, runA, runB string, threshold float64, outputFiles []string) error {
	path := fmt.Sprintf("/api/v1/compare?a=%s&b=%s",
		url.QueryEscape(runA), url.QueryEscape(runB))
	if threshold > 0 {
		path += fmt.Sprintf("&threshold=%.1f", threshold)
	}

	body, status, err := c.doJSON("GET", path, nil)
	if err != nil {
		return fmt.Errorf("compare request: %w", err)
	}
	if status != 200 {
		return fmt.Errorf("server error %d: %s", status, string(body))
	}

	var result compareResult
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	// Always print table to console.
	var table strings.Builder
	renderTable(&table, &result)
	fmt.Print(table.String())

	// Write output files.
	for _, outPath := range outputFiles {
		var buf strings.Builder
		switch formatFromExt(outPath) {
		case "json":
			var pretty bytes.Buffer
			json.Indent(&pretty, body, "", "  ")
			buf.WriteString(pretty.String())
			buf.WriteString("\n")
		case "junit":
			renderJUnit(&buf, &result)
		case "markdown":
			renderMarkdown(&buf, &result)
		default:
			renderTable(&buf, &result)
		}
		if err := os.WriteFile(outPath, []byte(buf.String()), 0644); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		fmt.Fprintf(os.Stderr, "Saved: %s\n", outPath)
	}

	return nil
}

func formatFromExt(path string) string {
	switch filepath.Ext(path) {
	case ".md", ".markdown":
		return "markdown"
	case ".json":
		return "json"
	case ".xml":
		return "junit"
	default:
		return "table"
	}
}

func renderTable(w *strings.Builder, r *compareResult) {
	fmt.Fprintf(w, "\nCompare: %s vs %s\n\n", r.RunA, r.RunB)
	fmt.Fprintf(w, "%-35s %12s %12s %10s %8s\n", "METRIC", "BASELINE", "CANDIDATE", "DIFF %", "VERDICT")
	fmt.Fprintln(w, strings.Repeat("-", 82))
	for _, m := range r.Metrics {
		verdict := m.Verdict
		if verdict == "same" {
			verdict = "= same"
		}
		fmt.Fprintf(w, "%-35s %12.2f %12.2f %+9.1f%% %8s\n",
			m.Name, m.AvgA, m.AvgB, m.DiffAvgPct, verdict)
	}
	fmt.Fprintf(w, "\nSummary: %d better, %d worse, %d same\n",
		r.Summary.Better, r.Summary.Worse, r.Summary.Same)
}

func renderMarkdown(w *strings.Builder, r *compareResult) {
	fmt.Fprintf(w, "## Benchmark: %s vs %s\n\n", r.RunA, r.RunB)
	fmt.Fprintln(w, "| Metric | Baseline | Candidate | Diff % | Verdict |")
	fmt.Fprintln(w, "|--------|----------|-----------|--------|---------|")
	for _, m := range r.Metrics {
		verdict := m.Verdict
		switch verdict {
		case "better":
			verdict = ":white_check_mark: better"
		case "worse":
			verdict = ":x: worse"
		case "same":
			verdict = ":heavy_minus_sign: same"
		}
		fmt.Fprintf(w, "| %s | %.2f %s | %.2f %s | %+.1f%% | %s |\n",
			m.Name, m.AvgA, m.Unit, m.AvgB, m.Unit, m.DiffAvgPct, verdict)
	}
	fmt.Fprintf(w, "\n**Summary:** %d better, %d worse, %d same\n",
		r.Summary.Better, r.Summary.Worse, r.Summary.Same)
}

func renderJUnit(w *strings.Builder, r *compareResult) {
	fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintf(w, "<testsuite name=\"stroppy-compare\" tests=\"%d\" failures=\"%d\">\n",
		len(r.Metrics), r.Summary.Worse)
	for _, m := range r.Metrics {
		fmt.Fprintf(w, "  <testcase name=\"%s\" classname=\"stroppy.%s\">\n", m.Name, m.Key)
		if m.Verdict == "worse" {
			fmt.Fprintf(w, "    <failure message=\"%s regressed by %.1f%%\">avg_a=%.2f avg_b=%.2f diff=%.1f%%</failure>\n",
				m.Name, m.DiffAvgPct, m.AvgA, m.AvgB, m.DiffAvgPct)
		}
		fmt.Fprintln(w, "  </testcase>")
	}
	fmt.Fprintln(w, "</testsuite>")
}

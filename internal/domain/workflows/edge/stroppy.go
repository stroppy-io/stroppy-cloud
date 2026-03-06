package edge

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/samber/lo"
	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
	"github.com/stroppy-io/hatchet-workflow/internal/core/defaults"
	"github.com/stroppy-io/hatchet-workflow/internal/core/envs"
	hatchet_ext "github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	edgeDomain "github.com/stroppy-io/hatchet-workflow/internal/domain/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/workflows"
)

func InstallStroppy(c *hatchetLib.Client, identifier *edge.Task_Identifier) *hatchetLib.StandaloneTask {
	return c.NewStandaloneTask(
		edgeDomain.TaskIdToString(identifier),
		hatchet_ext.WTask(
			func(
				ctx hatchetLib.Context,
				input *workflows.Tasks_InstallStroppy_Input,
			) (*workflows.Tasks_InstallStroppy_Output, error) {
				err := input.Validate()
				if err != nil {
					return nil, err
				}
				url := fmt.Sprintf(
					"https://github.com/stroppy-io/stroppy/releases/download/%s/stroppy_linux_amd64.tar.gz",
					input.GetStroppyCli().GetVersion(),
				)
				downloadPath := filepath.Join("/tmp", "stroppy_linux_amd64.tar.gz")

				out, err := os.Create(downloadPath)
				if err != nil {
					return nil, fmt.Errorf("failed to create file: %w", err)
				}
				defer out.Close()

				resp, err := http.Get(url)
				if err != nil {
					return nil, fmt.Errorf("failed to download file: %w", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					return nil, fmt.Errorf("bad status: %s", resp.Status)
				}

				_, err = io.Copy(out, resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to write file: %w", err)
				}

				// Unpack to /usr/bin
				cmd := exec.Command("tar", "-xzf", downloadPath, "-C", filepath.Dir(input.GetStroppyCli().GetBinaryPath()))
				if output, err := cmd.CombinedOutput(); err != nil {
					return nil, fmt.Errorf("failed to unpack stroppy: %s: %w", string(output), err)
				}
				return &workflows.Tasks_InstallStroppy_Output{}, nil
			}),
	)
}

func streamLogsWithPrefix(ctx context.Context, r io.Reader, prefix string, log func(string), wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			line := scanner.Text()
			fmt.Println(prefix + line)
			log(prefix + line)
		}
	}
}

const grafanaBaseURLEnv consts.EnvKey = "GRAFANA_BASE_URL"

func grafanaURL(runID string) string {
	base := os.Getenv(string(grafanaBaseURLEnv))
	if base == "" {
		base = "http://localhost:3000"
	}
	return fmt.Sprintf("%s/d/stroppy-overview?orgId=1&var-node=All&var-run_id=%s", base, runID)
}

const (
	StroppyCommandGen = "gen"
	StroppyCommandRun = "run"

	StroppyWorkdirFlag = "--workdir"
	StroppyPresetFlag  = "--preset"
	TagFlag            = "--tag"
	OutputFlag         = "--out"

	K6RunIdTagName    = "run_id"
	K6WorkloadTagName = "workload"

	DriverUrlEnvVar   consts.EnvKey = "DRIVER_URL"
	ScaleFactorEnvVar consts.EnvKey = "SCALE_FACTOR"
	DurationEnvVar    consts.EnvKey = "DURATION"

	defaultScaleFactor uint32 = 1
	doubleDashFlag     string = "--"
	opentelemetryOut   string = "opentelemetry"
)

func RunStroppyTask(
	c *hatchetLib.Client,
	identifier *edge.Task_Identifier,
) *hatchetLib.StandaloneTask {
	return c.NewStandaloneTask(
		edgeDomain.TaskIdToString(identifier),
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *workflows.Tasks_RunStroppy_Input,
		) (*stroppy.TestResult, error) {
			runcmd := func(cmd *exec.Cmd) error {
				stdout, _ := cmd.StdoutPipe()
				stderr, _ := cmd.StderrPipe()
				err := cmd.Start()
				if err != nil {
					return err
				}
				var wg sync.WaitGroup
				wg.Add(2)
				go streamLogsWithPrefix(ctx, stdout, "", ctx.Log, &wg)
				go streamLogsWithPrefix(ctx, stderr, "", ctx.Log, &wg)
				wg.Wait()
				return cmd.Wait()
			}

			workloadName := strings.ToLower(input.GetStroppyCliCall().GetWorkload().String())
			envsCmd := append(
				os.Environ(),
				envs.ToSlice(
					lo.Assign(
						input.GetStroppyCliCall().GetStroppyEnv(),
						map[string]string{
							DriverUrlEnvVar: input.GetConnectionString(),
							ScaleFactorEnvVar: strconv.Itoa(int(defaults.Uint32PtrOrDefault(
								input.GetStroppyCliCall().ScaleFactor,
								defaultScaleFactor,
							))),
							DurationEnvVar: defaults.DurationOrDefault(
								input.GetStroppyCliCall().GetDuration().AsDuration(),
								time.Hour,
							).String(),
						},
					),
				)...,
			)
			genCmd := exec.Command(
				input.GetStroppyCliCall().GetBinaryPath(),
				StroppyCommandGen,
				StroppyWorkdirFlag,
				input.GetStroppyCliCall().GetWorkdir(),
				StroppyPresetFlag,
				workloadName,
			)
			genCmd.Env = envsCmd
			ctx.Log(fmt.Sprintf(
				"Running Stroppy generation with command: %s in workdir: %s",
				genCmd.String(),
				input.GetStroppyCliCall().GetWorkdir(),
			))
			err := runcmd(genCmd)
			if err != nil {
				return nil, fmt.Errorf("failed to run stroppy gen: %w", err)
			}
			runCmd := exec.Command(
				input.GetStroppyCliCall().GetBinaryPath(),
				StroppyCommandRun,
				fmt.Sprintf("%s.ts", workloadName),
				fmt.Sprintf("%s.sql", workloadName),
				// TODO: Add tags after stroppy release
				doubleDashFlag,
				TagFlag,
				fmt.Sprintf("%s=%s", K6RunIdTagName, input.GetRunSettings().GetRunId()),
				TagFlag,
				fmt.Sprintf("%s=%s", K6WorkloadTagName, workloadName),
				OutputFlag,
				opentelemetryOut,
			)
			runCmd.Env = envsCmd
			runCmd.Dir = input.GetStroppyCliCall().GetWorkdir()
			ctx.Log(fmt.Sprintf("Running Stroppy test with command: %s in dir: %s", runCmd.String(), runCmd.Dir))
			err = runcmd(runCmd)
			if err != nil {
				return nil, fmt.Errorf("failed to run stroppy: %w", err)
			}
			return &stroppy.TestResult{
				RunId: input.GetRunSettings().GetRunId(),
				Test:  input.GetRunSettings().GetTest(),
				GrafanaUrl: lo.ToPtr(grafanaURL(input.GetRunSettings().GetRunId())),
			}, nil
		}),
		hatchetLib.WithExecutionTimeout(24*time.Hour),
	)
}

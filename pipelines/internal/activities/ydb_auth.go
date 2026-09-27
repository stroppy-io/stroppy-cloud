package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/graphene-ci/pipeline/pkg/workerapi"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/cloud"
	"github.com/stroppy-io/stroppy-cloud/pipelines/stroppycfg"
)

// Resolve credentials only on the runner agent. The persistent config artifact
// stays credential-free; the container reads a separate mode-0600 runtime file
// which is removed after execution, including error and cancellation paths.
func ydbRuntimeArgs(ctx context.Context, configPath, secret string, args []string, probe stroppyContainer) (runCtx context.Context, updated []string, remove func(), err error) {
	if secret == "" {
		return ctx, args, func() {}, nil
	}
	raw, err := workerapi.GetSecret(ctx, secret)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve YDB credentials: %w", err)
	}
	probeCtx, cancelProbe := context.WithTimeout(ctx, 5*time.Minute)
	version, err := runStroppy(probeCtx, probe)
	cancelProbe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("check Stroppy authentication capabilities: %w", err)
	}
	var capabilities map[string]string
	canRefresh := version.ExitCode == 0 && json.Unmarshal(version.Log, &capabilities) == nil && capabilities["ydb_token_file"] == "1"
	issue := func(ctx context.Context) (cloud.IAMCredential, error) {
		return (cloud.Yandex{}).IAMTokenWithExpiration(ctx, raw)
	}
	first, err := issue(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create YDB IAM token: %w", err)
	}
	if canRefresh {
		return renewingRuntimeConfig(ctx, configPath, first, issue, args)
	}
	// Preserve old images for bounded short segments, but never imply renewal
	// support by silently passing a field that an old binary might ignore.
	if err := validateLegacyIAMDeadline(ctx, first.ExpiresAt); err != nil {
		return nil, nil, nil, err
	}
	updated, remove, err = runtimeTokenConfig(configPath, first.Token, args)
	return ctx, updated, remove, err
}

func validateLegacyIAMDeadline(ctx context.Context, expiry time.Time) error {
	deadline, ok := ctx.Deadline()
	if !ok || !deadline.Before(expiry.Add(-time.Minute)) {
		return fmt.Errorf("selected Stroppy image does not support renewable YDB token-file authentication; use an image advertising ydb_token_file=1 for this segment deadline")
	}
	return nil
}

func runtimeTokenConfig(configPath, token string, args []string) (updated []string, remove func(), err error) {
	if token == "" {
		return nil, nil, fmt.Errorf("empty YDB IAM token")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, nil, err
	}
	drivers, ok := cfg["drivers"].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("runtime YDB config has no driver")
	}
	driver, ok := drivers["0"].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("runtime YDB driver is not an object")
	}
	driver["authToken"] = token
	raw, err = json.Marshal(cfg)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.CreateTemp(filepath.Dir(configPath), ".stroppy-iam-*.json")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.Remove(file.Name()) }
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		cleanup()
		return nil, nil, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return nil, nil, err
	}
	result := append([]string(nil), args...)
	for i := 0; i+1 < len(result); i++ {
		if result[i] == "-f" {
			result[i+1] = path.Join(stroppycfg.ContainerWorkspace, filepath.Base(file.Name()))
			return result, cleanup, nil
		}
	}
	cleanup()
	return nil, nil, fmt.Errorf("stroppy config argument missing")
}

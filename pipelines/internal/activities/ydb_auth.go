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
func ydbRuntimeArgs(ctx context.Context, configPath, secret string, args []string, probe stroppyContainer) (updated []string, remove func(), err error) {
	if secret == "" {
		return args, func() {}, nil
	}
	raw, err := workerapi.GetSecret(ctx, secret)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve YDB credentials: %w", err)
	}
	probeCtx, cancelProbe := context.WithTimeout(ctx, 5*time.Minute)
	version, err := runStroppy(probeCtx, probe)
	cancelProbe()
	if err != nil {
		return nil, nil, fmt.Errorf("check Stroppy authentication capabilities: %w", err)
	}
	var capabilities map[string]string
	canUseKey := version.ExitCode == 0 && json.Unmarshal(version.Log, &capabilities) == nil && capabilities["ydb_service_account_key_file"] == "1"
	if canUseKey {
		key, err := cloud.YandexServiceAccountKeyJSON(raw)
		if err != nil {
			return nil, nil, err
		}
		return runtimeServiceAccountConfig(configPath, key, args)
	}
	first, err := (cloud.Yandex{}).IAMTokenWithExpiration(ctx, raw)
	if err != nil {
		return nil, nil, fmt.Errorf("create YDB IAM token: %w", err)
	}
	// Preserve old images for bounded short segments, but never imply renewal
	// support by silently passing a field that an old binary might ignore.
	if err := validateLegacyIAMDeadline(ctx, first.ExpiresAt); err != nil {
		return nil, nil, err
	}
	return runtimeTokenConfig(configPath, first.Token, args)
}

func validateLegacyIAMDeadline(ctx context.Context, expiry time.Time) error {
	deadline, ok := ctx.Deadline()
	if !ok || !deadline.Before(expiry.Add(-time.Minute)) {
		return fmt.Errorf("selected Stroppy image does not support YDB service-account key authentication; use an image advertising ydb_service_account_key_file=1 for this segment deadline")
	}
	return nil
}

func runtimeTokenConfig(configPath, token string, args []string) (updated []string, remove func(), err error) {
	if token == "" {
		return nil, nil, fmt.Errorf("empty YDB IAM token")
	}
	return runtimeCredentialConfig(configPath, "authToken", token, args)
}

func runtimeServiceAccountConfig(configPath, key string, args []string) (updated []string, remove func(), err error) {
	file, err := os.CreateTemp(filepath.Dir(configPath), ".stroppy-sa-key-*.json")
	if err != nil {
		return nil, nil, err
	}
	removeKey := func() { _ = os.Remove(file.Name()) }
	if _, err := file.WriteString(key); err != nil {
		_ = file.Close()
		removeKey()
		return nil, nil, err
	}
	if err := file.Close(); err != nil {
		removeKey()
		return nil, nil, err
	}
	args, removeConfig, err := runtimeCredentialConfig(configPath, "serviceAccountKeyFile", path.Join(stroppycfg.ContainerWorkspace, filepath.Base(file.Name())), args)
	if err != nil {
		removeKey()
		return nil, nil, err
	}
	return args, func() { removeConfig(); removeKey() }, nil
}

func runtimeCredentialConfig(configPath, field, value string, args []string) (updated []string, remove func(), err error) {
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
	for _, key := range []string{"authToken", "authTokenFile", "serviceAccountKeyFile", "authUser", "authPassword"} {
		delete(driver, key)
	}
	driver[field] = value
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

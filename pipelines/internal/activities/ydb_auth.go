package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/graphene-ci/pipeline/pkg/workerapi"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/cloud"
	"github.com/stroppy-io/stroppy-cloud/pipelines/stroppycfg"
)

// Resolve credentials only on the runner agent. The persistent config artifact
// stays credential-free; the container reads a separate mode-0600 runtime file
// which is removed after execution, including error and cancellation paths.
func ydbRuntimeArgs(ctx context.Context, configPath, secret string, args []string) (updated []string, remove func(), err error) {
	if secret == "" {
		return args, func() {}, nil
	}
	raw, err := workerapi.GetSecret(ctx, secret)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve YDB credentials: %w", err)
	}
	token, err := (cloud.Yandex{}).IAMToken(ctx, raw)
	if err != nil {
		return nil, nil, fmt.Errorf("create YDB IAM token: %w", err)
	}
	return runtimeTokenConfig(configPath, token, args)
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

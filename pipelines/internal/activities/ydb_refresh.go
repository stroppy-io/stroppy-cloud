package activities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/graphene-ci/pipeline/pkg/obs"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/cloud"
	"github.com/stroppy-io/stroppy-cloud/pipelines/stroppycfg"
)

var errIAMExpired = errors.New("YDB IAM token expired before refresh succeeded")

// Atomic rename inside the mounted workspace makes a complete new credential
// visible in the running container without persisting it as a config artifact.
func replaceTokenFile(name, token string) error {
	f, err := os.CreateTemp(filepath.Dir(name), ".stroppy-token-next-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.WriteString(token); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), name)
}

func renewingRuntimeConfig(ctx context.Context, configPath string, first cloud.IAMCredential,
	issue func(context.Context) (cloud.IAMCredential, error), args []string,
) (runContext context.Context, arguments []string, release func(), err error) {
	if first.Token == "" || !first.ExpiresAt.After(time.Now()) {
		return nil, nil, nil, fmt.Errorf("initial IAM token is missing or expired")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, nil, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, nil, nil, err
	}
	drivers, ok := cfg["drivers"].(map[string]any)
	if !ok {
		return nil, nil, nil, fmt.Errorf("runtime YDB config has no driver")
	}
	driver, ok := drivers["0"].(map[string]any)
	if !ok {
		return nil, nil, nil, fmt.Errorf("runtime YDB driver is not an object")
	}
	tokenFile, err := os.CreateTemp(filepath.Dir(configPath), ".stroppy-token-*")
	if err != nil {
		return nil, nil, nil, err
	}
	tokenName := tokenFile.Name()
	if err = tokenFile.Close(); err != nil {
		_ = os.Remove(tokenName)
		return nil, nil, nil, err
	}
	cleanupFiles := func() { _ = os.Remove(tokenName) }
	if err = replaceTokenFile(tokenName, first.Token); err != nil {
		cleanupFiles()
		return nil, nil, nil, err
	}
	delete(driver, "authToken")
	driver["authTokenFile"] = path.Join(stroppycfg.ContainerWorkspace, filepath.Base(tokenName))
	raw, err = json.Marshal(cfg)
	if err != nil {
		cleanupFiles()
		return nil, nil, nil, err
	}
	runtimeFile, err := os.CreateTemp(filepath.Dir(configPath), ".stroppy-iam-*")
	if err != nil {
		cleanupFiles()
		return nil, nil, nil, err
	}
	configName := runtimeFile.Name()
	cleanupFiles = func() { _ = os.Remove(tokenName); _ = os.Remove(configName) }
	if _, err = runtimeFile.Write(raw); err != nil {
		_ = runtimeFile.Close()
		cleanupFiles()
		return nil, nil, nil, err
	}
	if err = runtimeFile.Close(); err != nil {
		cleanupFiles()
		return nil, nil, nil, err
	}
	updated := append([]string(nil), args...)
	found := false
	for i := 0; i+1 < len(updated); i++ {
		if updated[i] == "-f" {
			updated[i+1] = path.Join(stroppycfg.ContainerWorkspace, filepath.Base(configName))
			found = true
			break
		}
	}
	if !found {
		cleanupFiles()
		return nil, nil, nil, fmt.Errorf("stroppy config argument missing")
	}
	runCtx, cancel := context.WithCancelCause(ctx)
	done := make(chan struct{})
	go func() { defer close(done); refreshIAM(runCtx, cancel, first, issue, tokenName) }()
	cleanup := func() { cancel(context.Canceled); <-done; cleanupFiles() }
	return runCtx, updated, cleanup, nil
}

func refreshIAM(ctx context.Context, cancel context.CancelCauseFunc, current cloud.IAMCredential,
	issue func(context.Context) (cloud.IAMCredential, error), filename string,
) {
	delay := min(10*time.Minute, time.Until(current.ExpiresAt)/2)
	for {
		timer := time.NewTimer(max(time.Millisecond, delay))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if !current.ExpiresAt.After(time.Now()) {
			cancel(errIAMExpired)
			return
		}
		issueCtx, stop := context.WithDeadline(ctx, minTime(current.ExpiresAt, time.Now().Add(30*time.Second)))
		next, err := issue(issueCtx)
		stop()
		if err == nil && (next.Token == "" || !next.ExpiresAt.After(time.Now())) {
			err = fmt.Errorf("invalid renewed IAM credential")
		}
		if err == nil {
			err = replaceTokenFile(filename, next.Token)
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// Do not log issuer errors: upstream text may contain credentials.
			obs.Warn(ctx, "YDB IAM token refresh failed; retaining the unexpired token")
			remaining := time.Until(current.ExpiresAt)
			if remaining <= 0 {
				cancel(errIAMExpired)
				return
			}
			delay = min(30*time.Second, remaining/4)
			continue
		}
		state := "unchanged"
		if next.Token != current.Token {
			state = "replaced"
		}
		previousExpiry := current.ExpiresAt
		current = next
		obs.Info(ctx, "YDB IAM token refreshed",
			obs.Str("credential_state", state),
			obs.Str("previous_expires_at", previousExpiry.UTC().Format(time.RFC3339)),
			obs.Str("expires_at", current.ExpiresAt.UTC().Format(time.RFC3339)))
		delay = min(10*time.Minute, time.Until(current.ExpiresAt)/2)
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

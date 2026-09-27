package activities

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/cloud"
)

func TestIAMRefreshRetainsValidTokenAndCleansRuntimeSecrets(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "stroppy-config.json")
	original := []byte(`{"drivers":{"0":{"driverType":"ydb"}}}`)
	require.NoError(t, os.WriteFile(config, original, 0600))
	var calls atomic.Int32
	issue := func(context.Context) (cloud.IAMCredential, error) {
		if calls.Add(1) == 1 {
			return cloud.IAMCredential{}, errors.New("transient issuer failure with sensitive payload")
		}
		return cloud.IAMCredential{Token: "rotated-secret", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}
	runCtx, args, cleanup, err := renewingRuntimeConfig(t.Context(), config, cloud.IAMCredential{Token: "initial-secret", ExpiresAt: time.Now().Add(time.Second)}, issue, []string{"run", "-f", "/workspace/stroppy-config.json"})
	require.NoError(t, err)
	t.Cleanup(cleanup)
	runtimeName := filepath.Join(dir, filepath.Base(args[2]))
	b, err := os.ReadFile(runtimeName)
	require.NoError(t, err)
	require.NotContains(t, string(b), "initial-secret")
	var cfg struct {
		Drivers map[string]struct{ AuthTokenFile string }
	}
	require.NoError(t, json.Unmarshal(b, &cfg))
	tokenName := filepath.Join(dir, filepath.Base(cfg.Drivers["0"].AuthTokenFile))
	for _, name := range []string{runtimeName, tokenName} {
		info, err := os.Stat(name)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
	require.Eventually(t, func() bool { b, err := os.ReadFile(tokenName); return err == nil && string(b) == "rotated-secret" }, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, runCtx.Err())
	b, err = os.ReadFile(config)
	require.NoError(t, err)
	require.Equal(t, original, b)
	cleanup()
	files, err := filepath.Glob(filepath.Join(dir, ".stroppy-*"))
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestIAMRefreshExpiryCancelsWorkloadWithoutReplacingToken(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "stroppy-config.json")
	require.NoError(t, os.WriteFile(config, []byte(`{"drivers":{"0":{}}}`), 0600))
	runCtx, _, cleanup, err := renewingRuntimeConfig(t.Context(), config, cloud.IAMCredential{Token: "initial-secret", ExpiresAt: time.Now().Add(100 * time.Millisecond)}, func(context.Context) (cloud.IAMCredential, error) {
		return cloud.IAMCredential{}, errors.New("issuer unavailable")
	}, []string{"run", "-f", "/workspace/stroppy-config.json"})
	require.NoError(t, err)
	defer cleanup()
	select {
	case <-runCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("expired token left workload running")
	}
	require.ErrorIs(t, context.Cause(runCtx), errIAMExpired)
	require.NoError(t, t.Context().Err())
}

func TestLegacyIAMRequiresDeadlineWithinCredentialLifetime(t *testing.T) {
	expires := time.Now().Add(time.Hour)
	require.Error(t, validateLegacyIAMDeadline(t.Context(), expires))
	short, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	require.NoError(t, validateLegacyIAMDeadline(short, expires))
	long, cancelLong := context.WithTimeout(t.Context(), 2*time.Hour)
	defer cancelLong()
	require.Error(t, validateLegacyIAMDeadline(long, expires))
	require.Error(t, validateLegacyIAMDeadline(short, time.Now().Add(-time.Minute)))
}

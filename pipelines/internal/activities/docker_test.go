package activities

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
)

func TestReadPullProgress(t *testing.T) {
	for _, tc := range []struct {
		name, stream, want string
	}{
		{"success", "{\"status\":\"Downloading\"}\n{\"status\":\"Pull complete\"}\n", ""},
		{"daemon error after progress", "{\"status\":\"Downloading\"}\n{\"errorDetail\":{\"message\":\"failed to extract layer\"},\"error\":\"failed to extract layer\"}\n", "failed to extract layer"},
		{"legacy error", `{"error":"access denied"}`, "access denied"},
		{"truncated response", `{"status":`, "decode docker progress"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := readPullProgress(strings.NewReader(tc.stream))
			if tc.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.want)
			}
		})
	}
}

func TestPullImageCacheAndRegistryRetry(t *testing.T) {
	for _, tc := range []struct {
		name, image, firstError string
		cached                  bool
		wantPulls               int32
		wantError               bool
	}{
		{"cached digest", "test.invalid/image@sha256:" + strings.Repeat("a", 64), "", true, 0, false},
		{"tag is refreshed", "test.invalid/image:latest", "", true, 1, false},
		{"transient registry", "test.invalid/image:latest", "dial tcp: i/o timeout", false, 2, false},
		{"permanent rejection", "test.invalid/image:latest", "unauthorized: authentication required", false, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var pulls atomic.Int32
			engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/_ping"):
					w.Header().Set("API-Version", "1.47")
					_, _ = fmt.Fprint(w, "OK")
				case strings.HasSuffix(r.URL.Path, "/json"):
					w.Header().Set("Content-Type", "application/json")
					if tc.cached {
						_, _ = fmt.Fprint(w, `{"Id":"sha256:cached"}`)
					} else {
						w.WriteHeader(http.StatusNotFound)
						_, _ = fmt.Fprint(w, `{"message":"No such image"}`)
					}
				case strings.HasSuffix(r.URL.Path, "/images/create"):
					w.Header().Set("Content-Type", "application/json")
					if pulls.Add(1) == 1 && tc.firstError != "" {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = fmt.Fprintf(w, `{"message":%q}`, tc.firstError)
					} else {
						_, _ = fmt.Fprintln(w, `{"status":"Pull complete"}`)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer engine.Close()
			t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(engine.URL, "http://"))
			t.Setenv("DOCKER_API_VERSION", "1.47")
			t.Setenv("DOCKER_TLS_VERIFY", "")
			t.Setenv("DOCKER_CERT_PATH", "")
			_, err := PullImage(t.Context(), PullImageRequest{Image: tc.image})
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantPulls, pulls.Load())
		})
	}
}

func TestPullImageHeartbeatsWhileDockerIsSilent(t *testing.T) {
	// With progress-only heartbeats Docker never receives permission to reply.
	// Cover both the initial HTTP wait and the silent progress-stream wait.
	var beats atomic.Int32
	initialWait, streamWait := make(chan struct{}), make(chan struct{})
	var initialOnce, streamOnce sync.Once
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/_ping") {
			w.Header().Set("API-Version", "1.47")
			_, _ = fmt.Fprint(w, "OK")
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/images/create") {
			http.NotFound(w, r)
			return
		}
		select {
		case <-initialWait:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, `{"status":"Downloading"}`)
		w.(http.Flusher).Flush()
		select {
		case <-streamWait:
			_, _ = fmt.Fprintln(w, `{"status":"Pull complete"}`)
		case <-r.Context().Done():
		}
	}))
	defer engine.Close()
	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(engine.URL, "http://"))
	t.Setenv("DOCKER_API_VERSION", "1.47")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	ctx, cancel := context.WithTimeout(t.Context(), 75*time.Second)
	defer cancel()
	env.SetWorkerOptions(worker.Options{BackgroundActivityContext: ctx})
	env.RegisterActivity(PullImage)
	// The test environment batches heartbeat delivery every 30 seconds;
	// its worker throttle options are ignored. Two silent waits take ~60s.
	env.SetOnActivityHeartbeatListener(func(_ *activity.Info, _ converter.EncodedValues) {
		n := beats.Add(1)
		if n >= 2 {
			initialOnce.Do(func() { close(initialWait) })
		}
		if n >= 3 {
			streamOnce.Do(func() { close(streamWait) })
		}
	})
	result, err := env.ExecuteActivity(PullImage, PullImageRequest{Image: "test.invalid/silent:1"})
	require.NoError(t, err)
	var pulled PullImageResult
	require.NoError(t, result.Get(&pulled))
	require.Equal(t, "test.invalid/silent:1", pulled.Image)
	require.GreaterOrEqual(t, beats.Load(), int32(3))
}

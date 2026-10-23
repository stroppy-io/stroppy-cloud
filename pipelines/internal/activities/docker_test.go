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
	"go.temporal.io/sdk/temporal"
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
	backoff := pullBackoff
	t.Cleanup(func() { pullBackoff = backoff })
	pullBackoff = time.Millisecond
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
				var app *temporal.ApplicationError
				require.ErrorAs(t, err, &app)
				require.True(t, app.NonRetryable(), "a rejected pull must fail the run, not retry forever")
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
	// Watch the beats directly: Temporal's test environment delivers them
	// in 30-second batches, which would make this a minute-long test.
	every, on, beat := heartbeatEvery, heartbeatsOn, heartbeat
	t.Cleanup(func() { heartbeatEvery, heartbeatsOn, heartbeat = every, on, beat })
	heartbeatEvery = 10 * time.Millisecond
	heartbeatsOn = func(context.Context) bool { return true }
	heartbeat = func(context.Context, ...any) {
		n := beats.Add(1)
		if n >= 2 {
			initialOnce.Do(func() { close(initialWait) })
		}
		if n >= 3 {
			streamOnce.Do(func() { close(streamWait) })
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	pulled, err := PullImage(ctx, PullImageRequest{Image: "test.invalid/silent:1"})
	require.NoError(t, err)
	require.Equal(t, "test.invalid/silent:1", pulled.Image)
	require.GreaterOrEqual(t, beats.Load(), int32(3))
}

package activities

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

func TestStreamAndWaitCutsOversizedLines(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		cut          bool // the line is longer than the log keeps
	}{
		{"complete", "=== bench summary ===\n  iterations_total  42\n", false},
		// A 2 MB line (a driver dumping a statement) is cut, not fatal.
		{"oversized-line", "=== bench summary ===\n  iterations_total  42\n" + strings.Repeat("x", 2<<20), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/logs") {
					w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
					_, _ = io.WriteString(stdcopy.NewStdWriter(w, stdcopy.Stdout), tc.output)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/wait") {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"StatusCode":0}`)
					return
				}
				http.NotFound(w, r)
			}))
			defer server.Close()
			cli, err := dockerclient.NewClientWithOpts(dockerclient.WithHost(server.URL), dockerclient.WithVersion("1.47"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cli.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			path := filepath.Join(t.TempDir(), "stroppy.log")
			code, log, stdout, err := streamAndWait(ctx, cli, "test", path, "measure")
			if tc.cut {
				want := "=== bench summary ===\n  iterations_total  42\n" + strings.Repeat("x", 1<<20) + "… [1048576 bytes cut]\n"
				raw, rerr := os.ReadFile(path)
				if err != nil || code != 0 || string(log) != want || string(raw) != want || string(stdout) != tc.output {
					t.Fatalf("code=%d log=%d bytes file=%d bytes error=%v/%v", code, len(log), len(raw), err, rerr)
				}
				return
			}
			if err != nil || code != 0 || string(log) != tc.output || string(stdout) != tc.output {
				t.Fatalf("code=%d log=%q stdout=%q error=%v", code, log, stdout, err)
			}
			raw, err := os.ReadFile(path)
			if err != nil || string(raw) != tc.output {
				t.Fatalf("file=%q error=%v", raw, err)
			}
		})
	}
}

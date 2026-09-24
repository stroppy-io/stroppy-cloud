package pipelines

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	workerplanev1 "github.com/graphene-ci/pipeline/pkg/proto/workerplane/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/graphene"
)

func TestReadManifestWithoutToolchain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell")
	}
	// The executable must only export its manifest. No CLI command, Graphene
	// connection or Go compiler is involved in reading a packaged binary.
	t.Setenv("PATH", "")
	for _, tc := range []struct {
		name, body, wantError string
	}{
		{"valid", `printf '%s' '{"pipelineId":"stroppy-run","activities":[]}'`, ""},
		{"wrong pipeline", `printf '%s' '{"pipelineId":"other"}'`, "declares pipeline"},
		{"invalid JSON", `printf '%s' 'bad json'`, "manifest JSON"},
		{"failed binary", `printf '%s' 'cannot export' >&2; exit 7`, "cannot export"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := filepath.Join(t.TempDir(), "pipeline")
			script := "#!/bin/sh\n[ \"$#\" = 0 ] && [ \"$GRAPHENE_MANIFEST\" = 1 ] || exit 8\n" + tc.body + "\n"
			if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			raw, err := readManifest(t.Context(), bin, "stroppy-run")
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("wanted %q, got %v", tc.wantError, err)
				}
				return
			}
			if err != nil || string(raw) != `{"pipelineId":"stroppy-run","activities":[]}` {
				t.Fatalf("manifest = %q, error = %v", raw, err)
			}
		})
	}
}

type manifestRecorder struct {
	workerplanev1.UnimplementedManifestAPIServer
	request *workerplanev1.PublishManifestRequest
	headers metadata.MD
	err     error
}

func (r *manifestRecorder) PublishManifest(ctx context.Context, req *workerplanev1.PublishManifestRequest) (*workerplanev1.PublishManifestResponse, error) {
	r.request = req
	r.headers, _ = metadata.FromIncomingContext(ctx)
	return &workerplanev1.PublishManifestResponse{}, r.err
}

func TestPublishManifestScopesTenantAndPropagatesFailure(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "denied"}[denied], func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			recorder := &manifestRecorder{}
			if denied {
				recorder.err = status.Error(codes.PermissionDenied, "tenant not allowed")
			}
			server := grpc.NewServer()
			workerplanev1.RegisterManifestAPIServer(server, recorder)
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(server.Stop)
			cfg := &graphene.Config{Address: listener.Addr().String(), Token: "test-token", Insecure: true}
			raw := []byte(`{"pipelineId":"stroppy-run","activities":[]}`)
			err = publishManifest(t.Context(), cfg, "t-test", "registry/t-test/stroppy-run:digest", raw)
			if denied {
				if status.Code(err) != codes.PermissionDenied {
					t.Fatalf("wanted PermissionDenied, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			// Stop joins the RPC handler before reading its captured request.
			server.GracefulStop()
			if recorder.headers.Get("authorization")[0] != "Bearer test-token" || recorder.headers.Get(graphene.NamespaceHeader)[0] != "t-test" {
				t.Fatal("missing tenant or authentication metadata")
			}
			if string(recorder.request.GetManifest()) != string(raw) || recorder.request.GetImage() != "registry/t-test/stroppy-run:digest" {
				t.Fatalf("published different manifest/image: %v", recorder.request)
			}
		})
	}
	if !(publishAuth{}).RequireTransportSecurity() {
		t.Fatal("TLS must be required by default")
	}
}

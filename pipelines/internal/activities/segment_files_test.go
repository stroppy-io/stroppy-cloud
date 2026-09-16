package activities

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestSegmentFilesPreservePathsEmptyContentAndArtifacts(t *testing.T) {
	dir := t.TempDir()
	files := []spec.SegmentFile{{Name: "empty.sql"}, {Name: "sql/schema.sql", Content: "CREATE TABLE t(id int)"}, {Name: "sql/query.sql", Ref: "artifact/query"}}
	calls := 0
	fetch := func(_ context.Context, location string) (io.ReadCloser, error) {
		calls++
		if location != "blob-location" {
			t.Fatalf("location=%s", location)
		}
		return io.NopCloser(strings.NewReader("SELECT 1")), nil
	}
	if err := writeSegmentFiles(context.Background(), dir, files, map[string]string{"sql/query.sql": "blob-location"}, fetch); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"empty.sql": "", "sql/schema.sql": "CREATE TABLE t(id int)", "sql/query.sql": "SELECT 1"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(raw) != want {
			t.Fatalf("%s: %q %v", name, raw, err)
		}
	}
	if calls != 1 {
		t.Fatalf("fetch calls=%d", calls)
	}
	if err := writeSegmentFiles(context.Background(), dir, files, nil, fetch); err == nil {
		t.Fatal("unresolved artifact silently accepted")
	}
}

func TestSegmentFilesRejectReservedAndEscapingPaths(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "stroppy-config.json", "ca.pem", "stroppy.log", "sql/../escape"} {
		if err := writeSegmentFiles(context.Background(), t.TempDir(), []spec.SegmentFile{{Name: name, Content: "x"}}, nil, nil); err == nil {
			t.Errorf("accepted %s", name)
		}
	}
}

package activities

import (
	"os"
	"path/filepath"
	"testing"
)

// The agent publishes workspace files the stroppy container wrote: a link it
// planted to a file outside the workspace is not one of them.
func TestRegularFileRefusesSymlinks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(dir, "tpcc-delivery.jsonl")
	if err := os.WriteFile(plain, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.jsonl")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{plain: true, link: false, dir: false, filepath.Join(dir, "absent"): false} {
		if got := regularFile(path); got != want {
			t.Errorf("regularFile(%s) = %t, want %t", path, got, want)
		}
	}
}

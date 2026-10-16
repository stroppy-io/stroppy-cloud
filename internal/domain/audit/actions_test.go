package audit

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Every action and target kind the code records is in Actions/TargetKinds.
func TestCodeRecordsOnlyListedAudit(t *testing.T) {
	action := regexp.MustCompile(`Action:\s*"([a-z_.]+)"|\.record\(ctx, "([a-z_.]+)"|\baction\s*:?=\s*"([a-z_.]+)"`)
	kind := regexp.MustCompile(`Target\{Kind:\s*"([a-z_]+)"`)
	seen := 0
	err := filepath.WalkDir(filepath.Join("..", ".."), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range action.FindAllSubmatch(src, -1) {
			a := string(m[1]) + string(m[2]) + string(m[3])
			seen++
			if !slices.Contains(Actions, a) {
				t.Errorf("%s records action %q, not in audit.Actions", p, a)
			}
		}
		for _, m := range kind.FindAllSubmatch(src, -1) {
			if !slices.Contains(TargetKinds, string(m[1])) {
				t.Errorf("%s records target kind %q, not in audit.TargetKinds", p, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen < 40 {
		t.Fatalf("found only %d actions; the scan is broken", seen)
	}
}

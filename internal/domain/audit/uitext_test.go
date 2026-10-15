package audit

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/uitext"
)

// Every audit action and target kind the server records has its text in the
// SPA (`settings.audit.actions.<action with _>`, `settings.audit.targetKinds`).
func TestAuditHasUIText(t *testing.T) {
	action := regexp.MustCompile(`Action:\s*"([a-z_.]+)"`)
	kind := regexp.MustCompile(`Target\{Kind:\s*"([a-z_]+)"`)
	keys := map[string]string{}
	err := filepath.WalkDir(filepath.Join("..", ".."), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range action.FindAllSubmatch(src, -1) {
			keys["settings.audit.actions."+strings.ReplaceAll(string(m[1]), ".", "_")] = p
		}
		for _, m := range kind.FindAllSubmatch(src, -1) {
			keys["settings.audit.targetKinds."+string(m[1])] = p
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) < 40 {
		t.Fatalf("found only %d audit keys; the scan is broken", len(keys))
	}
	for key, from := range keys {
		for _, lang := range uitext.Langs {
			if ok, err := uitext.Has(lang, key); err != nil || !ok {
				t.Errorf("%s: %s (%s) has no text", lang, key, from)
			}
		}
	}
}

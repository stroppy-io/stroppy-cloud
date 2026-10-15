package topology

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/uitext"
)

// Every topology role (a `Role… = "…"` constant here) has its name in the
// SPA (`common.topologyRole.<role>`).
func TestRolesHaveUIText(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	role := regexp.MustCompile(`\bRole[A-Z][A-Za-z]*\s*=\s*"([a-z0-9-]+)"`)
	roles := map[string]bool{}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range role.FindAllSubmatch(src, -1) {
			roles[string(m[1])] = true
		}
	}
	if len(roles) < 5 {
		t.Fatalf("found only %d roles; the scan is broken", len(roles))
	}
	for r := range roles {
		for _, lang := range uitext.Langs {
			if ok, err := uitext.Has(lang, "common.topologyRole."+r); err != nil || !ok {
				t.Errorf("%s: role %q has no text in common.topologyRole (%v)", lang, r, err)
			}
		}
	}
}

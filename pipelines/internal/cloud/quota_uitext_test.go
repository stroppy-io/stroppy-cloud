package cloud

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every quota the providers report has its name and unit in the SPA
// (`common.quota.<id>`, `common.quotaUnit.<unit>`, dots and dashes as
// underscores — web/src/helpers/quotas.ts): an id without one reaches the
// user as `compute.disks.count`.
func TestQuotasHaveUIText(t *testing.T) {
	key := strings.NewReplacer(".", "_", "-", "_").Replace
	want := map[string]string{}
	for _, q := range yandexQuotaIDs {
		want["quota."+key(q.id)] = q.id
		want["quotaUnit."+key(q.unit)] = q.unit
	}
	for _, q := range awsQuotaCodes {
		want["quota."+key(q.name)] = q.name
		want["quotaUnit."+key(q.unit)] = q.unit
	}
	for _, lang := range []string{"ru", "en"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "src", "locales", lang, "common.json"))
		if err != nil {
			t.Fatal(err)
		}
		var common map[string]any
		if err := json.Unmarshal(raw, &common); err != nil {
			t.Fatal(err)
		}
		for path, from := range want {
			group, name, _ := strings.Cut(path, ".")
			m, _ := common[group].(map[string]any)
			if s, _ := m[name].(string); s == "" {
				t.Errorf("%s: %q has no common.%s", lang, from, path)
			}
		}
	}
}

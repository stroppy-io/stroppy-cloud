package catalog

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/uitext"
)

// Every metric has its title in the SPA (`catalog.metrics.<key>.title`),
// and every group that is not a database's own (those read as the
// product's name) has one in `runs.metrics.groups.<group>`.
func TestMetricsHaveUIText(t *testing.T) {
	for _, m := range metrics() {
		keys := []string{"catalog.metrics." + m.Key + ".title"}
		if m.Description != "" {
			keys = append(keys, "catalog.metrics."+m.Key+".description")
		}
		if m.Scope != "db" {
			keys = append(keys, "runs.metrics.groups."+strings.ToLower(m.Group))
		}
		for _, key := range keys {
			for _, lang := range uitext.Langs {
				if ok, err := uitext.Has(lang, key); err != nil || !ok {
					t.Errorf("%s: metric %q has no %s", lang, m.Key, key)
				}
			}
		}
	}
}

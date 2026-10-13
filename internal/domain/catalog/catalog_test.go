package catalog_test

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
)

// TestCatalogBakes is the whole point of the static tables: every size row,
// the stroppy catalog and every topology template must fit their schemas,
// and every referenced schema id must exist.
func TestCatalogBakes(t *testing.T) {
	reg := schemas.New()
	c, err := catalog.New(context.Background(), reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Databases) != 12 {
		t.Fatalf("databases = %d", len(c.Databases))
	}
	for _, d := range c.Databases {
		defaults := 0
		for _, v := range d.Versions {
			if v.Default {
				defaults++
			}
		}
		if defaults != 1 {
			t.Errorf("%s: %d default versions", d.Kind, defaults)
		}
		if len(d.Topologies) == 0 || len(d.Roles) == 0 {
			t.Errorf("%s: no topologies or roles", d.Kind)
		}
		hasRunner := false
		for _, r := range d.Roles {
			if r.Role == "runner" {
				hasRunner = true
			}
		}
		if !hasRunner {
			t.Errorf("%s: no runner role", d.Kind)
		}
	}
	if _, ok := c.StroppyVersion(""); !ok {
		t.Error("no default stroppy version")
	}
	if len(c.SchemaList("cfg")) == 0 || len(c.SchemaList("nope")) != 0 {
		t.Error("schema listing by namespace")
	}
	if len(c.MetricsFor(catalog.Postgres)) >= len(c.Metrics) || len(c.MetricsFor("")) != len(c.Metrics) {
		t.Error("metric filtering")
	}
}

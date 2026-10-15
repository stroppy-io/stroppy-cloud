//go:build integration

package application

import (
	"net/http"
	"strings"
	"testing"
)

func TestE2ECatalog(t *testing.T) {
	e := e2eServer(t)
	owner := e.person(slug("cat")+"@example.com", "Cat")
	tn := e.tenant(owner, "Catalog")
	tok := e.token(owner, tn)

	t.Run("databases", func(t *testing.T) {
		var got struct {
			Data []struct {
				Kind         string `json:"kind"`
				ParamsSchema string `json:"params_schema"`
				Topologies   []struct {
					ID     string         `json:"id"`
					Params map[string]any `json:"params"`
				} `json:"topologies"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/catalog/databases", nil, tok), http.StatusOK, &got)
		if len(got.Data) != 12 || got.Data[0].Kind != "postgres" || got.Data[0].ParamsSchema != "db.postgres.params@1" {
			t.Fatalf("got %+v", got.Data[:1])
		}
		// A template's params validate against the kind's schema.
		var v struct {
			Result struct {
				Errors []any `json:"errors"`
			} `json:"result"`
			Resolved map[string]any `json:"resolved"`
		}
		e.want(e.req(http.MethodPost, "/api/v1/catalog/schemas/"+got.Data[0].ParamsSchema+":validate", map[string]any{"value": got.Data[0].Topologies[2].Params}, tok), http.StatusOK, &v)
		if len(v.Result.Errors) != 0 || v.Resolved["ha"] != "patroni" {
			t.Fatalf("validate %+v", v)
		}
	})

	t.Run("providers carry the size table", func(t *testing.T) {
		var got struct {
			Data []struct {
				Kind  string `json:"kind"`
				Sizes map[string][]struct {
					Size string `json:"size"`
					CPU  int    `json:"cpu"`
				} `json:"sizes"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/catalog/providers", nil, tok), http.StatusOK, &got)
		if len(got.Data) != 2 || len(got.Data[0].Sizes["db"]) != 5 || got.Data[0].Sizes["db"][0].Size != "XS" {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("stroppy", func(t *testing.T) {
		var got struct {
			Versions []struct {
				Version string `json:"version"`
				Scripts []struct {
					ID string `json:"id"`
				} `json:"scripts"`
			} `json:"versions"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/catalog/stroppy?version=6.0.0", nil, tok), http.StatusOK, &got)
		if len(got.Versions) != 1 || len(got.Versions[0].Scripts) < 5 {
			t.Fatalf("got %+v", got)
		}
		e.problem(e.req(http.MethodGet, "/api/v1/catalog/stroppy?version=0.0.1", nil, tok), http.StatusNotFound, "not_found")
	})

	t.Run("schemas: list, get, validate, render", func(t *testing.T) {
		var list struct {
			Data []struct {
				ID        string   `json:"id"`
				Templates []string `json:"templates"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/catalog/schemas?namespace=cfg", nil, tok), http.StatusOK, &list)
		if len(list.Data) == 0 {
			t.Fatal("no cfg schemas")
		}
		var schema map[string]any
		e.want(e.req(http.MethodGet, "/api/v1/catalog/schemas/cfg.pgbouncer.ini@1", nil, tok), http.StatusOK, &schema)
		if _, ok := schema["fields"]; !ok {
			t.Fatalf("schema %v", schema)
		}
		e.problem(e.req(http.MethodGet, "/api/v1/catalog/schemas/nope@1", nil, tok), http.StatusNotFound, "not_found")

		var v struct {
			Result struct {
				Errors []struct {
					Path string `json:"path"`
				} `json:"errors"`
			} `json:"result"`
		}
		e.want(e.req(http.MethodPost, "/api/v1/catalog/schemas/db.postgres.params@1:validate", map[string]any{"value": map[string]any{"version": "99"}}, tok), http.StatusOK, &v)
		if len(v.Result.Errors) == 0 || v.Result.Errors[0].Path != "version" {
			t.Fatalf("validate %+v", v)
		}

		r := e.req(http.MethodPost, "/api/v1/catalog/schemas/cfg.pgbouncer.ini@1:render", map[string]any{"value": map[string]any{}}, tok)
		if r.Status != http.StatusOK || !strings.Contains(string(r.Body), "[pgbouncer]") {
			t.Fatalf("render: %d %s", r.Status, r.Body)
		}
	})

	t.Run("metrics per kind", func(t *testing.T) {
		var all, pg struct {
			Data []struct {
				Key string `json:"key"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/catalog/metrics", nil, tok), http.StatusOK, &all)
		e.want(e.req(http.MethodGet, "/api/v1/catalog/metrics?db_kind=postgres", nil, tok), http.StatusOK, &pg)
		if len(pg.Data) == 0 || len(pg.Data) >= len(all.Data) {
			t.Fatalf("all=%d pg=%d", len(all.Data), len(pg.Data))
		}
	})

	t.Run("catalog needs a caller", func(t *testing.T) {
		e.problem(e.req(http.MethodGet, "/api/v1/catalog/databases", nil, ""), http.StatusUnauthorized, "unauthenticated")
	})
}

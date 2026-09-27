//go:build integration

package application

import (
	"encoding/json"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

// Record only operation IDs and statuses, never URLs, tokens or response bodies.
// Reachability is reported separately from semantic acceptance assertions.
type acceptanceRoute struct {
	Method, ID string
	Pattern    *regexp.Regexp
	Parameters int
}

var acceptanceRoutes = sync.OnceValues(func() ([]acceptanceRoute, error) {
	raw, err := os.ReadFile("../../../openapi/openapi.yaml")
	if err != nil {
		return nil, err
	}
	var doc struct {
		Paths map[string]map[string]struct {
			ID string `yaml:"operationId"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	wildcard := regexp.MustCompile(`\{[^}]+\}`)
	var out []acceptanceRoute
	for path, methods := range doc.Paths {
		// Quote each literal fragment; parameter values cannot consume action suffixes.
		pattern := "^"
		loc := 0
		for _, idx := range wildcard.FindAllStringIndex(path, -1) {
			pattern += regexp.QuoteMeta(path[loc:idx[0]]) + `[^/:]+`
			loc = idx[1]
		}
		pattern += regexp.QuoteMeta(path[loc:]) + "$"
		for method, op := range methods {
			if op.ID != "" {
				out = append(out, acceptanceRoute{strings.ToUpper(method), op.ID, regexp.MustCompile(pattern), len(wildcard.FindAllStringIndex(path, -1))})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Parameters != out[j].Parameters {
			return out[i].Parameters < out[j].Parameters
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
})

func (e *e2e) recordAcceptanceHTTP(method, path string, status int) {
	if os.Getenv("STROPPY_ACCEPTANCE_REPORT") != "1" {
		return
	}
	routes, err := acceptanceRoutes()
	if err != nil {
		e.t.Fatal(err)
	}
	u, err := url.Parse(path)
	if err != nil {
		e.t.Fatal(err)
	}
	for _, r := range routes {
		if r.Method == method && r.Pattern.MatchString(u.Path) {
			key := r.ID
			e.acceptanceMu.Lock()
			if e.acceptanceHTTP == nil {
				e.acceptanceHTTP = map[string]map[int]int{}
			}
			if e.acceptanceHTTP[key] == nil {
				e.acceptanceHTTP[key] = map[int]int{}
			}
			e.acceptanceHTTP[key][status]++
			e.acceptanceMu.Unlock()
			return
		}
	}
}

func (e *e2e) reportAcceptanceHTTP() {
	e.acceptanceMu.Lock()
	defer e.acceptanceMu.Unlock()
	if len(e.acceptanceHTTP) == 0 {
		return
	}
	raw, err := json.Marshal(e.acceptanceHTTP)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Logf("ACCEPTANCE_HTTP %s", raw)
}

func TestAcceptanceRouteInventory(t *testing.T) {
	routes, err := acceptanceRoutes()
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) < 100 {
		t.Fatalf("incomplete OpenAPI inventory: %d", len(routes))
	}
	for _, r := range routes {
		if r.Method == "GET" && r.Pattern.MatchString("/api/v1/tenants/suggest-name") {
			if r.ID != "suggestTenantName" {
				t.Fatalf("literal route shadowed by %s", r.ID)
			}
			break
		}
	}
	for _, path := range []string{"/api/v1/t/example/tests:diff", "/api/v1/t/example/tests/id:launch", "/api/v1/t/example/runs/id/logs:facets"} {
		matched := 0
		for _, r := range routes {
			if r.Pattern.MatchString(path) {
				matched++
			}
		}
		if matched != 1 {
			t.Fatalf("ambiguous route %s: %d", path, matched)
		}
	}
}

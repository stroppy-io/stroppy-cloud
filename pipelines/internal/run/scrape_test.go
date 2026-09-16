package run

import (
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"testing"
)

func TestMetricsPortDoesNotFollowSQLPort(t *testing.T) {
	for _, tc := range []struct {
		name      string
		sql, http int
		path      string
	}{
		{"picodata", 4327, 8081, "/metrics"},
		{"cockroach", 26257, 8080, "/_status/vars"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := spec.Container{Scrape: tc.path, ScrapePort: tc.http, Ports: []spec.Port{{Container: tc.sql}, {Container: tc.http}}}
			got := scrapeURL(c, spec.Run{})
			c.Ports[0], c.Ports[1] = c.Ports[1], c.Ports[0]
			if got != scrapeURL(c, spec.Run{}) {
				t.Fatal("metrics endpoint depends on port order")
			}
			c.ScrapePort = 0
			if got != scrapeURL(c, spec.Run{}) {
				t.Fatal("legacy first-port contract changed")
			}
		})
	}
}

//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeDocker func(*http.Request) (*http.Response, error)

func (f fakeDocker) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTargetRefusesForeignOrAmbiguousContainers(t *testing.T) {
	t.Setenv("TARGET_CONTAINER", "db-1-mysql")
	t.Setenv("PROBE_CONTAINER", "db-1-ha-probe")
	original := docker
	t.Cleanup(func() { docker = original })
	for _, tc := range []struct {
		name, dbRun, probeRun string
		targets               int
		want                  bool
	}{
		{"owned", "run-a", "run-a", 1, true},
		{"foreign", "run-b", "run-a", 1, false},
		{"unlabelled", "", "", 1, false},
		{"ambiguous", "run-a", "run-a", 2, false},
		{"missing", "run-a", "run-a", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			docker = &http.Client{Transport: fakeDocker(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" {
					t.Fatalf("scope check mutated Docker")
				}
				var filters map[string][]string
				if err := json.Unmarshal([]byte(r.URL.Query().Get("filters")), &filters); err != nil {
					t.Fatal(err)
				}
				count, run := tc.targets, tc.dbRun
				if strings.Contains(filters["label"][0], "ha-probe") {
					count, run = 1, tc.probeRun
				}
				found := []container{}
				for i := 0; i < count; i++ {
					found = append(found, container{ID: "owned-id", State: "running", Labels: map[string]string{"stroppy-run": run}})
				}
				b, _ := json.Marshal(found)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
			})}
			id, err := target(context.Background())
			if (err == nil) != tc.want {
				t.Fatalf("id=%s error=%v", id, err)
			}
		})
	}
}

//go:build integration

package application

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
)

func TestE2EConcurrentLaunch(t *testing.T) {
	for _, sameKey := range []bool{true, false} {
		name := "no cap"
		if sameKey {
			name = "same idempotency key"
		}
		t.Run(name, func(t *testing.T) {
			e := e2eServer(t)
			// Room for every run: the quota is the only gate.
			e.graphene.quotas = []provider.Quota{{Name: "compute.instanceCores.count", Limit: 4096, Used: 4, Unit: "cores"}}
			base, tok, testID, _ := runFixtureWith(t, e, nil, "0s")
			e.graphene.hold("segment.started")
			const n = 6
			responses := make(chan resp, n)
			start := make(chan struct{})
			var wg sync.WaitGroup
			for range n {
				wg.Go(func() {
					<-start
					req, err := http.NewRequestWithContext(e.ctx, http.MethodPost, e.ts.URL+base+"/tests/"+testID+":launch", strings.NewReader(`{}`))
					if err != nil {
						t.Error(err)
						return
					}
					req.Header.Set("Authorization", "Bearer "+tok)
					req.Header.Set("Content-Type", "application/json")
					if sameKey {
						req.Header.Set("Idempotency-Key", "concurrent-"+testID)
					}
					response, err := http.DefaultClient.Do(req)
					if err != nil {
						t.Error(err)
						return
					}
					defer response.Body.Close()
					body, err := io.ReadAll(response.Body)
					if err != nil {
						t.Error(err)
						return
					}
					e.recordAcceptanceHTTP(http.MethodPost, base+"/tests/"+testID+":launch", response.StatusCode)
					responses <- resp{Status: response.StatusCode, Body: body}
				})
			}
			close(start)
			wg.Wait()
			close(responses)
			ids := map[string]bool{}
			created := 0
			for r := range responses {
				if r.Status == http.StatusCreated {
					var view runView
					if err := json.Unmarshal(r.Body, &view); err != nil {
						t.Fatal(err)
					}
					ids[view.ID] = true
					created++
				} else {
					t.Errorf("unexpected admission response %d %s", r.Status, r.Body)
				}
			}
			if sameKey && (created != n || len(ids) != 1) {
				t.Errorf("replay created=%d distinct=%d", created, len(ids))
			}
			if !sameKey {
				// Every launch is accepted and starts: nothing but quotas holds a run.
				started := 0
				for id := range ids {
					if _, ok := e.graphene.runOf(id); ok {
						started++
					}
				}
				if created != n || started != n {
					t.Errorf("no cap: created=%d started=%d", created, started)
				}
			}
			var listed struct{ Data []runView }
			e.want(e.req(http.MethodGet, base+"/runs", nil, tok), http.StatusOK, &listed)
			if len(listed.Data) != len(ids) {
				t.Error("unexpected stored runs")
			}
			// A queued run starts (held, like the others) only once a
			// started one finished: release whatever has started until all
			// are done.
			eventually(t, 20*time.Second, func() bool {
				e.app.services.Runs.AdmitQueued(e.ctx)
				done := 0
				for id := range ids {
					e.graphene.release(id)
					e.app.services.Projector.Tick(e.ctx)
					var r runView
					e.want(e.req(http.MethodGet, base+"/runs/"+id, nil, tok), http.StatusOK, &r)
					if r.Status == "completed" || r.Status == "failed" || r.Status == "cancelled" {
						done++
					}
				}
				return done == len(ids)
			})
		})
	}
}

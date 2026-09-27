//go:build integration

package application

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestE2EConcurrentLaunch(t *testing.T) {
	for _, sameKey := range []bool{true, false} {
		name := "tenant limit"
		if sameKey {
			name = "same idempotency key"
		}
		t.Run(name, func(t *testing.T) {
			e := e2eServer(t)
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
			created, limited := 0, 0
			for r := range responses {
				if r.Status == http.StatusCreated {
					var view runView
					if err := json.Unmarshal(r.Body, &view); err != nil {
						t.Fatal(err)
					}
					ids[view.ID] = true
					created++
				} else if !sameKey && r.Status == http.StatusUnprocessableEntity && strings.Contains(string(r.Body), "limit_exceeded") {
					limited++
				} else {
					t.Errorf("unexpected admission response %d %s", r.Status, r.Body)
				}
			}
			if sameKey && (created != n || len(ids) != 1) {
				t.Errorf("replay created=%d distinct=%d", created, len(ids))
			}
			if !sameKey && (created != 3 || limited != n-3) {
				t.Errorf("limit 3: created=%d rejected=%d", created, limited)
			}
			var listed struct{ Data []runView }
			e.want(e.req(http.MethodGet, base+"/runs", nil, tok), http.StatusOK, &listed)
			if len(listed.Data) != len(ids) {
				t.Error("unexpected stored runs")
			}
			for id := range ids {
				e.graphene.release(id)
				finishRun(t, e, base, tok, id)
			}
		})
	}
}

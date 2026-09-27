//go:build integration

package application

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/graphene"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/repositories"
)

func TestE2EObserverRecovery(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixtureWith(t, e, nil, "0s")
	e.graphene.hold("segment.started")
	var launched runView
	e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{}, tok), http.StatusCreated, &launched)
	first, cancel := context.WithCancel(e.ctx)
	defer cancel()
	e.app.services.Projector.Tick(first)
	eventually(t, 10*time.Second, func() bool {
		var view runView
		e.want(e.req(http.MethodGet, base+"/runs/"+launched.ID, nil, tok), http.StatusOK, &view)
		return view.Phase == "workload"
	})
	repo := repositories.NewRunRepo(testDB.TxDB, postgres.NewTransactor(testDB.Tx))
	before, err := repo.ByID(e.ctx, uuid.MustParse(launched.ID))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	eventually(t, 5*time.Second, func() bool { return !e.app.services.Projector.Following(before.ID) })
	// A fresh observer has no in-memory cursors. It must use persisted state.
	observer := run.NewProjector(repo, e.app.infra.Graphene, e.app.services.Webhooks, graphene.WithNamespace, testLog)
	if observer.Tick(e.ctx) < 1 {
		t.Fatal("fresh observer did not recover the run")
	}
	if observer.Tick(e.ctx) != 0 {
		t.Fatal("duplicate follower for an active run")
	}
	e.graphene.release(launched.ID)
	eventually(t, 10*time.Second, func() bool {
		var view runView
		e.want(e.req(http.MethodGet, base+"/runs/"+launched.ID, nil, tok), http.StatusOK, &view)
		return view.Status == "completed" && len(view.Result.Segments) == 1
	})
	after, err := repo.ByID(e.ctx, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.LastEventID <= before.LastEventID {
		t.Fatal("cursor did not advance")
	}
	var events struct {
		Data []struct {
			ID   string
			Kind string
		}
	}
	e.want(e.req(http.MethodGet, base+"/runs/"+launched.ID+"/events?limit=200", nil, tok), http.StatusOK, &events)
	counts := map[string]int{}
	for _, event := range events.Data {
		counts[event.Kind]++
	}
	if counts["run-started"] != 1 || counts["run-completed"] != 1 || counts["segment.finished"] != 1 {
		t.Fatalf("replayed/lost terminal milestones: %v", counts)
	}
	var runs struct{ Data []runView }
	e.want(e.req(http.MethodGet, base+"/runs", nil, tok), http.StatusOK, &runs)
	if len(runs.Data) != 1 || runs.Data[0].ID != launched.ID {
		t.Fatal("observation recovery relaunched the workload")
	}
}

func TestE2EKeptStandRecovery(t *testing.T) {
	for _, reason := range []string{"expired", "release acknowledgement lost"} {
		t.Run(reason, func(t *testing.T) {
			e := e2eServer(t)
			base, tok, testID, _ := runFixtureWith(t, e, nil, "1h")
			var launched runView
			e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", nil, tok), http.StatusCreated, &launched)
			finishRun(t, e, base, tok, launched.ID)
			repo := repositories.NewRunRepo(testDB.TxDB, postgres.NewTransactor(testDB.Tx))
			before, err := repo.ByID(e.ctx, uuid.MustParse(launched.ID))
			if err != nil || !before.StandKept {
				t.Fatalf("stand not kept: %v %+v", err, before)
			}
			if reason == "expired" {
				e.graphene.advance(2 * time.Hour)
			} else {
				// Graphene has completed deletion, but the HTTP caller disconnected
				// before the server could commit SetKeep. Leave that stale row behind.
				sctx := graphene.WithNamespace(e.ctx, before.GrapheneNamespace)
				held, err := e.app.infra.Graphene.Holdings(sctx, before.GrapheneRef())
				if err != nil {
					t.Fatal(err)
				}
				for _, h := range held {
					if !strings.HasPrefix(h.Ref, "artifact/") {
						if err := e.app.infra.Graphene.KeepRelease(sctx, h.Ref); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			observer := run.NewProjector(repo, e.app.infra.Graphene, e.app.services.Webhooks, graphene.WithNamespace, testLog)
			eventually(t, 10*time.Second, func() bool {
				observer.Tick(e.ctx)
				var view runView
				e.want(e.req(http.MethodGet, base+"/runs/"+launched.ID, nil, tok), http.StatusOK, &view)
				return !view.StandKept
			})
			var artifacts struct{ Data []struct{ ID string } }
			e.want(e.req(http.MethodGet, base+"/runs/"+launched.ID+"/artifacts", nil, tok), http.StatusOK, &artifacts)
			if len(artifacts.Data) == 0 {
				t.Fatal("keep reconciliation lost artifacts")
			}
		})
	}
}

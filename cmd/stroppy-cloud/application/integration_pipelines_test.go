//go:build integration

package application

import (
	"net/http"
	"testing"
	"time"
)

func TestE2EPipelinePush(t *testing.T) {
	e := e2eServer(t)
	root := e.person("root@example.com", "Root")
	tn := e.tenant(root, slug("push"))
	rtok := e.token(root, tn)

	type nsState struct {
		Namespace string `json:"namespace"`
		Status    string `json:"status"`
		Revision  string `json:"revision"`
		Error     string `json:"error"`
	}
	var st struct {
		Pipelines struct {
			ExpectedRevision string    `json:"expected_revision"`
			Namespaces       []nsState `json:"namespaces"`
		} `json:"pipelines"`
	}
	find := func() nsState {
		e.want(e.req(http.MethodGet, "/api/v1/admin/status", nil, rtok), http.StatusOK, &st)
		for _, ns := range st.Pipelines.Namespaces {
			if ns.Namespace == tn.GrapheneNamespace {
				return ns
			}
		}
		return nsState{}
	}

	t.Run("a new tenant gets every pipeline pushed", func(t *testing.T) {
		eventually(t, 10*time.Second, func() bool { return find().Status == "synced" })
		ns := find()
		if ns.Revision != st.Pipelines.ExpectedRevision || e.pushes.count(tn.GrapheneNamespace) != 5 {
			t.Fatalf("state %+v pushes %d", ns, e.pushes.count(tn.GrapheneNamespace))
		}
		// Nothing to do on the periodic check.
		if n := e.app.services.Pipelines.SyncAll(e.ctx, false); n != 0 {
			t.Fatalf("resynced %d namespaces", n)
		}
	})

	t.Run("resync forces a push and a failure is visible", func(t *testing.T) {
		e.pushes.fail = true
		e.want(e.req(http.MethodPost, "/api/v1/admin/pipelines:resync", map[string]any{"tenant_slug": tn.Slug}, rtok), http.StatusAccepted, nil)
		eventually(t, 10*time.Second, func() bool { return find().Status == "failed" })
		if ns := find(); ns.Error == "" {
			t.Fatalf("failed without error %+v", ns)
		}
		e.pushes.fail = false
		// The periodic check retries failed namespaces.
		if n := e.app.services.Pipelines.SyncAll(e.ctx, false); n != 1 {
			t.Fatalf("retried %d namespaces", n)
		}
		if ns := find(); ns.Status != "synced" {
			t.Fatalf("after retry %+v", ns)
		}
	})
}

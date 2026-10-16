//go:build integration

package application

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/webhook"
)

func ctxOf(a auth.Actor) context.Context { return auth.WithActor(context.Background(), a) }

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("uuid %q: %v", s, err)
	}
	return id
}

func TestE2ESettings(t *testing.T) {
	e := e2eServer(t)
	owner := e.person(slug("owner")+"@example.com", "Owner")
	tn := e.tenant(owner, "Set")
	tok := e.token(owner, tn)
	base := "/api/v1/t/" + tn.Slug

	var st struct {
		RunRetentionDays int      `json:"run_retention_days"`
		DefaultKeep      string   `json:"default_keep"`
		Emails           []string `json:"notification_emails"`
		Rating           struct {
			Global bool `json:"global"`
		} `json:"default_rating"`
	}
	e.want(e.req(http.MethodGet, base+"/settings", nil, tok), http.StatusOK, &st)
	if st.RunRetentionDays != 90 {
		t.Fatalf("defaults %+v", st)
	}
	e.want(e.req(http.MethodPatch, base+"/settings", map[string]any{"run_retention_days": 30, "default_keep": "2h", "default_rating": map[string]any{"global": true}, "notification_emails": []string{"Ops@x.io"}}, tok), http.StatusOK, &st)
	if st.RunRetentionDays != 30 || st.DefaultKeep != "2h0m0s" || !st.Rating.Global || len(st.Emails) != 1 || st.Emails[0] != "ops@x.io" {
		t.Fatalf("patched %+v", st)
	}
	// No ceiling on keep or retention; only nonsense is refused.
	e.want(e.req(http.MethodPatch, base+"/settings", map[string]any{"default_keep": "100h", "run_retention_days": 3650}, tok), http.StatusOK, nil)
	e.problem(e.req(http.MethodPatch, base+"/settings", map[string]any{"run_retention_days": 0}, tok), http.StatusUnprocessableEntity, "invalid")
}

func TestE2EProviderProfiles(t *testing.T) {
	e := e2eServer(t)
	owner := e.person(slug("owner")+"@example.com", "Owner")
	tn := e.tenant(owner, "Prov")
	tok := e.token(owner, tn)
	base := "/api/v1/t/" + tn.Slug + "/providers"
	yandex := map[string]any{
		"name": "yc", "kind": "yandex",
		"settings":    map[string]any{"cloud_id": "b1gcloud000000000000", "folder_id": "b1gfolder00000000000", "zone": "ru-central1-a", "network": map[string]any{"kind": "create"}},
		"credentials": map[string]any{"sa_key_json": `{"id":"ajekey00000000000000","service_account_id":"ajesa000000000000000","created_at":"2026-01-01T00:00:00Z","key_algorithm":"RSA_2048","public_key":"-----BEGIN PUBLIC KEY-----\nMIIB\n-----END PUBLIC KEY-----\n","private_key":"-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----\n"}`},
	}

	t.Run("schema validation is a Problem with details", func(t *testing.T) {
		bad := map[string]any{"name": "bad", "kind": "yandex", "settings": map[string]any{"zone": "mars"}, "credentials": map[string]any{"sa_key_json": "x"}}
		var p struct {
			Code       string `json:"code"`
			Validation struct {
				Errors []struct {
					Path string `json:"path"`
					Code string `json:"code"`
				} `json:"errors"`
			} `json:"validation"`
		}
		e.want(e.req(http.MethodPost, base, bad, tok), http.StatusUnprocessableEntity, &p)
		if p.Code != "validation_failed" || len(p.Validation.Errors) == 0 {
			t.Fatalf("got %+v", p)
		}
	})

	var created struct {
		ID          string   `json:"id"`
		Status      string   `json:"status"`
		SecretNames []string `json:"secret_names"`
	}
	e.want(e.req(http.MethodPost, base, yandex, tok), http.StatusCreated, &created)
	if created.Status != "verifying" || len(created.SecretNames) != 1 {
		t.Fatalf("created %+v", created)
	}
	if _, ok := e.graphene.secret(tn.GrapheneNamespace, created.SecretNames[0]); !ok {
		t.Fatalf("secret %s not stored in namespace %s", created.SecretNames[0], tn.GrapheneNamespace)
	}

	t.Run("verification lands ready with quotas", func(t *testing.T) {
		var got struct {
			Status           string  `json:"status"`
			QuotasObservedAt *string `json:"quotas_observed_at"`
		}
		eventually(t, 10*time.Second, func() bool {
			e.want(e.req(http.MethodGet, base+"/"+created.ID, nil, tok), http.StatusOK, &got)
			return got.Status == "ready" && got.QuotasObservedAt != nil
		})
		var q struct {
			Stale  bool `json:"stale"`
			Quotas []struct {
				Name string `json:"name"`
			} `json:"quotas"`
		}
		e.want(e.req(http.MethodGet, base+"/"+created.ID+"/quotas", nil, tok), http.StatusOK, &q)
		if q.Stale || len(q.Quotas) != 1 || q.Quotas[0].Name != "compute.instanceCores.count" {
			t.Fatalf("quotas %+v", q)
		}
	})

	t.Run("failed verification carries the reason", func(t *testing.T) {
		e.graphene.verifyOK = false
		var got struct {
			Status       string `json:"status"`
			StatusReason string `json:"status_reason"`
		}
		e.want(e.req(http.MethodPost, base+"/"+created.ID+":verify", nil, tok), http.StatusAccepted, &got)
		if got.Status != "verifying" {
			t.Fatalf("got %+v", got)
		}
		eventually(t, 10*time.Second, func() bool {
			e.want(e.req(http.MethodGet, base+"/"+created.ID, nil, tok), http.StatusOK, &got)
			return got.Status == "failed"
		})
		if got.StatusReason != "missing rights" {
			t.Fatalf("reason %q", got.StatusReason)
		}
		e.problem(e.req(http.MethodPost, base+"/"+created.ID+"/quotas:refresh", nil, tok), http.StatusConflict, "conflict")
		e.graphene.verifyOK = true
	})

	t.Run("duplicate name is 409, delete removes the secret", func(t *testing.T) {
		e.problem(e.req(http.MethodPost, base, yandex, tok), http.StatusConflict, "conflict")
		e.want(e.req(http.MethodDelete, base+"/"+created.ID, nil, tok), http.StatusNoContent, nil)
		if _, ok := e.graphene.secret(tn.GrapheneNamespace, provider.CredentialsSecret(mustUUID(t, created.ID))); ok {
			t.Fatal("secret still stored")
		}
		e.problem(e.req(http.MethodGet, base+"/"+created.ID, nil, tok), http.StatusNotFound, "not_found")
	})
}

func TestE2EWebhooks(t *testing.T) {
	e := e2eServer(t)
	owner := e.person(slug("owner")+"@example.com", "Owner")
	tn := e.tenant(owner, "Hook")
	tok := e.token(owner, tn)
	base := "/api/v1/t/" + tn.Slug + "/webhooks"

	// receiver records signed deliveries.
	got := make(chan received, 4)
	receiver := newReceiver(t, got)

	var created struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
	}
	e.want(e.req(http.MethodPost, base, map[string]any{"url": receiver.URL + "/hook", "events": []string{"run.finished"}, "description": "ci"}, tok), http.StatusCreated, &created)
	if created.Secret == "" {
		t.Fatal("no secret")
	}
	e.problem(e.req(http.MethodPost, base, map[string]any{"url": "ftp://x", "events": []string{"run.finished"}}, tok), http.StatusUnprocessableEntity, "invalid")

	t.Run("publish, dispatch, verify signature", func(t *testing.T) {
		if err := e.app.services.Webhooks.Publish(context.Background(), tn.ID, webhook.RunFinished, map[string]any{"run_id": "r1", "status": "completed"}); err != nil {
			t.Fatal(err)
		}
		// An unsubscribed event creates no delivery.
		if err := e.app.services.Webhooks.Publish(context.Background(), tn.ID, webhook.SuiteStarted, map[string]any{}); err != nil {
			t.Fatal(err)
		}
		if n := e.app.services.Dispatcher.Tick(context.Background()); n != 1 {
			t.Fatalf("dispatched %d, want 1", n)
		}
		select {
		case r := <-got:
			if !webhook.Verify(created.Secret, r.sig, r.id, r.ts, r.body) {
				t.Fatalf("bad signature %q", r.sig)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no delivery received")
		}
		var log struct {
			Data []struct {
				Status         string `json:"status"`
				Event          string `json:"event"`
				ResponseStatus *int   `json:"response_status"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/"+created.ID+"/deliveries", nil, tok), http.StatusOK, &log)
		if len(log.Data) != 1 || log.Data[0].Status != "success" || log.Data[0].ResponseStatus == nil || *log.Data[0].ResponseStatus != 200 {
			t.Fatalf("log %+v", log)
		}
	})

	t.Run("rotation keeps the old secret verifying", func(t *testing.T) {
		var rotated struct {
			Secret string `json:"secret"`
		}
		e.want(e.req(http.MethodPost, base+"/"+created.ID+":rotate-secret", nil, tok), http.StatusOK, &rotated)
		if rotated.Secret == created.Secret {
			t.Fatal("secret unchanged")
		}
		_ = e.app.services.Webhooks.Publish(context.Background(), tn.ID, webhook.RunFinished, map[string]any{"run_id": "r2"})
		e.app.services.Dispatcher.Tick(context.Background())
		r := <-got
		if !webhook.Verify(rotated.Secret, r.sig, r.id, r.ts, r.body) || !webhook.Verify(created.Secret, r.sig, r.id, r.ts, r.body) {
			t.Fatalf("signature does not verify with both secrets: %q", r.sig)
		}
	})

	t.Run("failure is retried and replayable", func(t *testing.T) {
		receiver.fail = true
		_ = e.app.services.Webhooks.Publish(context.Background(), tn.ID, webhook.RunFinished, map[string]any{"run_id": "r3"})
		e.app.services.Dispatcher.Tick(context.Background())
		<-got
		var log struct {
			Data []struct {
				ID       string `json:"id"`
				Status   string `json:"status"`
				Attempts int    `json:"attempts"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/"+created.ID+"/deliveries?limit=1", nil, tok), http.StatusOK, &log)
		if log.Data[0].Status != "pending" || log.Data[0].Attempts != 1 {
			t.Fatalf("log %+v", log)
		}
		receiver.fail = false
		var replayed struct {
			Status string `json:"status"`
		}
		e.want(e.req(http.MethodPost, base+"/"+created.ID+"/deliveries/"+log.Data[0].ID+":replay", nil, tok), http.StatusAccepted, &replayed)
		e.app.services.Dispatcher.Tick(context.Background())
		<-got
		e.want(e.req(http.MethodGet, base+"/"+created.ID+"/deliveries?limit=1", nil, tok), http.StatusOK, &log)
		if log.Data[0].Status != "success" {
			t.Fatalf("after replay %+v", log)
		}
	})

	t.Run("disable and delete", func(t *testing.T) {
		var w struct {
			Enabled bool `json:"enabled"`
		}
		e.want(e.req(http.MethodPatch, base+"/"+created.ID, map[string]any{"enabled": false}, tok), http.StatusOK, &w)
		if w.Enabled {
			t.Fatal("still enabled")
		}
		e.want(e.req(http.MethodDelete, base+"/"+created.ID, nil, tok), http.StatusNoContent, nil)
		e.problem(e.req(http.MethodGet, base+"/"+created.ID+"/deliveries", nil, tok), http.StatusNotFound, "not_found")
	})
}

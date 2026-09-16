//go:build integration

package application

import (
	"net/http"
	"testing"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
)

// Real Postgres and HTTP, fake Graphene: preserve quota visibility in the
// cache/API and prove that only permission denial relaxes the cloud precheck.
func TestE2EQuotaVisibility(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixture(t, e)
	var profiles struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	e.want(e.req(http.MethodGet, base+"/providers", nil, tok), http.StatusOK, &profiles)
	if len(profiles.Data) != 1 {
		t.Fatalf("profiles: %+v", profiles)
	}
	path := base + "/providers/" + profiles.Data[0].ID + "/quotas"
	launch := base + "/tests/" + testID + ":launch"
	var report struct {
		UnavailableReason string           `json:"unavailable_reason"`
		Scope             string           `json:"scope"`
		Quotas            []provider.Quota `json:"quotas"`
		Stale             bool             `json:"stale"`
	}
	e.graphene.mu.Lock()
	e.graphene.quotas = []provider.Quota{{Name: "compute.instanceCores.count", Limit: 1, Used: 1, Unit: "cores"}}
	e.graphene.mu.Unlock()
	e.want(e.req(http.MethodPost, path+":refresh", nil, tok), http.StatusOK, &report)
	e.problem(e.req(http.MethodPost, launch, map[string]any{}, tok), http.StatusUnprocessableEntity, "limit_exceeded")

	e.graphene.mu.Lock()
	e.graphene.quotaUnavailableReason = "permission_denied"
	e.graphene.mu.Unlock()
	e.want(e.req(http.MethodPost, path+":refresh", nil, tok), http.StatusOK, nil)
	e.want(e.req(http.MethodGet, path, nil, tok), http.StatusOK, &report)
	if report.UnavailableReason != "permission_denied" || report.Scope != "cloud:b1gcloud000000000000" || len(report.Quotas) != 0 || report.Stale {
		t.Fatalf("unavailable snapshot was not persisted: %+v", report)
	}
	var launched runView
	e.want(e.req(http.MethodPost, launch, map[string]any{}, tok), http.StatusCreated, &launched)
	e.want(e.req(http.MethodPost, base+"/runs/"+launched.ID+":cancel", nil, tok), http.StatusOK, nil)

	e.graphene.mu.Lock()
	e.graphene.quotaUnavailableReason = ""
	e.graphene.mu.Unlock()
	report.UnavailableReason = ""
	e.want(e.req(http.MethodPost, path+":refresh", nil, tok), http.StatusOK, &report)
	if report.UnavailableReason != "" || len(report.Quotas) != 1 {
		t.Fatalf("quota visibility did not recover: %+v", report)
	}
	e.problem(e.req(http.MethodPost, launch, map[string]any{}, tok), http.StatusUnprocessableEntity, "limit_exceeded")

	e.graphene.mu.Lock()
	e.graphene.quotaError = connect.NewError(connect.CodeUnavailable, nil)
	e.graphene.mu.Unlock()
	e.problem(e.req(http.MethodPost, path+":refresh", nil, tok), http.StatusServiceUnavailable, "unavailable")
}

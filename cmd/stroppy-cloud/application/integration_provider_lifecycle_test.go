//go:build integration

package application

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
)

func TestE2EProviderLifecycle(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, tn := runFixture(t, e)
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	e.want(e.req(http.MethodGet, base+"/providers", nil, tok), http.StatusOK, &list)
	require.Len(t, list.Data, 1)
	id := list.Data[0].ID
	path := base + "/providers/" + id
	read := func() provider.Profile {
		p, err := e.app.services.Providers.ByID(e.ctx, tn.ID, mustUUID(t, id))
		require.NoError(t, err)
		return p
	}

	t.Run("transport failure reattaches durable setup", func(t *testing.T) {
		e.graphene.mu.Lock()
		e.graphene.configResultError = connect.NewError(connect.CodeUnavailable, errors.New("temporary connection loss"))
		e.graphene.mu.Unlock()
		e.want(e.req(http.MethodPost, path+":verify", nil, tok), http.StatusAccepted, nil)
		p := read()
		require.Equal(t, provider.StatusVerifying, p.Status)
		e.graphene.mu.Lock()
		e.graphene.configResultError = nil
		e.graphene.mu.Unlock()
		eventually(t, 10*time.Second, func() bool {
			require.NoError(t, e.app.services.Providers.RecoverPending(e.ctx))
			return read().Status == provider.StatusReady
		})
		require.Equal(t, p.VerifyRunID, read().VerifyRunID)
	})

	t.Run("rotation selects a schema-valid immutable credential version", func(t *testing.T) {
		old := read()
		raw, ok := e.graphene.secret(tn.GrapheneNamespace, provider.ActiveCredentials(old))
		require.True(t, ok)
		var credentials map[string]any
		require.NoError(t, json.Unmarshal([]byte(raw), &credentials))
		e.want(e.req(http.MethodPatch, path, map[string]any{"credentials": credentials}, tok), http.StatusOK, nil)
		eventually(t, 10*time.Second, func() bool { return read().Status == provider.StatusReady })
		current := read()
		require.NotEqual(t, provider.ActiveCredentials(old), provider.ActiveCredentials(current))
		require.LessOrEqual(t, len(provider.ActiveCredentials(current)), 63)
		require.Contains(t, current.SecretNames, provider.ActiveCredentials(old))
	})
	t.Run("active runs and kept stands protect the profile", func(t *testing.T) {
		e.graphene.hold("segment.started")
		var launched runView
		e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", nil, tok), http.StatusCreated, &launched)
		e.problem(e.req(http.MethodDelete, path, nil, tok), http.StatusConflict, "conflict")
		e.problem(e.req(http.MethodPost, path+":verify", nil, tok), http.StatusConflict, "conflict")
		e.problem(e.req(http.MethodPatch, path, map[string]any{"settings": map[string]any{"folder_id": "b1gfolder00000000000", "cloud_id": "b1gcloud000000000000", "zone": "ru-central1-a", "network": map[string]any{"kind": "create"}}}, tok), http.StatusConflict, "conflict")
		require.Equal(t, provider.StatusReady, read().Status)
		e.app.services.Projector.Tick(e.ctx)
		e.graphene.release(launched.ID)
		eventually(t, 15*time.Second, func() bool {
			e.want(e.req(http.MethodGet, base+"/runs/"+launched.ID, nil, tok), http.StatusOK, &launched)
			return launched.Status == "completed"
		})
		e.problem(e.req(http.MethodDelete, path, nil, tok), http.StatusConflict, "conflict")
		e.want(e.req(http.MethodPost, base+"/runs/"+launched.ID+":keep-release", nil, tok), http.StatusOK, nil)
	})
	t.Run("failed cleanup keeps credentials and blocks new runs", func(t *testing.T) {
		e.graphene.mu.Lock()
		e.graphene.configDeleteFailure = "managed resource still exists"
		e.graphene.mu.Unlock()
		e.problem(e.req(http.MethodDelete, path, nil, tok), http.StatusConflict, "conflict")
		require.Equal(t, provider.StatusDeleteFailed, read().Status)
		_, exists := e.graphene.secret(tn.GrapheneNamespace, provider.CredentialsSecret(mustUUID(t, id)))
		require.True(t, exists)
		response := e.req(http.MethodPost, base+"/tests/"+testID+":launch", nil, tok)
		require.NotEqual(t, http.StatusCreated, response.Status)
		e.graphene.mu.Lock()
		e.graphene.configDeleteFailure = ""
		e.graphene.mu.Unlock()
		e.want(e.req(http.MethodDelete, path, nil, tok), http.StatusNoContent, nil)
		_, exists = e.graphene.secret(tn.GrapheneNamespace, provider.CredentialsSecret(mustUUID(t, id)))
		require.False(t, exists)
	})
}

func TestE2ETenantProviderCleanup(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, tn := runFixture(t, e)
	var profiles struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	e.want(e.req(http.MethodGet, base+"/providers", nil, tok), http.StatusOK, &profiles)
	require.Len(t, profiles.Data, 1)
	p, err := e.app.services.Providers.ByID(e.ctx, tn.ID, mustUUID(t, profiles.Data[0].ID))
	require.NoError(t, err)
	tenantPath := "/api/v1/tenants/" + tn.Slug

	e.graphene.mu.Lock()
	e.graphene.configDeleteFailure = "Crossplane cleanup pending"
	e.graphene.mu.Unlock()
	e.problem(e.req(http.MethodDelete, tenantPath, nil, tok), http.StatusConflict, "conflict")
	require.True(t, e.graphene.hasNamespace(tn.GrapheneNamespace))
	_, exists := e.graphene.secret(tn.GrapheneNamespace, provider.ActiveCredentials(p))
	require.True(t, exists)
	response := e.req(http.MethodPost, base+"/tests/"+testID+":launch", nil, tok)
	require.NotEqual(t, http.StatusCreated, response.Status)

	// Admission remains sealed even if an unrelated profile could be ready.
	raw, exists := e.graphene.secret(tn.GrapheneNamespace, provider.ActiveCredentials(p))
	require.True(t, exists)
	var credentials map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &credentials))
	e.problem(e.req(http.MethodPost, base+"/providers", map[string]any{
		"name": "late profile", "kind": p.Kind, "settings": p.Settings, "credentials": credentials,
	}, tok), http.StatusConflict, "conflict")

	e.graphene.mu.Lock()
	e.graphene.configDeleteFailure = ""
	e.graphene.mu.Unlock()
	e.want(e.req(http.MethodDelete, tenantPath, nil, tok), http.StatusNoContent, nil)
	require.False(t, e.graphene.hasNamespace(tn.GrapheneNamespace))
	_, exists = e.graphene.secret(tn.GrapheneNamespace, provider.ActiveCredentials(p))
	require.False(t, exists)
}

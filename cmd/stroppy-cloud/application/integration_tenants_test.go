//go:build integration

package application

import (
	"net/http"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
)

func TestE2ETenantLifecycle(t *testing.T) {
	e := e2eServer(t)
	owner := e.person(slug("owner")+"@example.com", "Owner")
	tn := e.tenant(owner, "Acme")
	tok := e.token(owner, tn)

	if !e.graphene.hasNamespace(tn.GrapheneNamespace) {
		t.Fatalf("namespace %s not applied in graphene", tn.GrapheneNamespace)
	}

	t.Run("get as owner", func(t *testing.T) {
		var got struct {
			Slug        string `json:"slug"`
			MemberCount int    `json:"member_count"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug, nil, tok), http.StatusOK, &got)
		if got.Slug != tn.Slug || got.MemberCount != 1 {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("patch", func(t *testing.T) {
		var got struct {
			Name       string  `json:"name"`
			PublicName *string `json:"public_name"`
		}
		e.want(e.req(http.MethodPatch, "/api/v1/tenants/"+tn.Slug, map[string]any{"name": "Acme Inc", "public_name": "ACME"}, tok), http.StatusOK, &got)
		if got.Name != "Acme Inc" || got.PublicName == nil || *got.PublicName != "ACME" {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("unknown tenant is 404", func(t *testing.T) {
		e.problem(e.req(http.MethodGet, "/api/v1/tenants/nope", nil, tok), http.StatusNotFound, "not_found")
	})

	t.Run("no token is 401", func(t *testing.T) {
		e.problem(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug, nil, ""), http.StatusUnauthorized, "unauthenticated")
	})

	t.Run("garbage token is 401", func(t *testing.T) {
		e.problem(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug, nil, "stc_abcdefgh_nope"), http.StatusUnauthorized, "unauthenticated")
	})

	t.Run("second owned tenant is 409", func(t *testing.T) {
		e.problem(e.req(http.MethodPost, "/api/v1/tenants", map[string]any{"name": "Another"}, tok), http.StatusForbidden, "forbidden") // tokens cannot create
	})

	t.Run("my tenants lists it", func(t *testing.T) {
		var got struct {
			Data []struct {
				Role string `json:"role"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/tenants", nil, tok), http.StatusOK, &got)
		if len(got.Data) != 1 || got.Data[0].Role != "owner" {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("delete retires the namespace", func(t *testing.T) {
		e.want(e.req(http.MethodDelete, "/api/v1/tenants/"+tn.Slug, nil, tok), http.StatusNoContent, nil)
		if e.graphene.hasNamespace(tn.GrapheneNamespace) {
			t.Fatal("namespace still present")
		}
		// The token died with the tenant.
		e.problem(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug, nil, tok), http.StatusUnauthorized, "unauthenticated")
	})
}

func TestE2EMembersAndRoles(t *testing.T) {
	e := e2eServer(t)
	owner := e.person(slug("owner")+"@example.com", "Owner")
	bob := e.person(slug("bob")+"@example.com", "Bob")
	tn := e.tenant(owner, "Team")
	e.member(tn, bob, tenant.RoleViewer)
	ownerTok := e.token(owner, tn)
	bobTok := e.token(bob, tn)
	viewerSvc := e.serviceToken(owner, tn, "viewer")

	t.Run("members listed with roles", func(t *testing.T) {
		var got struct {
			Data []struct {
				User struct {
					ID string `json:"id"`
				} `json:"user"`
				Role string `json:"role"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug+"/members", nil, bobTok), http.StatusOK, &got)
		if len(got.Data) != 2 {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("viewer cannot change roles", func(t *testing.T) {
		e.problem(e.req(http.MethodPatch, "/api/v1/tenants/"+tn.Slug+"/members/"+bob.UserID.String(), map[string]any{"role": "admin"}, bobTok), http.StatusForbidden, "forbidden")
	})

	t.Run("service token is confined to its role", func(t *testing.T) {
		e.problem(e.req(http.MethodPost, "/api/v1/tenants/"+tn.Slug+"/invites", map[string]any{"email": "x@example.com", "role": "member"}, viewerSvc), http.StatusForbidden, "forbidden")
		e.want(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug, nil, viewerSvc), http.StatusOK, nil)
	})

	t.Run("owner promotes bob to admin", func(t *testing.T) {
		var got struct {
			Role string `json:"role"`
		}
		e.want(e.req(http.MethodPatch, "/api/v1/tenants/"+tn.Slug+"/members/"+bob.UserID.String(), map[string]any{"role": "admin"}, ownerTok), http.StatusOK, &got)
		if got.Role != "admin" {
			t.Fatalf("role = %s", got.Role)
		}
	})

	t.Run("owner role only by transfer", func(t *testing.T) {
		e.problem(e.req(http.MethodPatch, "/api/v1/tenants/"+tn.Slug+"/members/"+bob.UserID.String(), map[string]any{"role": "owner"}, ownerTok), http.StatusUnprocessableEntity, "invalid")
	})

	t.Run("transfer ownership", func(t *testing.T) {
		var got struct {
			Owner struct {
				ID string `json:"id"`
			} `json:"owner"`
		}
		e.want(e.req(http.MethodPost, "/api/v1/tenants/"+tn.Slug+":transfer", map[string]any{"user_id": bob.UserID.String()}, ownerTok), http.StatusOK, &got)
		if got.Owner.ID != bob.UserID.String() {
			t.Fatalf("owner = %s", got.Owner.ID)
		}
		// The old owner is admin now and cannot transfer back without being owner.
		e.problem(e.req(http.MethodPost, "/api/v1/tenants/"+tn.Slug+":transfer", map[string]any{"user_id": owner.UserID.String()}, ownerTok), http.StatusForbidden, "forbidden")
	})

	t.Run("owner cannot be removed; admin can leave", func(t *testing.T) {
		e.problem(e.req(http.MethodDelete, "/api/v1/tenants/"+tn.Slug+"/members/"+bob.UserID.String(), nil, ownerTok), http.StatusForbidden, "forbidden")
		e.want(e.req(http.MethodDelete, "/api/v1/tenants/"+tn.Slug+"/members/"+owner.UserID.String(), nil, ownerTok), http.StatusNoContent, nil)
		// Personal tokens die with the membership.
		e.problem(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug, nil, ownerTok), http.StatusUnauthorized, "unauthenticated")
	})

	t.Run("audit has the trail", func(t *testing.T) {
		// bob's first token was minted as a viewer and stays a viewer:
		// a personal token never grows with its owner. Mint a fresh one.
		bobTok = e.token(bob, tn)
		var got struct {
			Data []struct {
				Action string `json:"action"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/t/"+tn.Slug+"/audit?limit=100", nil, bobTok), http.StatusOK, &got)
		seen := map[string]bool{}
		for _, d := range got.Data {
			seen[d.Action] = true
		}
		for _, want := range []string{"tenant.create", "member.role", "tenant.transfer", "member.leave", "token.create"} {
			if !seen[want] {
				t.Errorf("audit lacks %s: %+v", want, got.Data)
			}
		}
	})
}

func TestE2EInvites(t *testing.T) {
	e := e2eServer(t)
	owner := e.person(slug("owner")+"@example.com", "Owner")
	tn := e.tenant(owner, "Inv")
	ownerTok := e.token(owner, tn)
	carolEmail := slug("carol") + "@example.com"

	var inv struct {
		ID     string `json:"id"`
		Email  string `json:"email"`
		Status string `json:"status"`
	}
	e.want(e.req(http.MethodPost, "/api/v1/tenants/"+tn.Slug+"/invites", map[string]any{"email": carolEmail, "role": "member", "message": "hi"}, ownerTok), http.StatusCreated, &inv)
	if inv.Email != carolEmail || inv.Status != "pending" {
		t.Fatalf("got %+v", inv)
	}

	t.Run("duplicate pending invite is 409", func(t *testing.T) {
		e.problem(e.req(http.MethodPost, "/api/v1/tenants/"+tn.Slug+"/invites", map[string]any{"email": carolEmail, "role": "viewer"}, ownerTok), http.StatusConflict, "conflict")
	})

	t.Run("listed for the tenant", func(t *testing.T) {
		var got struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug+"/invites", nil, ownerTok), http.StatusOK, &got)
		if len(got.Data) != 1 || got.Data[0].ID != inv.ID {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("accepted by the invitee", func(t *testing.T) {
		carol := e.person(carolEmail, "Carol")
		// Carol has no tenant yet, so no personal token; accept through the service as her first login would.
		m, err := e.app.services.Tenants.Accept(ctxOf(carol), carol, mustUUID(t, inv.ID))
		if err != nil {
			t.Fatalf("accept: %v", err)
		}
		if m.Role != tenant.RoleMember || m.Tenant.ID != tn.ID {
			t.Fatalf("membership %+v", m)
		}
		carolTok := e.token(carol, tn)
		e.want(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug, nil, carolTok), http.StatusOK, nil)
		// A second accept is a conflict.
		if _, err := e.app.services.Tenants.Accept(ctxOf(carol), carol, mustUUID(t, inv.ID)); err == nil {
			t.Fatal("second accept succeeded")
		}
	})

	t.Run("pending invite applies on first login", func(t *testing.T) {
		daveEmail := slug("dave") + "@example.com"
		e.want(e.req(http.MethodPost, "/api/v1/tenants/"+tn.Slug+"/invites", map[string]any{"email": daveEmail, "role": "viewer"}, ownerTok), http.StatusCreated, nil)
		dave := e.person(daveEmail, "Dave")
		if err := e.app.services.Tenants.ApplyPending(ctxOf(dave), dave.UserID, daveEmail); err != nil {
			t.Fatalf("apply pending: %v", err)
		}
		daveTok := e.token(dave, tn)
		var got struct {
			Data []struct {
				Role string `json:"role"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/tenants", nil, daveTok), http.StatusOK, &got)
		if len(got.Data) != 1 || got.Data[0].Role != "viewer" {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("revoke", func(t *testing.T) {
		var inv2 struct {
			ID string `json:"id"`
		}
		e.want(e.req(http.MethodPost, "/api/v1/tenants/"+tn.Slug+"/invites", map[string]any{"email": slug("eve") + "@example.com", "role": "viewer"}, ownerTok), http.StatusCreated, &inv2)
		e.want(e.req(http.MethodDelete, "/api/v1/tenants/"+tn.Slug+"/invites/"+inv2.ID, nil, ownerTok), http.StatusNoContent, nil)
		e.problem(e.req(http.MethodDelete, "/api/v1/tenants/"+tn.Slug+"/invites/"+inv2.ID, nil, ownerTok), http.StatusConflict, "conflict")
	})
}

func TestE2ETokens(t *testing.T) {
	e := e2eServer(t)
	owner := e.person(slug("owner")+"@example.com", "Owner")
	tn := e.tenant(owner, "Tok")
	ownerTok := e.token(owner, tn)

	t.Run("tokens cannot mint tokens", func(t *testing.T) {
		e.problem(e.req(http.MethodPost, "/api/v1/me/tokens", map[string]any{"name": "ro", "tenant_id": tn.ID.String()}, ownerTok), http.StatusForbidden, "forbidden")
	})

	t.Run("personal token with weaker role", func(t *testing.T) {
		// Minted as the person (the SPA path); the API mints only for people.
		c, err := e.app.services.Tokens.CreatePersonal(ctxOf(owner), owner, "ro", tn.ID, "viewer", nil)
		if err != nil {
			t.Fatal(err)
		}
		viewer := e.person(slug("viewer")+"@example.com", "Viewer")
		e.member(tn, viewer, tenant.RoleViewer)
		if _, err := e.app.services.Tokens.CreatePersonal(ctxOf(viewer), viewer, "too-strong", tn.ID, "member", nil); err == nil {
			t.Fatal("a viewer minted a member token")
		}
		created := struct {
			ID     string
			Secret string
			Role   string
			Kind   string
		}{c.ID.String(), c.Secret, c.Role, string(c.Kind)}
		if created.Role != "viewer" || created.Kind != "personal" || created.Secret == "" {
			t.Fatalf("got %+v", created)
		}
		// Confined: cannot invite.
		e.problem(e.req(http.MethodPost, "/api/v1/tenants/"+tn.Slug+"/invites", map[string]any{"email": "x@example.com", "role": "viewer"}, created.Secret), http.StatusForbidden, "forbidden")
		// Listed, then revoked.
		var list struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/me/tokens", nil, ownerTok), http.StatusOK, &list)
		if len(list.Data) != 2 {
			t.Fatalf("got %d tokens", len(list.Data))
		}
		e.want(e.req(http.MethodDelete, "/api/v1/me/tokens/"+created.ID, nil, ownerTok), http.StatusNoContent, nil)
		e.problem(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug, nil, created.Secret), http.StatusUnauthorized, "unauthenticated")
	})

	t.Run("service token role is bounded", func(t *testing.T) {
		e.problem(e.req(http.MethodPost, "/api/v1/tenants/"+tn.Slug+"/tokens", map[string]any{"name": "ci", "role": "admin"}, ownerTok), http.StatusUnprocessableEntity, "invalid")
		var created struct {
			ID     string `json:"id"`
			Secret string `json:"secret"`
		}
		e.want(e.req(http.MethodPost, "/api/v1/tenants/"+tn.Slug+"/tokens", map[string]any{"name": "ci", "role": "member"}, ownerTok), http.StatusCreated, &created)
		e.want(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug+"/tokens", nil, ownerTok), http.StatusOK, nil)
		e.want(e.req(http.MethodDelete, "/api/v1/tenants/"+tn.Slug+"/tokens/"+created.ID, nil, ownerTok), http.StatusNoContent, nil)
		e.problem(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug, nil, created.Secret), http.StatusUnauthorized, "unauthenticated")
	})
}

func TestE2EMeAndPublic(t *testing.T) {
	e := e2eServer(t)
	owner := e.person(slug("me")+"@example.com", "Me")
	tn := e.tenant(owner, "Me Myself")
	tok := e.token(owner, tn)

	t.Run("me", func(t *testing.T) {
		var me struct {
			ID          string `json:"id"`
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/me", nil, tok), http.StatusOK, &me)
		if me.ID != owner.UserID.String() || me.DisplayName != "Me" {
			t.Fatalf("got %+v", me)
		}
		e.want(e.req(http.MethodPatch, "/api/v1/me", map[string]any{"display_name": "Mia", "preferences": map[string]any{"theme": "dark"}}, tok), http.StatusOK, &me)
		if me.DisplayName != "Mia" {
			t.Fatalf("got %+v", me)
		}
	})

	t.Run("public config and health", func(t *testing.T) {
		var cfg struct {
			Auth struct {
				Mode   string `json:"mode"`
				Kratos struct {
					PublicURL string `json:"public_url"`
				} `json:"kratos"`
			} `json:"auth"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/public/config", nil, ""), http.StatusOK, &cfg)
		if cfg.Auth.Mode != "kratos" || cfg.Auth.Kratos.PublicURL == "" {
			t.Fatalf("got %+v", cfg)
		}
		var health struct {
			Status string `json:"status"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/public/health", nil, ""), http.StatusOK, &health)
		if health.Status != "ok" {
			t.Fatalf("health %+v", health)
		}
		e.want(e.req(http.MethodGet, "/healthz/readiness", nil, ""), http.StatusOK, nil)
	})
}

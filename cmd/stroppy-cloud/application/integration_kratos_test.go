//go:build integration

package application

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

/*
The Kratos contract of the verifier (deployments/kratos/README.md):
a verified session JWT becomes an actor; an unverified e-mail does not
claim invites; anything else is unauthenticated.
*/

func TestE2EKratosJWT(t *testing.T) {
	e := e2eServerWith(t, e2eOptions{JWTKeys: true})

	owner := e.person("owner@example.com", "Owner")
	tn := e.tenant(owner, slug("kratos"))

	// A pending invite for the guest's e-mail.
	if r := e.req("POST", "/api/v1/tenants/"+tn.Slug+"/invites", map[string]any{"email": "guest@example.com", "role": "viewer"}, e.token(owner, tn)); r.Status != 201 && r.Status != 200 {
		t.Fatalf("invite: status %d: %s", r.Status, r.Body)
	}

	// A verified guest: the profile fills from claims, the invite applies.
	guestID := uuid.New()
	guest := e.jwt.token(t, guestID.String(), "guest@example.com", true, "Guest")
	var me struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	}
	e.want(e.req("GET", "/api/v1/me", nil, guest), 200, &me)
	if me.ID != guestID.String() || me.Email != "guest@example.com" || me.DisplayName != "Guest" {
		t.Fatalf("me from claims: %+v", me)
	}
	var memberships struct {
		Data []struct {
			Tenant struct {
				Slug string `json:"slug"`
			} `json:"tenant"`
			Role string `json:"role"`
		} `json:"data"`
	}
	e.want(e.req("GET", "/api/v1/tenants", nil, guest), 200, &memberships)
	found := false
	for _, m := range memberships.Data {
		if m.Tenant.Slug == tn.Slug && m.Role == "viewer" {
			found = true
		}
	}
	if !found {
		t.Fatalf("verified e-mail did not claim the invite: %+v", memberships.Data)
	}

	// An unverified e-mail: actor is valid, the invite does NOT apply and
	// the e-mail is not recorded.
	strangerID := uuid.New()
	stranger := e.jwt.token(t, strangerID.String(), "guest@example.com", false, "Stranger")
	var strangerMe struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	e.want(e.req("GET", "/api/v1/me", nil, stranger), 200, &strangerMe)
	if strangerMe.Email != "" {
		t.Fatalf("unverified e-mail must not be recorded: %+v", strangerMe)
	}
	var strangerTenants struct {
		Data []json.RawMessage `json:"data"`
	}
	e.want(e.req("GET", "/api/v1/tenants", nil, stranger), 200, &strangerTenants)
	if len(strangerTenants.Data) != 0 {
		t.Fatalf("unverified e-mail claimed the invite: %+v", strangerTenants.Data)
	}

	// Garbage and foreign signatures are unauthenticated.
	e.problem(e.req("GET", "/api/v1/me", nil, "eyJhbGciOiJFUzI1NiJ9.garbage"), 401, "unauthenticated")
	foreign := newJWTSigner(t)
	e.problem(e.req("GET", "/api/v1/me", nil, foreign.token(t, uuid.NewString(), "x@example.com", true, "X")), 401, "unauthenticated")
}

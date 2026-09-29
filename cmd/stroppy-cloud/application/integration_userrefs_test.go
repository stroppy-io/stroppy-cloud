//go:build integration

package application

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/profile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
)

// UserRef enrichment: display data for every ref, emails only inside the
// tenant boundary (co-members) or for a platform admin.
func TestE2EUserRefs(t *testing.T) {
	e := e2eServer(t)
	ctx := context.Background()
	a := e.person(slug("a")+"@example.com", "Alice")
	x := e.tenant(a, slug("x"))
	c := e.person(slug("c")+"@example.com", "Carol")
	e.member(x, c, tenant.RoleMember)
	b := e.person(slug("b")+"@example.com", "Bob")
	y := e.tenant(b, slug("y"))
	nameless := e.person(slug("n")+"@example.com", "")
	e.member(y, nameless, tenant.RoleMember)
	root := e.person("root@example.com", "Root")
	rootTenant := e.tenant(root, slug("ops"))

	t.Run("query scopes emails to co-members", func(t *testing.T) {
		refs, err := e.app.services.Profiles.Refs(ctx, profile.Viewer{UserID: a.UserID}, []uuid.UUID{a.UserID, b.UserID, c.UserID, nameless.UserID, uuid.New()})
		if err != nil {
			t.Fatal(err)
		}
		if len(refs) != 4 {
			t.Fatalf("unknown id must be absent: %+v", refs)
		}
		if r := refs[b.UserID]; r.DisplayName != "Bob" || r.Email != "" || r.Name() != "Bob" {
			t.Fatalf("B from another tenant: %+v", r)
		}
		if r := refs[nameless.UserID]; r.Email != "" || r.Name() != "" {
			t.Fatalf("nameless stranger must show nothing derived from the email: %+v", r)
		}
		if r := refs[c.UserID]; r.Email != c.Email || r.Name() != "Carol" {
			t.Fatalf("co-member C: %+v", r)
		}
		if r := refs[a.UserID]; r.Email != a.Email {
			t.Fatalf("self: %+v", r)
		}
		// A token confined to another tenant of A does not see X's members.
		scoped, err := e.app.services.Profiles.Refs(ctx, profile.Viewer{UserID: a.UserID, Tenant: y.ID}, []uuid.UUID{c.UserID})
		if err != nil {
			t.Fatal(err)
		}
		if scoped[c.UserID].Email != "" {
			t.Fatalf("token scope leaked: %+v", scoped[c.UserID])
		}
		all, err := e.app.services.Profiles.Refs(ctx, profile.Viewer{UserID: root.UserID, Reveal: true}, []uuid.UUID{b.UserID, nameless.UserID})
		if err != nil {
			t.Fatal(err)
		}
		if all[b.UserID].Email != b.Email || all[nameless.UserID].Name() != nameless.Email {
			t.Fatalf("admin: %+v", all)
		}
	})

	type ref struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
	}
	t.Run("responses carry display data", func(t *testing.T) {
		var got struct {
			Owner ref `json:"owner"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/tenants/"+x.Slug, nil, e.token(a, x)), http.StatusOK, &got)
		if got.Owner.ID != a.UserID.String() || got.Owner.DisplayName != "Alice" || got.Owner.Email != a.Email {
			t.Fatalf("tenant owner %+v", got.Owner)
		}
		var list struct {
			Data []struct {
				Owner ref `json:"owner"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/admin/tenants?search="+y.Slug, nil, e.token(root, rootTenant)), http.StatusOK, &list)
		if len(list.Data) != 1 || list.Data[0].Owner.DisplayName != "Bob" || list.Data[0].Owner.Email != b.Email {
			t.Fatalf("admin sees B %+v", list.Data)
		}
	})
}

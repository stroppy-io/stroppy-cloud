package api

import (
	"context"
	"reflect"
	"sync"

	"github.com/google/uuid"
	"github.com/gopherex/xlog"
	"github.com/ogen-go/ogen/middleware"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/profile"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

/*
USER REFS: mappers put only the subject id into oas.UserRef; display data
is filled in ONE place, after the handler, for the whole response: walk it,
collect every UserRef id, load the profiles with one query, fill the refs.
The pass runs per request (no cache, so a profile change is visible on the
next response). Anonymous callers (public share/rating) get ids only.
Emails never cross the tenant boundary: the query returns a subject's
email only to co-members (or a platform admin), and display_name never
falls back to a withheld email. Unknown ids (deleted users, config
admins without a profile) keep the id only.
*/

var userRefType = reflect.TypeFor[oas.UserRef]()

// UsersMiddleware is the ogen middleware that fills UserRefs in responses.
func UsersMiddleware(h *Handler) middleware.Middleware {
	return func(req middleware.Request, next middleware.Next) (middleware.Response, error) {
		resp, err := next(req)
		if err != nil || resp.Type == nil {
			return resp, err
		}
		resp.Type = h.fillUsers(req.Context, resp.Type)
		return resp, nil
	}
}

// fillUsers returns v with every reachable UserRef filled. Pointers are
// filled in place; a non-pointer v is copied, filled and returned.
func (h *Handler) fillUsers(ctx context.Context, v any) any {
	if v == nil || h.deps.Profiles == nil || auth.ActorFrom(ctx).IsAnonymous() {
		return v
	}
	rv := reflect.ValueOf(v)
	if !mayHoldUserRef(rv.Type()) {
		return v
	}
	root := reflect.New(rv.Type()).Elem()
	root.Set(rv)

	seen := map[uuid.UUID]struct{}{}
	walkUserRefs(root, func(r *oas.UserRef) {
		if id, err := uuid.Parse(r.ID); err == nil {
			seen[id] = struct{}{}
		}
	})
	if len(seen) == 0 {
		return v
	}
	ids := make([]uuid.UUID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	refs, err := h.deps.Profiles.Refs(ctx, h.viewerOf(ctx), ids)
	if err != nil {
		if h.deps.Log != nil {
			h.deps.Log.Warn("user refs: load profiles failed, ids only", xlog.ErrorCause(err))
		}
		return v
	}
	walkUserRefs(root, func(r *oas.UserRef) {
		id, err := uuid.Parse(r.ID)
		if err != nil {
			return
		}
		if p, ok := refs[id]; ok {
			fillUserRef(r, p)
		}
	})
	return root.Interface()
}

// viewerOf scopes the email disclosure to the caller: co-members only
// (only the token's tenant for an API token), everything for a platform
// admin (admin endpoints accept admins' tokens too).
func (h *Handler) viewerOf(ctx context.Context) profile.Viewer {
	a := auth.ActorFrom(ctx)
	v := profile.Viewer{UserID: a.UserID, Reveal: h.deps.Admin != nil && h.deps.Admin.IsAdmin(ctx, a)}
	if a.IsAPIToken() {
		v.Tenant = a.TokenTenant
	}
	return v
}

// fillUserRef sets the unset display fields from the profile.
func fillUserRef(r *oas.UserRef, p profile.Ref) {
	if name := p.Name(); name != "" && r.DisplayName.Or("") == "" {
		r.DisplayName = oas.NewOptString(name)
	}
	if p.Email != "" && !r.Email.Set {
		r.Email = oas.NewOptString(p.Email)
	}
	if p.Avatar != "" && r.Avatar.Or("") == "" {
		r.Avatar = oas.NewOptString(p.Avatar)
	}
}

// walkUserRefs visits every UserRef under v; v must be settable. Values
// that are not addressable in place (map entries, interface contents) are
// copied, visited and written back.
func walkUserRefs(v reflect.Value, visit func(*oas.UserRef)) {
	t := v.Type()
	if !mayHoldUserRef(t) {
		return
	}
	if t == userRefType {
		visit(v.Addr().Interface().(*oas.UserRef)) //nolint:forcetypeassert,errcheck // checked by type
		return
	}
	switch v.Kind() { //nolint:exhaustive // only containers can reach a UserRef
	case reflect.Pointer:
		if !v.IsNil() {
			walkUserRefs(v.Elem(), visit)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if f := v.Field(i); f.CanSet() {
				walkUserRefs(f, visit)
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			walkUserRefs(v.Index(i), visit)
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			val := iter.Value()
			if val.Kind() == reflect.Interface && (val.IsNil() || !mayHoldUserRef(val.Elem().Type())) {
				continue
			}
			c := reflect.New(t.Elem()).Elem()
			c.Set(val)
			walkUserRefs(c, visit)
			v.SetMapIndex(iter.Key(), c)
		}
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		inner := v.Elem()
		if !mayHoldUserRef(inner.Type()) {
			return
		}
		if inner.Kind() == reflect.Pointer {
			walkUserRefs(inner, visit) // pointee is settable in place
			return
		}
		c := reflect.New(inner.Type()).Elem()
		c.Set(inner)
		walkUserRefs(c, visit)
		v.Set(c)
	}
}

// holdCache memoizes mayHoldUserRef per type: most response types hold no
// UserRef and are skipped without a walk.
var holdCache sync.Map // reflect.Type -> bool

// mayHoldUserRef reports whether a value of t can reach a UserRef.
// Interfaces are "maybe": their dynamic type is checked during the walk.
func mayHoldUserRef(t reflect.Type) bool {
	if v, ok := holdCache.Load(t); ok {
		return v.(bool) //nolint:forcetypeassert,errcheck // cache holds bools
	}
	return computeHold(t, map[reflect.Type]bool{})
}

func computeHold(t reflect.Type, visiting map[reflect.Type]bool) bool {
	if v, ok := holdCache.Load(t); ok {
		return v.(bool) //nolint:forcetypeassert,errcheck // cache holds bools
	}
	if visiting[t] {
		return false // the cycle's other members decide
	}
	visiting[t] = true
	var res bool
	switch {
	case t == userRefType:
		res = true
	case t.Kind() == reflect.Interface:
		res = true
	case t.Kind() == reflect.Pointer, t.Kind() == reflect.Slice, t.Kind() == reflect.Array:
		res = computeHold(t.Elem(), visiting)
	case t.Kind() == reflect.Map:
		res = computeHold(t.Elem(), visiting)
	case t.Kind() == reflect.Struct:
		for i := range t.NumField() {
			f := t.Field(i)
			if f.IsExported() && computeHold(f.Type, visiting) {
				res = true
				break
			}
		}
	}
	delete(visiting, t)
	// A false inside an unfinished cycle may be premature; cache only
	// definitive answers from the outermost call.
	if res || len(visiting) == 0 {
		holdCache.Store(t, res)
	}
	return res
}

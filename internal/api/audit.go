package api

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"

	"github.com/go-faster/jx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// ListTenantAudit — the tenant slice of the audit log (admin+).
func (h *Handler) ListTenantAudit(ctx context.Context, params oas.ListTenantAuditParams) (*oas.AuditPage, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	t, _, err := h.deps.Tenants.Require(ctx, a, params.Slug, tenant.RoleAdmin)
	if err != nil {
		return nil, err
	}
	q := audit.Query{TenantID: t.ID, Action: params.Action.Or(""), ActorID: params.Actor.Or(""), Limit: params.Limit.Or(50)}
	if c, ok := params.Cursor.Get(); ok && c != "" {
		id, err := strconv.ParseInt(c, 10, 64)
		if err != nil {
			return nil, errs.Invalid("cursor").Because("cursor_invalid")
		}
		q.BeforeID = id
	}
	if s, ok := params.Since.Get(); ok {
		q.Since = &s
	}
	limit := q.Limit
	q.Limit = limit + 1
	entries, err := h.deps.Audit.OfTenant(ctx, q)
	if err != nil {
		return nil, err
	}
	page := &oas.AuditPage{Data: make([]oas.AuditEntry, 0, len(entries))}
	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}
	for _, e := range entries {
		if entry, ok := auditOf(e); ok {
			page.Data = append(page.Data, entry)
		}
	}
	page.Meta.HasMore = oas.NewOptBool(hasMore)
	if hasMore {
		page.Meta.NextCursor = oas.NewOptNilString(strconv.FormatInt(entries[len(entries)-1].ID, 10))
	}
	return page, nil
}

// auditOf is one entry of the log; false for an entry recorded under an
// action or target kind the contract no longer names (older servers).
func auditOf(e audit.Entry) (oas.AuditEntry, bool) {
	if !slices.Contains(audit.Actions, e.Action) || (e.Target.Kind != "" && !slices.Contains(audit.TargetKinds, e.Target.Kind)) {
		return oas.AuditEntry{}, false
	}
	out := oas.AuditEntry{
		ID: strconv.FormatInt(e.ID, 10), At: e.At, Action: oas.AuditAction(e.Action),
		Actor: oas.AuditEntryActor{Kind: oas.AuditEntryActorKind(e.ActorKind)},
	}
	if e.ActorID != "" {
		out.Actor.ID = oas.NewOptString(e.ActorID)
	}
	if e.ActorName != "" {
		out.Actor.DisplayName = oas.NewOptString(e.ActorName)
	}
	if e.TenantID != nil {
		out.Tenant = oas.NewOptRef(oas.Ref{ID: *e.TenantID})
	}
	if e.Target.Kind != "" {
		out.Target.Kind = oas.NewOptAuditTargetKind(oas.AuditTargetKind(e.Target.Kind))
	}
	if e.Target.ID != "" {
		out.Target.ID = oas.NewOptString(e.Target.ID)
	}
	if e.Target.Name != "" {
		out.Target.Name = oas.NewOptString(e.Target.Name)
	}
	if e.RequestID != "" {
		out.RequestID = oas.NewOptString(e.RequestID)
	}
	if len(e.Details) > 0 {
		d := oas.AuditEntryDetails{}
		for k, v := range e.Details {
			if b, err := json.Marshal(v); err == nil {
				d[k] = jx.Raw(b)
			}
		}
		out.Details = oas.NewOptAuditEntryDetails(d)
	}
	return out, true
}

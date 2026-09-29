package api

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/go-faster/jx"
	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/build"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/settings"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

func limitsOf(l settings.Limits) oas.TenantLimits {
	src := oas.TenantLimitsSourcePlatformDefault
	if l.Overridden {
		src = oas.TenantLimitsSourceTenantOverride
	}
	return oas.TenantLimits{MaxConcurrentRuns: l.MaxConcurrentRuns, MaxMachinesPerRun: l.MaxMachinesPerRun, MaxSize: oas.Size(l.MaxSize), MaxKeep: l.MaxKeep.String(), RunRetentionMaxDays: l.RunRetentionMaxDays, Source: oas.NewOptTenantLimitsSource(src)}
}

// limitsWriteOf turns a TenantLimitsWrite into limits; nil when every
// field is null (= back to defaults).
func limitsWriteOf(w oas.TenantLimitsWrite) (*settings.Limits, error) {
	l := settings.Limits{}
	set := false
	if v, ok := w.MaxConcurrentRuns.Get(); ok {
		l.MaxConcurrentRuns, set = v, true
	}
	if v, ok := w.MaxMachinesPerRun.Get(); ok {
		l.MaxMachinesPerRun, set = v, true
	}
	if v, ok := w.MaxSize.Get(); ok {
		l.MaxSize, set = string(v), true
	}
	if v, ok := w.MaxKeep.Get(); ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, errs.Invalid("max_keep is not a duration")
		}
		l.MaxKeep, set = d, true
	}
	if v, ok := w.RunRetentionMaxDays.Get(); ok {
		l.RunRetentionMaxDays, set = v, true
	}
	if !set {
		return nil, nil //nolint:nilnil // all-null = clear the override
	}
	return &l, nil
}

func (h *Handler) adminTenantOf(v admin.TenantView, full bool) *oas.AdminTenant {
	t := v.Tenant
	out := &oas.AdminTenant{
		ID: t.ID, Slug: t.Slug, Name: t.Name, Status: oas.AdminTenantStatus(t.Status), Owner: oas.UserRef{ID: t.OwnerID.String()},
		MemberCount: oas.NewOptInt(t.MemberCount), CreatedAt: t.CreatedAt, GrapheneNamespace: oas.NewOptString(t.GrapheneNamespace), LastActivityAt: optTime(v.LastActivityAt),
		Counters: oas.NewOptAdminTenantCounters(oas.AdminTenantCounters{Members: oas.NewOptInt(v.Counters.Members), RunsTotal: oas.NewOptInt(v.Counters.RunsTotal), RunsRunning: oas.NewOptInt(v.Counters.RunsRunning), KeptStands: oas.NewOptInt(v.Counters.KeptStands), Providers: oas.NewOptInt(v.Counters.Providers)}),
	}
	if t.Description != "" {
		out.Description = oas.NewOptString(t.Description)
	}
	if t.PublicName != nil {
		out.PublicName = oas.NewOptNilString(*t.PublicName)
	} else {
		out.PublicName = oas.OptNilString{Null: true, Set: true}
	}
	if v.Owner.DisplayName != "" {
		out.Owner.DisplayName = oas.NewOptString(v.Owner.DisplayName)
	}
	if v.SuspendedReason != "" {
		out.SuspendedReason = oas.NewOptString(v.SuspendedReason)
	}
	if full {
		out.Limits = oas.NewOptTenantLimits(limitsOf(v.Limits))
	}
	return out
}

func adminUserOf(u admin.UserView) oas.AdminUser {
	out := oas.AdminUser{ID: u.Profile.ID.String(), Email: u.Profile.Email, DisplayName: u.Profile.DisplayName, IsPlatformAdmin: u.Profile.IsPlatformAdmin || u.AdminSource == "config", Memberships: oas.NewOptInt(u.Memberships), CreatedAt: u.Profile.CreatedAt, LastSeenAt: oas.OptNilDateTime{Null: true, Set: true}}
	if u.LastActivityAt != nil {
		out.LastSeenAt = oas.NewOptNilDateTime(*u.LastActivityAt)
	}
	if u.AdminSource != "" {
		out.AdminSource = oas.NewOptAdminUserAdminSource(oas.AdminUserAdminSource(u.AdminSource))
	}
	if u.OwnedTenant != nil {
		out.OwnedTenant = oas.NewOptRef(oas.Ref{ID: u.OwnedTenant.ID, Name: oas.NewOptString(u.OwnedTenant.Name)})
	}
	return out
}

func adminActor(ctx context.Context) (authActor, error) { return actor(ctx) }

// AdminListTenants — every tenant.
func (h *Handler) AdminListTenants(ctx context.Context, params oas.AdminListTenantsParams) (*oas.AdminListTenantsOK, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	offset, err := cursorOffset(params.Cursor)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := admin.TenantQuery{
		Search: params.Search.Or(""), Sort: string(params.Sort.Or(oas.AdminListTenantsSortCreatedAt)),
		Desc: params.Order.Or(oas.OrderDesc) == oas.OrderDesc, Limit: limit + 1, Offset: offset,
	}
	if v, ok := params.Status.Get(); ok {
		q.Status = string(v)
	}
	list, err := h.deps.Admin.Tenants(ctx, a, q)
	if err != nil {
		return nil, err
	}
	list, meta := page(list, offset, limit)
	out := &oas.AdminListTenantsOK{Data: make([]oas.AdminTenant, 0, len(list)), Meta: meta}
	for _, v := range list {
		out.Data = append(out.Data, *h.adminTenantOf(v, false))
	}
	return out, nil
}

// AdminGetTenant — details.
func (h *Handler) AdminGetTenant(ctx context.Context, params oas.AdminGetTenantParams) (*oas.AdminTenant, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := h.deps.Admin.Tenant(ctx, a, params.Slug)
	if err != nil {
		return nil, err
	}
	return h.adminTenantOf(v, true), nil
}

// AdminSuspendTenant — block.
func (h *Handler) AdminSuspendTenant(ctx context.Context, req oas.OptAdminSuspendTenantReq, params oas.AdminSuspendTenantParams) (*oas.AdminTenant, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	reason := ""
	if r, ok := req.Get(); ok {
		reason = r.Reason.Or("")
	}
	v, err := h.deps.Admin.Suspend(ctx, a, params.Slug, reason)
	if err != nil {
		return nil, err
	}
	return h.adminTenantOf(v, true), nil
}

// AdminResumeTenant — unblock.
func (h *Handler) AdminResumeTenant(ctx context.Context, params oas.AdminResumeTenantParams) (*oas.AdminTenant, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := h.deps.Admin.Resume(ctx, a, params.Slug)
	if err != nil {
		return nil, err
	}
	return h.adminTenantOf(v, true), nil
}

// AdminSetTenantLimits — override (all-null clears).
func (h *Handler) AdminSetTenantLimits(ctx context.Context, req *oas.TenantLimitsWrite, params oas.AdminSetTenantLimitsParams) (*oas.TenantLimits, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	l, err := limitsWriteOf(*req)
	if err != nil {
		return nil, err
	}
	out, err := h.deps.Admin.SetLimits(ctx, a, params.Slug, l)
	if err != nil {
		return nil, err
	}
	lim := limitsOf(out)
	return &lim, nil
}

// AdminAssignOwner — new owner.
func (h *Handler) AdminAssignOwner(ctx context.Context, req *oas.AdminAssignOwnerReq, params oas.AdminAssignOwnerParams) (*oas.AdminTenant, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, errs.Invalid("user_id is not a uuid")
	}
	v, err := h.deps.Admin.AssignOwner(ctx, a, params.Slug, id)
	if err != nil {
		return nil, err
	}
	return h.adminTenantOf(v, true), nil
}

// AdminDeleteTenant — remove.
func (h *Handler) AdminDeleteTenant(ctx context.Context, params oas.AdminDeleteTenantParams) error {
	a, err := adminActor(ctx)
	if err != nil {
		return err
	}
	return h.deps.Admin.DeleteTenant(ctx, a, params.Slug)
}

// AdminListUsers — profiles.
func (h *Handler) AdminListUsers(ctx context.Context, params oas.AdminListUsersParams) (*oas.AdminListUsersOK, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	offset, err := cursorOffset(params.Cursor)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	list, err := h.deps.Admin.Users(ctx, a, admin.UserQuery{
		Search: params.Search.Or(""), OnlyAdmins: params.PlatformAdmin.Or(false), Sort: string(params.Sort.Or(oas.AdminListUsersSortCreatedAt)),
		Desc: params.Order.Or(oas.OrderDesc) == oas.OrderDesc, Limit: limit + 1, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	list, meta := page(list, offset, limit)
	out := &oas.AdminListUsersOK{Data: make([]oas.AdminUser, 0, len(list)), Meta: meta}
	for _, u := range list {
		out.Data = append(out.Data, adminUserOf(u))
	}
	return out, nil
}

// AdminPatchUser — toggle platform admin.
func (h *Handler) AdminPatchUser(ctx context.Context, req *oas.AdminPatchUserReq, params oas.AdminPatchUserParams) (*oas.AdminUser, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(params.UserId)
	if err != nil {
		return nil, errs.Invalid("user id is not a uuid")
	}
	on, ok := req.IsPlatformAdmin.Get()
	if !ok {
		return nil, errs.Invalid("is_platform_admin is required")
	}
	u, err := h.deps.Admin.SetPlatformAdmin(ctx, a, id, on)
	if err != nil {
		return nil, err
	}
	out := adminUserOf(u)
	return &out, nil
}

// AdminListRuns — the global queue.
func (h *Handler) AdminListRuns(ctx context.Context, params oas.AdminListRunsParams) (*oas.AdminListRunsOK, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	offset, err := cursorOffset(params.Cursor)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := admin.RunQuery{
		Tenant: params.Tenant.Or(""), Sort: string(params.Sort.Or(oas.AdminListRunsSortCreatedAt)),
		Desc: params.Order.Or(oas.OrderDesc) == oas.OrderDesc, Limit: limit + 1, Offset: offset,
	}
	for _, s := range params.Status {
		q.Statuses = append(q.Statuses, string(s))
	}
	list, err := h.deps.Admin.Runs(ctx, a, q)
	if err != nil {
		return nil, err
	}
	list, meta := page(list, offset, limit)
	out := &oas.AdminListRunsOK{Data: make([]oas.AdminListRunsOKDataItem, 0, len(list)), Meta: meta}
	for _, v := range list {
		raw, err := json.Marshal(h.runOf(v.Run, nil))
		if err != nil {
			return nil, err
		}
		var item oas.AdminListRunsOKDataItem
		if err := item.UnmarshalJSON(raw); err != nil {
			return nil, err
		}
		item.Tenant = oas.NewOptRef(oas.Ref{ID: v.Run.TenantID, Name: oas.NewOptString(v.TenantSlug)})
		out.Data = append(out.Data, item)
	}
	return out, nil
}

// AdminCancelRun — cancel any run.
func (h *Handler) AdminCancelRun(ctx context.Context, params oas.AdminCancelRunParams) (*oas.Run, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.deps.Admin.CancelRun(ctx, a, params.ID)
	if err != nil {
		return nil, err
	}
	return h.runOf(r, nil), nil
}

// AdminListAudit — the whole log.
func (h *Handler) AdminListAudit(ctx context.Context, params oas.AdminListAuditParams) (*oas.AuditPage, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := admin.AuditQuery{Tenant: params.Tenant.Or(""), Action: params.Action.Or(""), ActorID: params.Actor.Or(""), Limit: limit + 1}
	if c, ok := params.Cursor.Get(); ok && c != "" {
		id, err := strconv.ParseInt(c, 10, 64)
		if err != nil {
			return nil, errs.Invalid("cursor")
		}
		q.BeforeID = id
	}
	if s, ok := params.Since.Get(); ok {
		q.Since = &s
	}
	entries, err := h.deps.Admin.Audit(ctx, a, q)
	if err != nil {
		return nil, err
	}
	pg := &oas.AuditPage{Data: make([]oas.AuditEntry, 0, len(entries))}
	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}
	for _, e := range entries {
		pg.Data = append(pg.Data, auditOf(e))
	}
	pg.Meta.HasMore = oas.NewOptBool(hasMore)
	if hasMore {
		pg.Meta.NextCursor = oas.NewOptNilString(strconv.FormatInt(entries[len(entries)-1].ID, 10))
	}
	return pg, nil
}

func systemOf(s admin.SystemSettings, defaults settings.Limits) *oas.SystemSettings {
	out := &oas.SystemSettings{
		TenantCreation: oas.SystemSettingsTenantCreation(s.TenantCreation), PublicRatingEnabled: s.PublicRatingEnabled, ExamplesEnabled: s.ExamplesEnabled,
		DefaultLimits: limitsOf(defaults), RunRetentionMaxDays: s.RunRetentionMaxDays, UpdatedAt: oas.NewOptDateTime(s.UpdatedAt),
	}
	if len(s.StroppyCatalog) > 0 {
		out.StroppyCatalog = oas.NewOptSchemaValue(schemaValueOf(s.StroppyCatalog))
	}
	if s.UpdatedBy != nil {
		out.UpdatedBy = oas.NewOptUserRef(oas.UserRef{ID: s.UpdatedBy.String()})
	}
	return out
}

// GetSystemSettings — the platform row.
func (h *Handler) GetSystemSettings(ctx context.Context) (*oas.SystemSettings, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.deps.Admin.Require(ctx, a); err != nil {
		return nil, err
	}
	s, err := h.deps.Admin.System(ctx)
	if err != nil {
		return nil, err
	}
	return systemOf(s, h.deps.Admin.DefaultLimits(ctx)), nil
}

// PatchSystemSettings — change the platform row.
func (h *Handler) PatchSystemSettings(ctx context.Context, req *oas.SystemSettingsPatch) (*oas.SystemSettings, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	p := admin.SystemPatch{}
	if v, ok := req.TenantCreation.Get(); ok {
		s := string(v)
		p.TenantCreation = &s
	}
	if v, ok := req.PublicRatingEnabled.Get(); ok {
		p.PublicRatingEnabled = &v
	}
	if v, ok := req.ExamplesEnabled.Get(); ok {
		p.ExamplesEnabled = &v
	}
	if v, ok := req.DefaultLimits.Get(); ok {
		l, err := limitsWriteOf(v)
		if err != nil {
			return nil, err
		}
		if l != nil {
			def := h.deps.Admin.DefaultLimits(ctx)
			if l.MaxConcurrentRuns <= 0 {
				l.MaxConcurrentRuns = def.MaxConcurrentRuns
			}
			if l.MaxMachinesPerRun <= 0 {
				l.MaxMachinesPerRun = def.MaxMachinesPerRun
			}
			if l.MaxSize == "" {
				l.MaxSize = def.MaxSize
			}
			if l.MaxKeep <= 0 {
				l.MaxKeep = def.MaxKeep
			}
			if l.RunRetentionMaxDays <= 0 {
				l.RunRetentionMaxDays = def.RunRetentionMaxDays
			}
		}
		p.DefaultLimits, p.SetDefaultLimits = l, true
	}
	if v, ok := req.RunRetentionMaxDays.Get(); ok {
		p.RunRetentionMaxDays = &v
	}
	if v, ok := req.StroppyCatalog.Get(); ok {
		p.StroppyCatalog, p.SetCatalog = rawOf(v), true
	}
	s, err := h.deps.Admin.UpdateSystem(ctx, a, p)
	if err != nil {
		return nil, err
	}
	return systemOf(s, h.deps.Admin.DefaultLimits(ctx)), nil
}

// GetAdminStatus — the status page.
func (h *Handler) GetAdminStatus(ctx context.Context) (*oas.AdminStatus, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	st, err := h.deps.Admin.Status(ctx, a)
	if err != nil {
		return nil, err
	}
	out := &oas.AdminStatus{
		Version: build.Version, Commit: build.Commit, Components: oas.AdminStatusComponents{},
		Pipelines: oas.AdminStatusPipelines{ExpectedRevision: build.Version, Namespaces: make([]oas.AdminStatusPipelinesNamespacesItem, 0, len(st.Namespaces))},
		Runs:      oas.AdminStatusRuns{Running: oas.NewOptInt(st.Runs.Running), Pending: oas.NewOptInt(st.Runs.Pending), KeptStands: oas.NewOptInt(st.Runs.KeptStands)},
		Tenants:   oas.NewOptAdminStatusTenants(oas.AdminStatusTenants{Active: oas.NewOptInt(st.Tenants.Active), Suspended: oas.NewOptInt(st.Tenants.Suspended), Orphaned: oas.NewOptInt(st.Tenants.Orphaned)}),
	}
	if t, err := time.Parse(time.RFC3339, build.BuildTime); err == nil {
		out.StartedAt = oas.NewOptDateTime(t)
	}
	for name, c := range st.Components {
		item := oas.AdminStatusComponentsItem{Status: oas.AdminStatusComponentsItemStatus(c.Status)}
		if c.Detail != "" {
			item.Detail = oas.NewOptString(c.Detail)
		}
		if c.Version != "" {
			item.Version = oas.NewOptString(c.Version)
		}
		out.Components[name] = item
	}
	for _, ns := range st.Namespaces {
		item := oas.AdminStatusPipelinesNamespacesItem{Namespace: ns.Namespace, TenantSlug: oas.NewOptString(ns.TenantSlug), Status: oas.AdminStatusPipelinesNamespacesItemStatus(ns.Status), PushedAt: optTime(ns.PushedAt)}
		if ns.Revision != "" {
			item.Revision = oas.NewOptString(ns.Revision)
		}
		if ns.Error != "" {
			item.Error = oas.NewOptString(ns.Error)
		}
		out.Pipelines.Namespaces = append(out.Pipelines.Namespaces, item)
	}
	return out, nil
}

// ResyncPipelines — ask for a push.
func (h *Handler) ResyncPipelines(ctx context.Context, req oas.OptResyncPipelinesReq) (*oas.ResyncPipelinesAccepted, error) {
	a, err := adminActor(ctx)
	if err != nil {
		return nil, err
	}
	slug := ""
	if r, ok := req.Get(); ok {
		slug = r.TenantSlug.Or("")
	}
	namespaces, err := h.deps.Admin.Resync(ctx, a, slug)
	if err != nil {
		return nil, err
	}
	return &oas.ResyncPipelinesAccepted{Namespaces: namespaces}, nil
}

var _ = jx.Raw{}

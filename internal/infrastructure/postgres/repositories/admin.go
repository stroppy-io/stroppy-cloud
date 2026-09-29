package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/settings"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// AdminRepo is the platform-wide reads and the system settings row.
type AdminRepo struct{ q *db.Queries }

// NewAdminRepo builds the repository.
func NewAdminRepo(database tx.DB) *AdminRepo { return &AdminRepo{q: db.New(database)} }

func window(limit, offset int) (lim, off int64) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return int64(limit), int64(offset)
}

func (r *AdminRepo) Tenants(ctx context.Context, q admin.TenantQuery) ([]tenant.Tenant, error) {
	lim, off := window(q.Limit, q.Offset)
	rows, err := r.q.AdminTenants(ctx, db.AdminTenantsParams{Search: q.Search, Status: q.Status, SortKey: q.Sort, Desc: q.Desc, Lim: lim, Off: off})
	if err != nil {
		return nil, infraf("admin: tenants: %v", err)
	}
	out := make([]tenant.Tenant, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenantRow(db.TenantByIDRow(row)))
	}
	return out, nil
}

func (r *AdminRepo) TenantCounts(ctx context.Context) (admin.TenantCounts, error) {
	row, err := r.q.TenantStatusCounts(ctx)
	if err != nil {
		return admin.TenantCounts{}, infraf("admin: tenant counts: %v", err)
	}
	return admin.TenantCounts{Active: int(row.Active), Suspended: int(row.Suspended), Orphaned: int(row.Orphaned)}, nil
}

func (r *AdminRepo) SuspendedReason(ctx context.Context, tenantID uuid.UUID) (string, error) {
	row, err := r.q.TenantSuspendedReason(ctx, tenantID)
	if err != nil {
		return "", infraf("admin: suspended reason: %v", err)
	}
	return row.SuspendedReason, nil
}

func (r *AdminRepo) SetSuspended(ctx context.Context, tenantID uuid.UUID, status tenant.Status, reason string) error {
	n, err := r.q.SetTenantSuspended(ctx, db.SetTenantSuspendedParams{ID: tenantID, Status: string(status), Reason: reason})
	if err != nil {
		return infraf("admin: set status: %v", err)
	}
	if n == 0 {
		return errs.NotFound("tenant")
	}
	return nil
}

func (r *AdminRepo) Counters(ctx context.Context, tenantID uuid.UUID) (admin.Counters, *time.Time, error) {
	row, err := r.q.TenantCounters(ctx, tenantID)
	if err != nil {
		return admin.Counters{}, nil, infraf("admin: counters: %v", err)
	}
	return admin.Counters{Members: int(row.Members), RunsTotal: int(row.RunsTotal), RunsRunning: int(row.RunsRunning), KeptStands: int(row.KeptStands), Providers: int(row.Providers)}, row.LastActivityAt, nil
}

func (r *AdminRepo) Users(ctx context.Context, q admin.UserQuery) ([]admin.UserView, error) {
	lim, off := window(q.Limit, q.Offset)
	rows, err := r.q.AdminProfiles(ctx, db.AdminProfilesParams{Search: q.Search, OnlyAdmins: q.OnlyAdmins, SortKey: q.Sort, Desc: q.Desc, Lim: lim, Off: off})
	if err != nil {
		return nil, infraf("admin: users: %v", err)
	}
	out := make([]admin.UserView, 0, len(rows))
	for _, row := range rows {
		v := admin.UserView{Profile: profileRow(db.ProfileByIDRow{
			ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Avatar: row.Avatar, IsPlatformAdmin: row.IsPlatformAdmin,
			Preferences: row.Preferences, Notifications: row.Notifications, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}), LastActivityAt: row.LastActivityAt}
		v.Memberships = int(row.Memberships)
		if row.OwnedTenantID != nil {
			name := ""
			if row.OwnedTenantName != nil {
				name = *row.OwnedTenantName
			}
			v.OwnedTenant = &run.Ref{ID: *row.OwnedTenantID, Name: name}
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *AdminRepo) SetPlatformAdmin(ctx context.Context, userID uuid.UUID, on bool) error {
	n, err := r.q.SetPlatformAdmin(ctx, db.SetPlatformAdminParams{ID: userID, IsPlatformAdmin: on})
	if err != nil {
		return infraf("admin: set platform admin: %v", err)
	}
	if n == 0 {
		return errs.NotFound("user")
	}
	return nil
}

func (r *AdminRepo) Runs(ctx context.Context, q admin.RunQuery) ([]admin.RunView, error) {
	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.q.AdminRuns(ctx, db.AdminRunsParams{Statuses: q.Statuses, Tenant: q.Tenant, SortKey: q.Sort, Desc: q.Desc, Lim: int64(limit), Off: int64(q.Offset)})
	if err != nil {
		return nil, infraf("admin: runs: %v", err)
	}
	out := make([]admin.RunView, 0, len(rows))
	for _, row := range rows {
		base := db.RunByIDRow{
			ID: row.ID, TenantID: row.TenantID, Name: row.Name, Status: row.Status, Phase: row.Phase, StatusReason: row.StatusReason, Trigger: row.Trigger,
			SuiteRunID: row.SuiteRunID, CellID: row.CellID, ScheduleID: row.ScheduleID, ParentRunID: row.ParentRunID, TestID: row.TestID, TestName: row.TestName, AuthorID: row.AuthorID,
			Snapshot: row.Snapshot, RunSpec: row.RunSpec, Summary: row.Summary, Result: row.Result, RuntimeState: row.RuntimeState, LastEventID: row.LastEventID,
			RatingTenant: row.RatingTenant, RatingGlobal: row.RatingGlobal, Keep: row.Keep, KeepUntil: row.KeepUntil, StandKept: row.StandKept, Notes: row.Notes, Labels: row.Labels,
			GrapheneNamespace: row.GrapheneNamespace, GrapheneRunID: row.GrapheneRunID, PipelineRevision: row.PipelineRevision, Tps: row.Tps, DurationSeconds: row.DurationSeconds,
			CreatedAt: row.CreatedAt, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt,
		}
		out = append(out, admin.RunView{Run: runOf(base), TenantSlug: row.TenantSlug, TenantName: row.TenantName})
	}
	return out, nil
}

func (r *AdminRepo) RunCounts(ctx context.Context) (admin.RunCounts, error) {
	row, err := r.q.AdminRunCounts(ctx)
	if err != nil {
		return admin.RunCounts{}, infraf("admin: run counts: %v", err)
	}
	return admin.RunCounts{Running: int(row.Running), Pending: int(row.Pending), KeptStands: int(row.KeptStands)}, nil
}

func (r *AdminRepo) Audit(ctx context.Context, q admin.AuditQuery) ([]audit.Entry, error) {
	limit := int64(q.Limit)
	if limit <= 0 || limit > 201 {
		limit = 51
	}
	rows, err := r.q.AuditAll(ctx, db.AuditAllParams{BeforeID: q.BeforeID, Tenant: q.Tenant, Action: q.Action, ActorID: q.ActorID, Since: q.Since, Lim: limit})
	if err != nil {
		return nil, infraf("admin: audit: %v", err)
	}
	out := make([]audit.Entry, 0, len(rows))
	for _, row := range rows {
		e := audit.Entry{
			ID: row.ID, At: row.At, TenantID: row.TenantID, ActorKind: audit.ActorKind(row.ActorKind), ActorID: row.ActorID, ActorName: row.ActorName, Action: row.Action,
			Target: audit.Target{Kind: row.TargetKind, ID: row.TargetID, Name: row.TargetName}, RequestID: row.RequestID,
		}
		_ = json.Unmarshal(row.Details, &e.Details) //nolint:errcheck // stored by us
		out = append(out, e)
	}
	return out, nil
}

func (r *AdminRepo) System(ctx context.Context) (admin.SystemSettings, error) {
	row, err := r.q.SystemSettingsRow(ctx)
	if err != nil {
		return admin.SystemSettings{}, infraf("admin: system settings: %v", err)
	}
	return systemOf(row), nil
}

func (r *AdminRepo) UpdateSystem(ctx context.Context, p admin.SystemPatch, by *uuid.UUID) (admin.SystemSettings, error) {
	if _, err := r.q.SystemSettingsRow(ctx); err != nil {
		return admin.SystemSettings{}, infraf("admin: system settings: %v", err)
	}
	params := db.UpdateSystemSettingsParams{TenantCreation: p.TenantCreation, PublicRatingEnabled: p.PublicRatingEnabled, ExamplesEnabled: p.ExamplesEnabled, UpdatedBy: by}
	if p.SetDefaultLimits {
		params.SetDefaultLimits = true
		if p.DefaultLimits != nil {
			params.DefaultLimits, _ = json.Marshal(p.DefaultLimits) //nolint:errcheck // struct
		}
	}
	if p.RunRetentionMaxDays != nil {
		d := int32(*p.RunRetentionMaxDays) //nolint:gosec // small
		params.RunRetentionMaxDays = &d
	}
	if p.SetCatalog {
		params.SetCatalog, params.StroppyCatalog = true, p.StroppyCatalog
	}
	row, err := r.q.UpdateSystemSettings(ctx, params)
	if err != nil {
		return admin.SystemSettings{}, infraf("admin: update system settings: %v", err)
	}
	return systemOf(db.SystemSettingsRowRow(row)), nil
}

func systemOf(row db.SystemSettingsRowRow) admin.SystemSettings {
	s := admin.SystemSettings{TenantCreation: row.TenantCreation, PublicRatingEnabled: row.PublicRatingEnabled, ExamplesEnabled: row.ExamplesEnabled, RunRetentionMaxDays: int(row.RunRetentionMaxDays), UpdatedAt: row.UpdatedAt, UpdatedBy: row.UpdatedBy}
	if len(row.DefaultLimits) > 0 && string(row.DefaultLimits) != "null" {
		var l settings.Limits
		if json.Unmarshal(row.DefaultLimits, &l) == nil {
			s.DefaultLimits = &l
		}
	}
	if len(row.StroppyCatalog) > 0 && string(row.StroppyCatalog) != "null" {
		s.StroppyCatalog = row.StroppyCatalog
	}
	return s
}

package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/gopherex/pgtx/pkg/tx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/settings"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// SettingsRepo stores tenant settings.
type SettingsRepo struct {
	q *db.Queries
}

var _ settings.Repository = (*SettingsRepo)(nil)

// NewSettingsRepo builds the repo.
func NewSettingsRepo(database tx.DB) *SettingsRepo { return &SettingsRepo{q: db.New(database)} }

func (r *SettingsRepo) Ensure(ctx context.Context, tenantID uuid.UUID) (settings.Settings, *settings.Limits, error) {
	row, err := r.q.EnsureSettings(ctx, tenantID)
	if err != nil {
		return settings.Settings{}, nil, infraf("settings: ensure: %v", err)
	}
	st := settingsRow(db.UpdateSettingsRow(row))
	var override *settings.Limits
	if len(row.LimitsOverride) > 0 && string(row.LimitsOverride) != "null" {
		var l settings.Limits
		if err := json.Unmarshal(row.LimitsOverride, &l); err == nil {
			override = &l
		}
	}
	return st, override, nil
}

func (r *SettingsRepo) Update(ctx context.Context, tenantID uuid.UUID, p settings.Patch) (settings.Settings, error) {
	params := db.UpdateSettingsParams{TenantID: tenantID, RatingTenant: p.RatingTenant, RatingGlobal: p.RatingGlobal}
	if p.RunRetentionDays != nil {
		v := int32(*p.RunRetentionDays) //nolint:gosec // bounded by the service
		params.RunRetentionDays = &v
	}
	if p.DefaultKeep != nil {
		v := p.DefaultKeep.String()
		params.DefaultKeep = &v
	}
	if p.NotificationEmails != nil {
		params.NotificationEmails, _ = json.Marshal(p.NotificationEmails) //nolint:errcheck // []string always marshals
	}
	row, err := r.q.UpdateSettings(ctx, params)
	if err != nil {
		return settings.Settings{}, infraf("settings: update: %v", err)
	}
	return settingsRow(row), nil
}

func (r *SettingsRepo) SetLimitsOverride(ctx context.Context, tenantID uuid.UUID, l *settings.Limits) error {
	var raw json.RawMessage
	if l != nil {
		raw, _ = json.Marshal(l) //nolint:errcheck // struct always marshals
	}
	// A tenant nobody has opened the settings of has no row yet: the
	// override must not vanish into an update of nothing.
	if _, err := r.q.EnsureSettings(ctx, tenantID); err != nil {
		return infraf("settings: ensure: %v", err)
	}
	n, err := r.q.SetLimitsOverride(ctx, db.SetLimitsOverrideParams{TenantID: tenantID, LimitsOverride: raw})
	if err != nil {
		return infraf("settings: limits: %v", err)
	}
	if n != 1 {
		return infraf("settings: limits: %d rows updated", n)
	}
	return nil
}

func settingsRow(row db.UpdateSettingsRow) settings.Settings {
	st := settings.Settings{
		TenantID: row.TenantID, RunRetentionDays: int(row.RunRetentionDays), RatingTenant: row.RatingTenant,
		RatingGlobal: row.RatingGlobal, UpdatedAt: row.UpdatedAt, NotificationEmails: []string{},
	}
	st.DefaultKeep, _ = time.ParseDuration(row.DefaultKeep)            //nolint:errcheck // stored by us
	_ = json.Unmarshal(row.NotificationEmails, &st.NotificationEmails) //nolint:errcheck // stored by us
	return st
}

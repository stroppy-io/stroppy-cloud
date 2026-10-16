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

func (r *SettingsRepo) Ensure(ctx context.Context, tenantID uuid.UUID) (settings.Settings, error) {
	row, err := r.q.EnsureSettings(ctx, tenantID)
	if err != nil {
		return settings.Settings{}, infraf("settings: ensure: %v", err)
	}
	return settingsRow(db.UpdateSettingsRow(row)), nil
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

func settingsRow(row db.UpdateSettingsRow) settings.Settings {
	st := settings.Settings{
		TenantID: row.TenantID, RunRetentionDays: int(row.RunRetentionDays), RatingTenant: row.RatingTenant,
		RatingGlobal: row.RatingGlobal, UpdatedAt: row.UpdatedAt, NotificationEmails: []string{},
	}
	st.DefaultKeep, _ = time.ParseDuration(row.DefaultKeep)            //nolint:errcheck // stored by us
	_ = json.Unmarshal(row.NotificationEmails, &st.NotificationEmails) //nolint:errcheck // stored by us
	return st
}

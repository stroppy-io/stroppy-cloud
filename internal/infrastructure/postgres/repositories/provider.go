package repositories

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gopherex/pgtx/pkg/tx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// ProviderRepo stores provider profiles.
type ProviderRepo struct {
	q *db.Queries
}

var _ provider.Repository = (*ProviderRepo)(nil)

// NewProviderRepo builds the repo.
func NewProviderRepo(database tx.DB) *ProviderRepo { return &ProviderRepo{q: db.New(database)} }

func (r *ProviderRepo) Insert(ctx context.Context, p provider.Profile) error {
	names, _ := json.Marshal(p.SecretNames) //nolint:errcheck // []string always marshals
	settings := p.Settings
	if len(settings) == 0 {
		settings = json.RawMessage("{}")
	}
	err := r.q.InsertProviderProfile(ctx, db.InsertProviderProfileParams{
		ID: p.ID, TenantID: p.TenantID, Name: p.Name, Kind: string(p.Kind), Settings: settings,
		SecretNames: names, Status: string(p.Status), CreatedBy: p.CreatedBy,
	})
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a profile with this name exists")
		}
		return infraf("provider: insert: %v", err)
	}
	return nil
}

func (r *ProviderRepo) ByID(ctx context.Context, id uuid.UUID) (provider.Profile, error) {
	row, err := r.q.ProviderProfileByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return provider.Profile{}, errs.NotFound("provider profile")
		}
		return provider.Profile{}, infraf("provider: by id: %v", err)
	}
	return profileOf(row), nil
}

func (r *ProviderRepo) OfTenant(ctx context.Context, tenantID uuid.UUID) ([]provider.Profile, error) {
	rows, err := r.q.ProviderProfilesOfTenant(ctx, tenantID)
	if err != nil {
		return nil, infraf("provider: of tenant: %v", err)
	}
	out := make([]provider.Profile, 0, len(rows))
	for _, row := range rows {
		out = append(out, profileOf(db.ProviderProfileByIDRow(row)))
	}
	return out, nil
}

func (r *ProviderRepo) Ready(ctx context.Context) ([]provider.Profile, error) {
	rows, err := r.q.ReadyProviderProfiles(ctx)
	if err != nil {
		return nil, infraf("provider: ready: %v", err)
	}
	out := make([]provider.Profile, 0, len(rows))
	for _, row := range rows {
		out = append(out, profileOf(db.ProviderProfileByIDRow(row)))
	}
	return out, nil
}

func (r *ProviderRepo) Update(ctx context.Context, id uuid.UUID, name *string, settings json.RawMessage) error {
	n, err := r.q.UpdateProviderProfile(ctx, db.UpdateProviderProfileParams{ID: id, Name: name, Settings: settings})
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a profile with this name exists")
		}
		return infraf("provider: update: %v", err)
	}
	if n == 0 {
		return errs.NotFound("provider profile")
	}
	return nil
}

func (r *ProviderRepo) SetStatus(ctx context.Context, id uuid.UUID, status provider.Status, reason, runID string) error {
	st := string(status)
	if _, err := r.q.SetProviderStatus(ctx, db.SetProviderStatusParams{ID: id, Status: &st, StatusReason: reason, VerifyRunID: runID}); err != nil {
		return infraf("provider: set status: %v", err)
	}
	return nil
}

func (r *ProviderRepo) SetQuotas(ctx context.Context, id uuid.UUID, result provider.QuotasResult) error {
	quotas := result.Quotas
	if quotas == nil || result.UnavailableReason != "" {
		quotas = []provider.Quota{}
	}
	raw, _ := json.Marshal(quotas) //nolint:errcheck // always marshals
	if _, err := r.q.SetProviderQuotas(ctx, db.SetProviderQuotasParams{ID: id, Quotas: raw, ObservedAt: &result.ObservedAt, UnavailableReason: result.UnavailableReason, Scope: result.Scope}); err != nil {
		return infraf("provider: set quotas: %v", err)
	}
	return nil
}

func (r *ProviderRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.SoftDeleteProviderProfile(ctx, id)
	if err != nil {
		return infraf("provider: delete: %v", err)
	}
	if n == 0 {
		return errs.NotFound("provider profile")
	}
	return nil
}

func profileOf(row db.ProviderProfileByIDRow) provider.Profile {
	p := provider.Profile{
		ID: row.ID, TenantID: row.TenantID, Name: row.Name, Kind: provider.Kind(row.Kind), Settings: row.Settings,
		Status: provider.Status(row.Status), StatusReason: row.StatusReason, VerifiedAt: row.VerifiedAt,
		QuotasUnavailableReason: row.QuotasUnavailableReason, QuotasScope: row.QuotasScope,
		VerifyRunID: row.VerifyRunID, QuotasObservedAt: row.QuotasObservedAt, CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, SecretNames: []string{},
	}
	_ = json.Unmarshal(row.SecretNames, &p.SecretNames) //nolint:errcheck // stored by us
	if len(row.Quotas) > 0 {
		_ = json.Unmarshal(row.Quotas, &p.Quotas) //nolint:errcheck // stored by us
	}
	return p
}

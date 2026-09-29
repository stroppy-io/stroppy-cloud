// Package repositories adapts the generated sqld queries to the domain
// ports. Every repo is built over the ctx-aware executor, so calls inside
// Tx.Do* share the transaction.
package repositories

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gopherex/pgtx/pkg/tx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/profile"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// ProfileRepo stores profiles.
type ProfileRepo struct {
	q *db.Queries
}

var _ profile.Repository = (*ProfileRepo)(nil)

// NewProfileRepo builds the repo.
func NewProfileRepo(database tx.DB) *ProfileRepo { return &ProfileRepo{q: db.New(database)} }

func (r *ProfileRepo) Ensure(ctx context.Context, id uuid.UUID, email string) (profile.Profile, bool, error) {
	row, err := r.q.EnsureProfile(ctx, db.EnsureProfileParams{ID: id, Email: email})
	if err != nil {
		return profile.Profile{}, false, infraf("profile: ensure: %v", err)
	}
	return profileRow(db.ProfileByIDRow{
		ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Avatar: row.Avatar, IsPlatformAdmin: row.IsPlatformAdmin,
		Preferences: row.Preferences, Notifications: row.Notifications, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), row.Created != nil && *row.Created, nil
}

func (r *ProfileRepo) Get(ctx context.Context, id uuid.UUID) (profile.Profile, error) {
	row, err := r.q.ProfileByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return profile.Profile{}, errs.NotFound("profile")
		}
		return profile.Profile{}, infraf("profile: get: %v", err)
	}
	return profileRow(row), nil
}

func (r *ProfileRepo) Update(ctx context.Context, p profile.Profile) (profile.Profile, error) {
	row, err := r.q.UpdateProfile(ctx, db.UpdateProfileParams{
		ID:            p.ID,
		DisplayName:   &p.DisplayName,
		Avatar:        &p.Avatar,
		Preferences:   profile.EncodeJSON(p.Preferences),
		Notifications: profile.EncodeJSON(p.Notifications),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return profile.Profile{}, errs.NotFound("profile")
		}
		return profile.Profile{}, infraf("profile: update: %v", err)
	}
	return profileRow(db.ProfileByIDRow(row)), nil
}

func (r *ProfileRepo) SetEmail(ctx context.Context, id uuid.UUID, email string) error {
	if _, err := r.q.SetProfileEmail(ctx, db.SetProfileEmailParams{ID: id, Email: email}); err != nil {
		return infraf("profile: set email: %v", err)
	}
	return nil
}

func (r *ProfileRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.q.DeleteProfile(ctx, id); err != nil {
		return infraf("profile: delete: %v", err)
	}
	return nil
}

// Refs loads display data for a batch of ids, the email scoped to the
// viewer in the query itself. An empty batch matches nothing, so it
// skips the round trip.
func (r *ProfileRepo) Refs(ctx context.Context, viewer profile.Viewer, ids []uuid.UUID) ([]profile.Ref, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.q.ProfileRefs(ctx, db.ProfileRefsParams{Reveal: viewer.Reveal, Viewer: viewer.UserID, ScopeTenant: viewer.Tenant, Ids: ids})
	if err != nil {
		return nil, infraf("profile: refs: %v", err)
	}
	out := make([]profile.Ref, 0, len(rows))
	for _, row := range rows {
		out = append(out, profile.Ref{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Avatar: row.Avatar})
	}
	return out, nil
}

// profileRow maps the SELECT list; every profile query returns the same
// columns, so the other row types convert to ProfileByIDRow.
func profileRow(row db.ProfileByIDRow) profile.Profile {
	p := profile.Profile{
		ID:              row.ID,
		Email:           row.Email,
		DisplayName:     row.DisplayName,
		Avatar:          row.Avatar,
		IsPlatformAdmin: row.IsPlatformAdmin,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	_ = json.Unmarshal(row.Preferences, &p.Preferences)     //nolint:errcheck // stored by us; malformed = defaults
	_ = json.Unmarshal(row.Notifications, &p.Notifications) //nolint:errcheck // same
	return p
}

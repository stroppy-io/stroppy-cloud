package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/jackc/pgx/v5"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/share"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// ShareRepo stores public shares.
type ShareRepo struct{ q *db.Queries }

// NewShareRepo builds the repository.
func NewShareRepo(database tx.DB) *ShareRepo { return &ShareRepo{q: db.New(database)} }

func (r *ShareRepo) Insert(ctx context.Context, s share.Share) error {
	snap, _ := json.Marshal(s.Snapshot) //nolint:errcheck // struct
	ids, _ := json.Marshal(s.RunIDs)    //nolint:errcheck // slice
	if len(s.RunIDs) == 0 {
		ids = []byte("[]")
	}
	err := r.q.InsertShare(ctx, db.InsertShareParams{
		ID: s.ID, TenantID: s.TenantID, Token: s.Token, TargetKind: string(s.Kind), TargetID: s.TargetID, TargetName: s.TargetName, RunIds: ids,
		Scope: string(s.Scope), Title: s.Title, Snapshot: snap, CapturedAt: s.CapturedAt, ExpiresAt: s.ExpiresAt, CreatedBy: s.CreatedBy,
	})
	if err != nil {
		return infraf("share: insert: %v", err)
	}
	return nil
}

func (r *ShareRepo) ByID(ctx context.Context, id uuid.UUID) (share.Share, error) {
	row, err := r.q.ShareByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return share.Share{}, errs.NotFound("share")
		}
		return share.Share{}, infraf("share: by id: %v", err)
	}
	return shareOf(row), nil
}

func (r *ShareRepo) ByToken(ctx context.Context, token string) (share.Share, error) {
	row, err := r.q.ShareByToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return share.Share{}, errs.NotFound("share")
		}
		return share.Share{}, infraf("share: by token: %v", err)
	}
	return shareOf(db.ShareByIDRow(row)), nil
}

func (r *ShareRepo) List(ctx context.Context, tenantID uuid.UUID, q share.ListQuery) ([]share.Share, error) {
	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	onlyActive, onlyInactive := false, false
	if q.Active != nil {
		onlyActive, onlyInactive = *q.Active, !*q.Active
	}
	rows, err := r.q.SharesOfTenant(ctx, db.SharesOfTenantParams{TenantID: tenantID, TargetKind: q.Kind, TargetID: q.TargetID, OnlyActive: onlyActive, OnlyInactive: onlyInactive, SortKey: q.Sort, Desc: q.Desc, Lim: int64(limit), Off: int64(q.Offset)})
	if err != nil {
		return nil, infraf("share: list: %v", err)
	}
	out := make([]share.Share, 0, len(rows))
	for _, row := range rows {
		out = append(out, shareOf(db.ShareByIDRow(row)))
	}
	return out, nil
}

func (r *ShareRepo) OfTarget(ctx context.Context, kind string, id uuid.UUID) ([]run.Ref, error) {
	rows, err := r.q.SharesOfTarget(ctx, db.SharesOfTargetParams{TargetKind: kind, TargetID: &id})
	if err != nil {
		return nil, infraf("share: of target: %v", err)
	}
	out := make([]run.Ref, 0, len(rows))
	for _, row := range rows {
		out = append(out, run.Ref{ID: row.ID, Name: row.Title})
	}
	return out, nil
}

func (r *ShareRepo) Update(ctx context.Context, id uuid.UUID, p share.Patch, expiresAt *time.Time) error {
	params := db.UpdateShareParams{ID: id}
	if p.Scope != nil {
		sc := string(*p.Scope)
		params.Scope = &sc
	}
	if p.Title != nil {
		params.Title = p.Title
	}
	if p.TTL != nil || p.ClearTTL {
		params.SetExpires, params.ExpiresAt = true, expiresAt
	}
	if err := r.q.UpdateShare(ctx, params); err != nil {
		return infraf("share: update: %v", err)
	}
	return nil
}

func (r *ShareRepo) SetSnapshot(ctx context.Context, id uuid.UUID, snap share.Snapshot, capturedAt time.Time, title string) error {
	raw, _ := json.Marshal(snap) //nolint:errcheck // struct
	if err := r.q.SetShareSnapshot(ctx, db.SetShareSnapshotParams{ID: id, Snapshot: raw, CapturedAt: capturedAt, Title: title}); err != nil {
		return infraf("share: set snapshot: %v", err)
	}
	return nil
}

func (r *ShareRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.RevokeShare(ctx, id)
	if err != nil {
		return infraf("share: revoke: %v", err)
	}
	if n == 0 {
		return errs.Conflict("share already revoked")
	}
	return nil
}

func (r *ShareRepo) BumpViews(ctx context.Context, id uuid.UUID) error {
	return r.q.BumpShareViews(ctx, id)
}

func shareOf(row db.ShareByIDRow) share.Share {
	s := share.Share{
		ID: row.ID, TenantID: row.TenantID, Token: row.Token, Kind: share.Kind(row.TargetKind), TargetID: row.TargetID, TargetName: row.TargetName,
		Scope: share.Scope(row.Scope), Title: row.Title, CapturedAt: row.CapturedAt, ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt, ViewCount: int(row.ViewCount),
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	_ = json.Unmarshal(row.RunIds, &s.RunIDs)     //nolint:errcheck // stored by us
	_ = json.Unmarshal(row.Snapshot, &s.Snapshot) //nolint:errcheck // stored by us
	return s
}

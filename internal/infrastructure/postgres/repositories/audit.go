package repositories

import (
	"context"
	"encoding/json"

	"github.com/gopherex/pgtx/pkg/tx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// AuditRepo stores the audit log.
type AuditRepo struct {
	q *db.Queries
}

var _ audit.Repository = (*AuditRepo)(nil)

// NewAuditRepo builds the repo.
func NewAuditRepo(database tx.DB) *AuditRepo { return &AuditRepo{q: db.New(database)} }

func (r *AuditRepo) Insert(ctx context.Context, e audit.Entry) error {
	details := json.RawMessage("{}")
	if e.Details != nil {
		if b, err := json.Marshal(e.Details); err == nil {
			details = b
		}
	}
	err := r.q.InsertAudit(ctx, db.InsertAuditParams{
		TenantID: e.TenantID, ActorKind: string(e.ActorKind), ActorID: e.ActorID, ActorName: e.ActorName,
		Action: e.Action, TargetKind: e.Target.Kind, TargetID: e.Target.ID, TargetName: e.Target.Name,
		Details: details, RequestID: e.RequestID,
	})
	if err != nil {
		return infraf("audit: insert: %v", err)
	}
	return nil
}

func (r *AuditRepo) OfTenant(ctx context.Context, q audit.Query) ([]audit.Entry, error) {
	tenantID := q.TenantID
	before := q.BeforeID
	limit := int64(q.Limit)
	rows, err := r.q.AuditOfTenant(ctx, db.AuditOfTenantParams{
		TenantID: &tenantID, BeforeID: before, Action: q.Action, ActorID: q.ActorID, Since: q.Since, Lim: limit,
	})
	if err != nil {
		return nil, infraf("audit: of tenant: %v", err)
	}
	out := make([]audit.Entry, 0, len(rows))
	for _, row := range rows {
		e := audit.Entry{
			ID: row.ID, At: row.At, TenantID: row.TenantID, ActorKind: audit.ActorKind(row.ActorKind),
			ActorID: row.ActorID, ActorName: row.ActorName, Action: row.Action,
			Target: audit.Target{Kind: row.TargetKind, ID: row.TargetID, Name: row.TargetName}, RequestID: row.RequestID,
		}
		_ = json.Unmarshal(row.Details, &e.Details) //nolint:errcheck // stored by us
		out = append(out, e)
	}
	return out, nil
}

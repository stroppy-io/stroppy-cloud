package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gopherex/pgtx/pkg/tx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/webhook"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// WebhookRepo stores webhooks and deliveries.
type WebhookRepo struct {
	q *db.Queries
}

var _ webhook.Repository = (*WebhookRepo)(nil)

// NewWebhookRepo builds the repo.
func NewWebhookRepo(database tx.DB) *WebhookRepo { return &WebhookRepo{q: db.New(database)} }

func (r *WebhookRepo) Insert(ctx context.Context, w webhook.Webhook) error {
	events, _ := json.Marshal(w.Events) //nolint:errcheck // []string always marshals
	err := r.q.InsertWebhook(ctx, db.InsertWebhookParams{
		ID: w.ID, TenantID: w.TenantID, URL: w.URL, Events: events, Enabled: w.Enabled, Description: w.Description, Secret: w.Secret,
	})
	if err != nil {
		return infraf("webhook: insert: %v", err)
	}
	return nil
}

func (r *WebhookRepo) ByID(ctx context.Context, id uuid.UUID) (webhook.Webhook, error) {
	row, err := r.q.WebhookByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return webhook.Webhook{}, errs.NotFound("webhook")
		}
		return webhook.Webhook{}, infraf("webhook: by id: %v", err)
	}
	return webhookOf(row), nil
}

func (r *WebhookRepo) OfTenant(ctx context.Context, tenantID uuid.UUID, enabledOnly bool) ([]webhook.Webhook, error) {
	var rows []db.WebhookByIDRow
	if enabledOnly {
		rs, err := r.q.EnabledWebhooksOfTenant(ctx, tenantID)
		if err != nil {
			return nil, infraf("webhook: of tenant: %v", err)
		}
		for _, row := range rs {
			rows = append(rows, db.WebhookByIDRow(row))
		}
	} else {
		rs, err := r.q.WebhooksOfTenant(ctx, tenantID)
		if err != nil {
			return nil, infraf("webhook: of tenant: %v", err)
		}
		for _, row := range rs {
			rows = append(rows, db.WebhookByIDRow(row))
		}
	}
	out := make([]webhook.Webhook, 0, len(rows))
	for _, row := range rows {
		out = append(out, webhookOf(row))
	}
	return out, nil
}

func (r *WebhookRepo) Update(ctx context.Context, id uuid.UUID, p webhook.Patch) error {
	params := db.UpdateWebhookParams{ID: id, URL: p.URL, Enabled: p.Enabled, Description: p.Description}
	if p.Events != nil {
		params.Events, _ = json.Marshal(p.Events) //nolint:errcheck // []string always marshals
	}
	n, err := r.q.UpdateWebhook(ctx, params)
	if err != nil {
		return infraf("webhook: update: %v", err)
	}
	if n == 0 {
		return errs.NotFound("webhook")
	}
	return nil
}

func (r *WebhookRepo) Rotate(ctx context.Context, id uuid.UUID, secret string, prevExpires time.Time) error {
	if _, err := r.q.RotateWebhookSecret(ctx, db.RotateWebhookSecretParams{ID: id, Secret: secret, PrevExpiresAt: &prevExpires}); err != nil {
		return infraf("webhook: rotate: %v", err)
	}
	return nil
}

func (r *WebhookRepo) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.DeleteWebhook(ctx, id)
	if err != nil {
		return infraf("webhook: delete: %v", err)
	}
	if n == 0 {
		return errs.NotFound("webhook")
	}
	return nil
}

func (r *WebhookRepo) LastDelivery(ctx context.Context, id uuid.UUID) (*webhook.LastDelivery, error) {
	row, err := r.q.LastDeliveryOfWebhook(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // no delivery yet is a legal "none"
		}
		return nil, infraf("webhook: last delivery: %v", err)
	}
	return &webhook.LastDelivery{At: row.At, Status: webhook.DeliveryStatus(row.Status)}, nil
}

func (r *WebhookRepo) InsertDelivery(ctx context.Context, d webhook.Delivery) error {
	if err := r.q.InsertDelivery(ctx, db.InsertDeliveryParams{ID: d.ID, WebhookID: d.WebhookID, Event: d.Event, Payload: d.Payload}); err != nil {
		return infraf("delivery: insert: %v", err)
	}
	return nil
}

func (r *WebhookRepo) DeliveryByID(ctx context.Context, id uuid.UUID) (webhook.Delivery, error) {
	row, err := r.q.DeliveryByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return webhook.Delivery{}, errs.NotFound("delivery")
		}
		return webhook.Delivery{}, infraf("delivery: by id: %v", err)
	}
	return deliveryOf(row), nil
}

func (r *WebhookRepo) Deliveries(ctx context.Context, webhookID uuid.UUID, before *time.Time, limit int) ([]webhook.Delivery, error) {
	lim := int64(limit)
	rows, err := r.q.DeliveriesOfWebhook(ctx, db.DeliveriesOfWebhookParams{WebhookID: webhookID, Before: before, Lim: lim})
	if err != nil {
		return nil, infraf("delivery: list: %v", err)
	}
	out := make([]webhook.Delivery, 0, len(rows))
	for _, row := range rows {
		out = append(out, deliveryOf(db.DeliveryByIDRow(row)))
	}
	return out, nil
}

func (r *WebhookRepo) Due(ctx context.Context, lease time.Duration, limit int) ([]webhook.Delivery, error) {
	lim := int64(limit)
	rows, err := r.q.DueDeliveries(ctx, db.DueDeliveriesParams{
		Lease: pgtype.Interval{Microseconds: lease.Microseconds(), Valid: true}, Lim: lim,
	})
	if err != nil {
		return nil, infraf("delivery: due: %v", err)
	}
	out := make([]webhook.Delivery, 0, len(rows))
	for _, row := range rows {
		out = append(out, deliveryOf(db.DeliveryByIDRow(row)))
	}
	return out, nil
}

func (r *WebhookRepo) Finish(ctx context.Context, id uuid.UUID, status webhook.DeliveryStatus, next time.Time, responseStatus *int, errText string) error {
	var rs *int32
	if responseStatus != nil {
		v := int32(*responseStatus) //nolint:gosec // HTTP status
		rs = &v
	}
	if err := r.q.FinishDelivery(ctx, db.FinishDeliveryParams{ID: id, Status: string(status), NextAttemptAt: next, ResponseStatus: rs, Error: errText}); err != nil {
		return infraf("delivery: finish: %v", err)
	}
	return nil
}

func (r *WebhookRepo) Reset(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.ResetDelivery(ctx, id)
	if err != nil {
		return infraf("delivery: reset: %v", err)
	}
	if n == 0 {
		return errs.NotFound("delivery")
	}
	return nil
}

func webhookOf(row db.WebhookByIDRow) webhook.Webhook {
	w := webhook.Webhook{
		ID: row.ID, TenantID: row.TenantID, URL: row.URL, Enabled: row.Enabled, Description: row.Description,
		Secret: row.Secret, PrevSecret: row.PrevSecret, PrevExpiresAt: row.PrevExpiresAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Events: []string{},
	}
	_ = json.Unmarshal(row.Events, &w.Events) //nolint:errcheck // stored by us
	return w
}

func deliveryOf(row db.DeliveryByIDRow) webhook.Delivery {
	d := webhook.Delivery{
		ID: row.ID, WebhookID: row.WebhookID, Event: row.Event, Status: webhook.DeliveryStatus(row.Status),
		Attempts: int(row.Attempts), NextAttemptAt: row.NextAttemptAt, LastAttemptAt: row.LastAttemptAt,
		Error: row.Error, Payload: row.Payload, CreatedAt: row.CreatedAt,
	}
	if row.ResponseStatus != nil {
		v := int(*row.ResponseStatus)
		d.ResponseStatus = &v
	}
	return d
}

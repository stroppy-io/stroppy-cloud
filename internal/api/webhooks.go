package api

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/go-faster/jx"
	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/webhook"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// ListWebhooks — outbound webhooks.
func (h *Handler) ListWebhooks(ctx context.Context, params oas.ListWebhooksParams) (*oas.ListWebhooksOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	ws, err := h.deps.Webhooks.List(ctx, a, t.ID)
	if err != nil {
		return nil, err
	}
	out := &oas.ListWebhooksOK{Data: make([]oas.Webhook, 0, len(ws))}
	for _, w := range ws {
		last, _ := h.deps.Webhooks.LastDelivery(ctx, w.ID) //nolint:errcheck // summary is best-effort
		out.Data = append(out.Data, webhookOf(w, last))
	}
	return out, nil
}

// CreateWebhook — the secret is returned once.
func (h *Handler) CreateWebhook(ctx context.Context, req *oas.WebhookCreate, params oas.CreateWebhookParams) (*oas.WebhookCreated, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	c := webhook.Create{URL: req.URL.String(), Events: eventsOf(req.Events), Description: req.Description.Or("")}
	if v, ok := req.Enabled.Get(); ok {
		c.Enabled = &v
	}
	w, err := h.deps.Webhooks.Create(ctx, a, t.ID, c)
	if err != nil {
		return nil, err
	}
	return createdWebhookOf(w), nil
}

// PatchWebhook — url/events/enabled/description.
func (h *Handler) PatchWebhook(ctx context.Context, req *oas.WebhookPatch, params oas.PatchWebhookParams) (*oas.Webhook, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p := webhook.Patch{}
	if v, ok := req.URL.Get(); ok {
		s := v.String()
		p.URL = &s
	}
	if req.Events != nil {
		p.Events = eventsOf(req.Events)
	}
	if v, ok := req.Enabled.Get(); ok {
		p.Enabled = &v
	}
	if v, ok := req.Description.Get(); ok {
		p.Description = &v
	}
	w, err := h.deps.Webhooks.Update(ctx, a, t.ID, params.ID, p)
	if err != nil {
		return nil, err
	}
	last, _ := h.deps.Webhooks.LastDelivery(ctx, w.ID) //nolint:errcheck // summary is best-effort
	out := webhookOf(w, last)
	return &out, nil
}

// DeleteWebhook — remove.
func (h *Handler) DeleteWebhook(ctx context.Context, params oas.DeleteWebhookParams) error {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Webhooks.Delete(ctx, a, t.ID, params.ID)
}

// RotateWebhookSecret — new secret, 24h overlap.
func (h *Handler) RotateWebhookSecret(ctx context.Context, params oas.RotateWebhookSecretParams) (*oas.WebhookCreated, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	w, err := h.deps.Webhooks.Rotate(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return createdWebhookOf(w), nil
}

// ListWebhookDeliveries — delivery log (cursor = created_at of the last row).
func (h *Handler) ListWebhookDeliveries(ctx context.Context, params oas.ListWebhookDeliveriesParams) (*oas.ListWebhookDeliveriesOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	var before *time.Time
	if c, ok := params.Cursor.Get(); ok && c != "" {
		ts, err := time.Parse(time.RFC3339Nano, c)
		if err != nil {
			return nil, errs.Invalid("cursor").Because("cursor_invalid")
		}
		before = &ts
	}
	limit := params.Limit.Or(50)
	ds, err := h.deps.Webhooks.Deliveries(ctx, a, t.ID, params.ID, before, limit)
	if err != nil {
		return nil, err
	}
	hasMore := len(ds) > limit
	if hasMore {
		ds = ds[:limit]
	}
	out := &oas.ListWebhookDeliveriesOK{Data: make([]oas.WebhookDelivery, 0, len(ds))}
	for _, d := range ds {
		out.Data = append(out.Data, deliveryOf(d))
	}
	out.Meta.HasMore = oas.NewOptBool(hasMore)
	if hasMore {
		out.Meta.NextCursor = oas.NewOptNilString(ds[len(ds)-1].CreatedAt.Format(time.RFC3339Nano))
	}
	return out, nil
}

// ReplayWebhookDelivery — re-send.
func (h *Handler) ReplayWebhookDelivery(ctx context.Context, params oas.ReplayWebhookDeliveryParams) (*oas.WebhookDelivery, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	deliveryID, err := uuid.Parse(params.DeliveryId)
	if err != nil {
		return nil, errs.Invalid("deliveryId is not a uuid").Because("delivery_id_is_not_a_uuid")
	}
	d, err := h.deps.Webhooks.Replay(ctx, a, t.ID, params.ID, deliveryID)
	if err != nil {
		return nil, err
	}
	out := deliveryOf(d)
	return &out, nil
}

func eventsOf(in []oas.WebhookEvent) []string {
	out := make([]string, 0, len(in))
	for _, e := range in {
		out = append(out, string(e))
	}
	return out
}

func webhookOf(w webhook.Webhook, last *webhook.LastDelivery) oas.Webhook {
	u, _ := url.Parse(w.URL) //nolint:errcheck // validated on write
	out := oas.Webhook{ID: w.ID, URL: *u, Enabled: w.Enabled, CreatedAt: w.CreatedAt, Events: make([]oas.WebhookEvent, 0, len(w.Events))}
	for _, e := range w.Events {
		out.Events = append(out.Events, oas.WebhookEvent(e))
	}
	if w.Description != "" {
		out.Description = oas.NewOptString(w.Description)
	}
	if last != nil {
		out.LastDelivery = oas.NewOptWebhookLastDelivery(oas.WebhookLastDelivery{
			At: oas.NewOptDateTime(last.At), Status: oas.NewOptWebhookLastDeliveryStatus(oas.WebhookLastDeliveryStatus(last.Status)),
		})
	}
	return out
}

func createdWebhookOf(w webhook.Webhook) *oas.WebhookCreated {
	base := webhookOf(w, nil)
	return &oas.WebhookCreated{
		ID: base.ID, URL: base.URL, Events: base.Events, Enabled: base.Enabled, Description: base.Description,
		CreatedAt: base.CreatedAt, Secret: w.Secret,
	}
}

func deliveryOf(d webhook.Delivery) oas.WebhookDelivery {
	out := oas.WebhookDelivery{
		ID: d.ID.String(), Event: oas.WebhookEvent(d.Event), Status: oas.WebhookDeliveryStatus(d.Status),
		Attempts: d.Attempts, CreatedAt: d.CreatedAt,
	}
	if d.LastAttemptAt != nil {
		out.LastAttemptAt = oas.NewOptNilDateTime(*d.LastAttemptAt)
	}
	if d.ResponseStatus != nil {
		out.ResponseStatus = oas.NewOptNilInt(*d.ResponseStatus)
	}
	if d.Error != "" {
		out.Error = oas.NewOptString(d.Error)
	}
	if len(d.Payload) > 0 {
		var m map[string]json.RawMessage
		if json.Unmarshal(d.Payload, &m) == nil {
			p := oas.WebhookDeliveryPayload{}
			for k, v := range m {
				p[k] = jx.Raw(v)
			}
			out.Payload = oas.NewOptWebhookDeliveryPayload(p)
		}
	}
	return out
}

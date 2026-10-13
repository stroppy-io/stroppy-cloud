// Package webhook is the tenant's outbound notifications: Standard
// Webhooks (HMAC-SHA256 over `id.timestamp.body`), per-event deliveries
// with backoff retries, a delivery log and replay.
package webhook

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

// Event names.
const (
	RunStarted        = "run.started"
	RunStage          = "run.stage"
	RunFinished       = "run.finished"
	RunFailed         = "run.failed"
	RunCancelled      = "run.cancelled"
	SuiteStarted      = "suite.started"
	SuiteCellFinished = "suite.cell_finished"
	SuiteFinished     = "suite.finished"
)

var knownEvents = map[string]bool{
	RunStarted: true, RunStage: true, RunFinished: true, RunFailed: true, RunCancelled: true,
	SuiteStarted: true, SuiteCellFinished: true, SuiteFinished: true,
}

// RotationOverlap is how long the previous secret keeps verifying.
const RotationOverlap = 24 * time.Hour

// Webhook is the record.
type Webhook struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	URL           string
	Events        []string
	Enabled       bool
	Description   string
	Secret        string
	PrevSecret    string
	PrevExpiresAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Wants reports a subscription to the event.
func (w Webhook) Wants(event string) bool {
	for _, e := range w.Events {
		if e == event {
			return true
		}
	}
	return false
}

// DeliveryStatus of one delivery.
type DeliveryStatus string

// Statuses.
const (
	DeliveryPending DeliveryStatus = "pending"
	DeliverySuccess DeliveryStatus = "success"
	DeliveryFailed  DeliveryStatus = "failed"
)

// Delivery is one event sent to one webhook.
type Delivery struct {
	ID             uuid.UUID
	WebhookID      uuid.UUID
	Event          string
	Status         DeliveryStatus
	Attempts       int
	NextAttemptAt  time.Time
	LastAttemptAt  *time.Time
	ResponseStatus *int
	Error          string
	Payload        json.RawMessage
	CreatedAt      time.Time
}

// LastDelivery is the summary on a webhook.
type LastDelivery struct {
	At     time.Time
	Status DeliveryStatus
}

// Create is the request.
type Create struct {
	URL         string
	Events      []string
	Enabled     *bool
	Description string
}

// Patch is a partial update; nil = keep.
type Patch struct {
	URL         *string
	Events      []string
	Enabled     *bool
	Description *string
}

// Repository is the storage port.
type Repository interface {
	Insert(ctx context.Context, w Webhook) error
	ByID(ctx context.Context, id uuid.UUID) (Webhook, error)
	OfTenant(ctx context.Context, tenantID uuid.UUID, enabledOnly bool) ([]Webhook, error)
	Update(ctx context.Context, id uuid.UUID, p Patch) error
	Rotate(ctx context.Context, id uuid.UUID, secret string, prevExpires time.Time) error
	Delete(ctx context.Context, id uuid.UUID) error
	LastDelivery(ctx context.Context, id uuid.UUID) (*LastDelivery, error)

	InsertDelivery(ctx context.Context, d Delivery) error
	DeliveryByID(ctx context.Context, id uuid.UUID) (Delivery, error)
	Deliveries(ctx context.Context, webhookID uuid.UUID, before *time.Time, limit int) ([]Delivery, error)
	Due(ctx context.Context, lease time.Duration, limit int) ([]Delivery, error)
	Finish(ctx context.Context, id uuid.UUID, status DeliveryStatus, next time.Time, responseStatus *int, errText string) error
	Reset(ctx context.Context, id uuid.UUID) error
}

// Access resolves the caller's role in a tenant.
type Access interface {
	RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, role string, err error)
}

// Service is the webhook use cases.
type Service struct {
	repo   Repository
	access Access
	audit  *audit.Service
}

// NewService builds the service.
func NewService(repo Repository, access Access, auditSvc *audit.Service) *Service {
	return &Service{repo: repo, access: access, audit: auditSvc}
}

func (s *Service) admin(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) error {
	_, role, err := s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return err
	}
	if role != "owner" && role != "admin" {
		return errs.Forbidden("requires role admin").Because("requires_role_admin")
	}
	return nil
}

func (s *Service) owned(ctx context.Context, tenantID, id uuid.UUID) error {
	w, err := s.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	if w.TenantID != tenantID {
		return errs.NotFound("webhook")
	}
	return nil
}

// List returns the tenant's webhooks (admin+; secrets are never listed).
func (s *Service) List(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) ([]Webhook, error) {
	if err := s.admin(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return s.repo.OfTenant(ctx, tenantID, false)
}

// LastDelivery is the summary for a listing.
func (s *Service) LastDelivery(ctx context.Context, id uuid.UUID) (*LastDelivery, error) {
	return s.repo.LastDelivery(ctx, id)
}

// Create adds a webhook; the secret is returned once.
func (s *Service) Create(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, req Create) (Webhook, error) {
	if err := s.admin(ctx, actor, tenantID); err != nil {
		return Webhook{}, err
	}
	if err := validateURL(req.URL); err != nil {
		return Webhook{}, err
	}
	events, err := validateEvents(req.Events)
	if err != nil {
		return Webhook{}, err
	}
	w := Webhook{
		ID: uuid.New(), TenantID: tenantID, URL: req.URL, Events: events, Enabled: true,
		Description: strings.TrimSpace(req.Description), Secret: newSecret(), CreatedAt: time.Now().UTC(),
	}
	if req.Enabled != nil {
		w.Enabled = *req.Enabled
	}
	if err := s.repo.Insert(ctx, w); err != nil {
		return Webhook{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "webhook.create", Target: audit.Target{Kind: "webhook", ID: w.ID.String(), Name: w.URL}}) //nolint:errcheck // audit never blocks
	return w, nil
}

// Update changes url/events/enabled/description.
func (s *Service) Update(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, p Patch) (Webhook, error) {
	if err := s.admin(ctx, actor, tenantID); err != nil {
		return Webhook{}, err
	}
	if err := s.owned(ctx, tenantID, id); err != nil {
		return Webhook{}, err
	}
	if p.URL != nil {
		if err := validateURL(*p.URL); err != nil {
			return Webhook{}, err
		}
	}
	if p.Events != nil {
		events, err := validateEvents(p.Events)
		if err != nil {
			return Webhook{}, err
		}
		p.Events = events
	}
	if err := s.repo.Update(ctx, id, p); err != nil {
		return Webhook{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "webhook.update", Target: audit.Target{Kind: "webhook", ID: id.String()}}) //nolint:errcheck // audit never blocks
	return s.repo.ByID(ctx, id)
}

// Delete removes a webhook and its delivery log.
func (s *Service) Delete(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) error {
	if err := s.admin(ctx, actor, tenantID); err != nil {
		return err
	}
	if err := s.owned(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "webhook.delete", Target: audit.Target{Kind: "webhook", ID: id.String()}})
}

// Rotate mints a new secret; the old one verifies for RotationOverlap.
func (s *Service) Rotate(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Webhook, error) {
	if err := s.admin(ctx, actor, tenantID); err != nil {
		return Webhook{}, err
	}
	if err := s.owned(ctx, tenantID, id); err != nil {
		return Webhook{}, err
	}
	if err := s.repo.Rotate(ctx, id, newSecret(), time.Now().UTC().Add(RotationOverlap)); err != nil {
		return Webhook{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "webhook.rotate", Target: audit.Target{Kind: "webhook", ID: id.String()}}) //nolint:errcheck // audit never blocks
	return s.repo.ByID(ctx, id)
}

// Deliveries is the delivery log page (limit+1 rows for has_more).
func (s *Service) Deliveries(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, before *time.Time, limit int) ([]Delivery, error) {
	if err := s.admin(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	if err := s.owned(ctx, tenantID, id); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.Deliveries(ctx, id, before, limit+1)
}

// Replay re-queues a delivery.
func (s *Service) Replay(ctx context.Context, actor auth.Actor, tenantID, id, deliveryID uuid.UUID) (Delivery, error) {
	if err := s.admin(ctx, actor, tenantID); err != nil {
		return Delivery{}, err
	}
	if err := s.owned(ctx, tenantID, id); err != nil {
		return Delivery{}, err
	}
	d, err := s.repo.DeliveryByID(ctx, deliveryID)
	if err != nil || d.WebhookID != id {
		return Delivery{}, errs.NotFound("delivery")
	}
	if err := s.repo.Reset(ctx, deliveryID); err != nil {
		return Delivery{}, err
	}
	return s.repo.DeliveryByID(ctx, deliveryID)
}

// Publish queues an event for every enabled webhook of the tenant that
// subscribes to it. Called by the run/suite projections.
func (s *Service) Publish(ctx context.Context, tenantID uuid.UUID, event string, payload any) error {
	hooks, err := s.repo.OfTenant(ctx, tenantID, true)
	if err != nil {
		return err
	}
	var body json.RawMessage
	for _, w := range hooks {
		if !w.Wants(event) {
			continue
		}
		if body == nil {
			body, err = json.Marshal(payload)
			if err != nil {
				return err
			}
		}
		if err := s.repo.InsertDelivery(ctx, Delivery{ID: uuid.New(), WebhookID: w.ID, Event: event, Status: DeliveryPending, Payload: body, CreatedAt: time.Now().UTC()}); err != nil {
			return err
		}
	}
	return nil
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return errs.Invalid("url must be an absolute http(s) URL").Because("url_not_absolute_http")
	}
	return nil
}

func validateEvents(events []string) ([]string, error) {
	if len(events) == 0 {
		return nil, errs.Invalid("at least one event").Because("at_least_one_event")
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(events))
	for _, e := range events {
		if !knownEvents[e] {
			return nil, errs.Invalid("unknown event "+e).Because("webhook_event_unknown", "event", e)
		}
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	return out, nil
}

// newSecret is a Standard Webhooks secret: whsec_ + base64 of 32 bytes.
func newSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return "whsec_" + base64.StdEncoding.EncodeToString(b)
}

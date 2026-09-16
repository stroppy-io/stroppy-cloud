// Package provider is the tenant's cloud accounts: named profiles whose
// credentials live only in Graphene secrets, verified by the
// stroppy-provider-verify pipeline and measured by stroppy-quotas. An
// unverified profile cannot launch anything.
package provider

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Kind of cloud.
type Kind string

// Kinds.
const (
	KindYandex Kind = "yandex"
	KindAWS    Kind = "aws"
)

// Valid reports a known kind.
func (k Kind) Valid() bool { return k == KindYandex || k == KindAWS }

// Status of a profile.
type Status string

// Statuses.
const (
	StatusVerifying Status = "verifying"
	StatusReady     Status = "ready"
	StatusFailed    Status = "failed"
)

// Profile is the record.
type Profile struct {
	ID                      uuid.UUID
	TenantID                uuid.UUID
	Name                    string
	Kind                    Kind
	Settings                json.RawMessage
	SecretNames             []string
	Status                  Status
	StatusReason            string
	VerifiedAt              *time.Time
	VerifyRunID             string
	Quotas                  []Quota
	QuotasObservedAt        *time.Time
	QuotasUnavailableReason string
	QuotasScope             string
	CreatedBy               *uuid.UUID
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// Quota is one cached cloud limit.
type Quota struct {
	Name  string  `json:"name"`
	Limit float64 `json:"limit"`
	Used  float64 `json:"used"`
	Unit  string  `json:"unit,omitempty"`
	Zone  string  `json:"zone,omitempty"`
}

// CredentialsSecret is the Graphene secret name of a profile.
func CredentialsSecret(id uuid.UUID) string { return "provider-" + id.String() }

// Repository is the storage port.
type Repository interface {
	Insert(ctx context.Context, p Profile) error
	ByID(ctx context.Context, id uuid.UUID) (Profile, error)
	OfTenant(ctx context.Context, tenantID uuid.UUID) ([]Profile, error)
	Ready(ctx context.Context) ([]Profile, error)
	Update(ctx context.Context, id uuid.UUID, name *string, settings json.RawMessage) error
	SetStatus(ctx context.Context, id uuid.UUID, status Status, reason, runID string) error
	SetQuotas(ctx context.Context, id uuid.UUID, result QuotasResult) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// Secrets is the Graphene secret store of a tenant namespace.
type Secrets interface {
	SetSecret(ctx context.Context, name, value string) error
	DeleteSecret(ctx context.Context, name string) error
}

// VerifyResult is what the verification pipeline returns.
type VerifyResult struct {
	OK          bool         `json:"ok"`
	AccountID   string       `json:"account_id,omitempty"`
	Scope       string       `json:"scope,omitempty"`
	Permissions []Permission `json:"permissions,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// Permission is one checked right.
type Permission struct {
	Name    string `json:"name"`
	Granted bool   `json:"granted"`
}

// QuotasResult is what the quota pipeline returns.
type QuotasResult struct {
	// UnavailableReason is permission_denied when cloud quotas cannot be read.
	// An unavailable snapshot has no quotas; provisioning still enforces limits.
	UnavailableReason string    `json:"unavailable_reason,omitempty"`
	Scope             string    `json:"scope,omitempty"`
	ObservedAt        time.Time `json:"observed_at"`
	Quotas            []Quota   `json:"quotas"`
}

// Pipelines runs the two probe pipelines in the tenant namespace and
// waits for their results.
type Pipelines interface {
	Verify(ctx context.Context, runID string, params VerifyParams) (VerifyResult, error)
	Quotas(ctx context.Context, runID string, params QuotasParams) (QuotasResult, error)
}

// VerifyParams is spec.provider_verify@1.
type VerifyParams struct {
	Provider          Kind            `json:"provider"`
	Settings          json.RawMessage `json:"settings"`
	CredentialsSecret string          `json:"credentials_secret"`
	DryRun            bool            `json:"dry_run,omitempty"`
}

// QuotasParams is spec.quotas@1.
type QuotasParams struct {
	Provider          Kind            `json:"provider"`
	Settings          json.RawMessage `json:"settings"`
	CredentialsSecret string          `json:"credentials_secret"`
	Location          string          `json:"location,omitempty"`
}

// Validator bakes schemapb values: settings and credentials of a kind.
type Validator interface {
	// Bake validates value against the schema id and returns the baked
	// (defaults applied) value, or a validation error (errs.CodeValidation).
	Bake(ctx context.Context, schemaID string, value json.RawMessage) (json.RawMessage, error)
}

// Schema ids of a kind.
func settingsSchema(k Kind) string    { return "provider." + string(k) + ".settings@1" }
func credentialsSchema(k Kind) string { return "provider." + string(k) + ".credentials@1" }

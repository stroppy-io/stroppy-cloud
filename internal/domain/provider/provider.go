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
	StatusVerifying    Status = "verifying"
	StatusReady        Status = "ready"
	StatusFailed       Status = "failed"
	StatusDeleting     Status = "deleting"
	StatusDeleteFailed Status = "delete_failed"
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
	// ConfigRevision is the ConfigRevision the last successful ensure
	// applied; below the server's, a Yandex profile has no profile network.
	ConfigRevision int
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Quota is one cached cloud limit.
type Quota struct {
	Name  string  `json:"name"`
	Limit float64 `json:"limit"`
	Used  float64 `json:"used"`
	Unit  string  `json:"unit,omitempty"`
	Zone  string  `json:"zone,omitempty"`
}

// ConfigRevision is what stroppy-provider-config ensure sets up today:
// 1 = the ProviderConfig plus, for Yandex, the profile network every run
// joins. A ready profile below it is configured again before its queued
// runs start (Service.UpgradeStale).
const ConfigRevision = 1

// Configured reports a profile whose configuration is what runs expect:
// a Yandex profile needs its profile network, AWS needs nothing more.
func (p Profile) Configured() bool {
	return p.Kind != KindYandex || p.ConfigRevision >= ConfigRevision
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
	BeginOperation(ctx context.Context, id uuid.UUID, status Status, runID string) error
	// FinishOperation ends an operation; a ready end records ConfigRevision.
	FinishOperation(ctx context.Context, id uuid.UUID, runID string, status Status, reason string) error
	// Stale lists ready profiles of a kind configured below revision.
	Stale(ctx context.Context, kind Kind, revision int) ([]Profile, error)
	// BeginUpgrade turns a stale ready profile verifying for runID while
	// its queued runs wait; false when it is not stale any more.
	BeginUpgrade(ctx context.Context, id uuid.UUID, runID string, revision int) (bool, error)
	// RetryUpgrade ends a failed upgrade: ready on the old configuration,
	// the reason kept, tried again later.
	RetryUpgrade(ctx context.Context, id uuid.UUID, runID, reason string) error
	Pending(ctx context.Context) ([]Profile, error)
	BeginTenantDeletion(ctx context.Context, id uuid.UUID) error
	UpdateConfiguration(ctx context.Context, id uuid.UUID, name *string, settings json.RawMessage, credential string, runID string) error
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
	Configure(ctx context.Context, runID string, params ConfigureParams) (VerifyResult, error)
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

// ConfigureParams is the dedicated provider lifecycle pipeline input.
type ConfigureParams struct {
	Action            string          `json:"action"`
	Provider          Kind            `json:"provider"`
	ProfileID         string          `json:"profile_id"`
	Settings          json.RawMessage `json:"settings,omitempty"`
	CredentialsSecret string          `json:"credentials_secret"`
}

// ActiveCredentials is the immutable credential version selected by the profile.
func ActiveCredentials(p Profile) string {
	if len(p.SecretNames) > 0 {
		return p.SecretNames[0]
	}
	return CredentialsSecret(p.ID)
}

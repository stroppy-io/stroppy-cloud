package spec

import (
	"encoding/json"
	"time"
)

// Result is spec.result.run@1 — what a run returns to the server.
type Result struct {
	Metrics   map[string]MetricValue `json:"metrics,omitempty"`
	Segments  []SegmentResult        `json:"segments,omitempty"`
	Artifacts []string               `json:"artifacts,omitempty"`
	Baseline  *BaselineResult        `json:"baseline,omitempty"`
	Summary   Summary                `json:"summary,omitempty,omitzero"`
}

// BaselineResult is the outcome of `stroppy baseline` on the runner.
type BaselineResult struct {
	OK       bool              `json:"ok"`
	Verdicts []BaselineVerdict `json:"verdicts,omitempty"`
	Report   json.RawMessage   `json:"report,omitempty"`
	Error    string            `json:"error,omitempty"`
}

// BaselineVerdict is one invariant stroppy checked.
type BaselineVerdict struct {
	Check  string `json:"check"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// ErrorCounts is stroppy's own nonfatal error accounting of a segment.
//
// doc: stroppy `help drivers` ERROR AND EXIT BEHAVIOR.
type ErrorCounts struct {
	TerminalErrors   int64 `json:"terminal_errors"`
	FailedIterations int64 `json:"failed_iterations"`
	FailedQueries    int64 `json:"failed_queries"`
	RetryAttempts    int64 `json:"retry_attempts"`
}

// MetricValue is one aggregated metric.
type MetricValue struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
	Min   float64 `json:"min,omitempty"`
	Max   float64 `json:"max,omitempty"`
	Avg   float64 `json:"avg,omitempty"`
}

// SegmentStatus is the terminal state of a segment.
type SegmentStatus string

// Segment statuses.
const (
	SegmentCompleted SegmentStatus = "completed"
	SegmentFailed    SegmentStatus = "failed"
	SegmentSkipped   SegmentStatus = "skipped"
	SegmentCancelled SegmentStatus = "canceled"
)

// SegmentResult is the outcome of one workload segment.
type SegmentResult struct {
	Name       string                 `json:"name"`
	Status     SegmentStatus          `json:"status"`
	StartedAt  time.Time              `json:"started_at,omitempty,omitzero"`
	FinishedAt time.Time              `json:"finished_at,omitempty,omitzero"`
	Metrics    map[string]MetricValue `json:"metrics,omitempty"`
	Errors     *ErrorCounts           `json:"errors,omitempty"`
	ExitCode   int                    `json:"exit_code"`
	// Compliance is the TPC-C report stroppy prints as {"compliance": …}.
	Compliance json.RawMessage `json:"compliance,omitempty"`
	// Report is the native versioned Stroppy envelope, with unknown fields retained.
	Report json.RawMessage `json:"report,omitempty"`
	// ReportArtifact is set only after the original file has been published.
	ReportArtifact string `json:"report_artifact,omitempty"`
	// ReportOmitted explains why the envelope is available only as an artifact.
	ReportOmitted string `json:"report_omitted,omitempty"`
	Error         string `json:"error,omitempty"`
	// ErrorLog is what Stroppy logged as errors and warnings, grouped.
	ErrorLog []ErrorLogEntry `json:"error_log,omitempty"`
}

// ErrorLogEntry is one kind of error a segment logged: the same level,
// message, error text and class, with how often and when it happened.
type ErrorLogEntry struct {
	Level   string    `json:"level"`
	Message string    `json:"message,omitempty"`
	Error   string    `json:"error,omitempty"`
	Class   string    `json:"class,omitempty"`
	Count   int64     `json:"count"`
	FirstAt time.Time `json:"first_at,omitzero"`
	LastAt  time.Time `json:"last_at,omitzero"`
	// SQL is one statement that failed this way, cut to 512 bytes.
	SQL string `json:"sql,omitempty"`
}

// Summary is the headline of a run.
type Summary struct {
	TPS          float64  `json:"tps,omitempty"`
	LatencyP50Ms float64  `json:"latency_p50_ms,omitempty"`
	LatencyP95Ms float64  `json:"latency_p95_ms,omitempty"`
	LatencyP99Ms float64  `json:"latency_p99_ms,omitempty"`
	Errors       int64    `json:"errors,omitempty"`
	Duration     Duration `json:"duration,omitempty"`
}

// ProviderVerify is spec.provider_verify@1 — params of the credential
// verification pipeline.
type ProviderVerify struct {
	Provider          ProviderKind    `json:"provider"`
	Settings          json.RawMessage `json:"settings"`
	CredentialsSecret string          `json:"credentials_secret"`
	// DryRun checks the credentials without touching billable resources.
	DryRun bool `json:"dry_run,omitempty"`
}

// ProviderVerifyResult is spec.result.provider_verify@1.
type ProviderVerifyResult struct {
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

// Quotas is spec.quotas@1 — params of the quota probe pipeline.
type Quotas struct {
	Provider          ProviderKind    `json:"provider"`
	Settings          json.RawMessage `json:"settings"`
	CredentialsSecret string          `json:"credentials_secret"`
	Location          string          `json:"location,omitempty"`
}

// QuotasResult is spec.result.quotas@1.
type QuotasResult struct {
	// UnavailableReason is permission_denied when cloud quotas cannot be read.
	// An unavailable snapshot has no quotas; provisioning still enforces limits.
	UnavailableReason string    `json:"unavailable_reason,omitempty"`
	Scope             string    `json:"scope,omitempty"`
	ObservedAt        time.Time `json:"observed_at"`
	Quotas            []Quota   `json:"quotas"`
}

// Quota is one cloud limit with its usage.
type Quota struct {
	Name  string  `json:"name"`
	Limit float64 `json:"limit"`
	Used  float64 `json:"used"`
	Unit  string  `json:"unit,omitempty"`
	Zone  string  `json:"zone,omitempty"`
}

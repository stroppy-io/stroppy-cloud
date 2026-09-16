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
	Error      string          `json:"error,omitempty"`
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

// Suite is spec.suite@1 — a fan-out of runs.
type Suite struct {
	SuiteRunID  string        `json:"suite_run_id"`
	Tenant      string        `json:"tenant"`
	Cells       []SuiteCell   `json:"cells"`
	Concurrency int           `json:"concurrency,omitempty"`
	Defaults    SuiteDefaults `json:"defaults,omitempty,omitzero"`
}

// SuiteCell is one run of a suite.
type SuiteCell struct {
	ID      string `json:"id"`
	RunSpec Run    `json:"run_spec"`
}

// SuiteDefaults apply to the whole fan-out.
type SuiteDefaults struct {
	// ContinueOnFailure keeps starting cells after one fails.
	ContinueOnFailure bool `json:"continue_on_failure,omitempty"`
	// Labels are stamped on every child run.
	Labels map[string]string `json:"labels,omitempty"`
}

// SuiteResult is what stroppy-suite returns.
type SuiteResult struct {
	Cells []SuiteCellResult `json:"cells"`
	Total int               `json:"total"`
	Done  int               `json:"done"`
	Fail  int               `json:"failed"`
}

// SuiteCellResult pairs a cell with its outcome.
type SuiteCellResult struct {
	ID     string  `json:"id"`
	RunID  string  `json:"run_id"`
	Status string  `json:"status"`
	Error  string  `json:"error,omitempty"`
	Result *Result `json:"result,omitempty"`
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

package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestContractVersionHeader(t *testing.T) {
	r := httptest.NewRecorder()
	AcceptMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := r.Header().Get("X-Stroppy-Contract-Version"); got != spec.ContractVersion {
		t.Fatalf("contract version header %q", got)
	}
}

func TestStructuredValidationReachesProblemAndFit(t *testing.T) {
	findings := []spec.ResourceIssue{
		{Scope: "run_spec", Path: "machines[0].disks[0].gb", Code: "capacity_insufficient", Severity: "ERROR", Message: "insufficient storage"},
		{Scope: "run_spec", Path: "workload.segments", Code: "capacity_unknown", Severity: "WARNING", Message: "custom SQL has unknown volume"},
	}
	h := &Handler{}
	p := h.NewError(t.Context(), fmt.Errorf("wrapped: %w", spec.ResourceErrors(findings)))
	if p.StatusCode != 422 || len(p.Response.Validation.Value.Errors) != 2 {
		t.Fatalf("lost validation: %+v", p.Response)
	}
	got := p.Response.Validation.Value.Errors[0]
	if got.Path != findings[0].Path || got.Code != findings[0].Code || string(got.Scope.Value) != "run_spec" {
		t.Fatalf("lost diagnostic fields: %+v", got)
	}
	issues := []library.Issue{{Scope: "run_spec", Path: findings[0].Path, Code: findings[0].Code, Severity: "ERROR", Message: findings[0].Message}}
	err := errs.Invalid("test does not fit")
	err.Validation = issues
	launch := h.NewError(t.Context(), err)
	fit := h.fitOf(library.Fit{Issues: issues})
	if launch.Response.Validation.Value.Errors[0].Path != fit.Issues[0].Path || launch.Response.Validation.Value.Errors[0].Scope != fit.Issues[0].Scope {
		t.Fatal("launch and preview disagree on diagnostic location")
	}
}

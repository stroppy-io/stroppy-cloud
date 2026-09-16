package probe

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/graphene-ci/pipeline/pkg/pipelinetest"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// The probes are one run-queue activity each; the simulation checks the
// wiring (params in, result out, routing to the run queue) without a cloud.

func TestSimulatedVerify(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	w := pipelinetest.Install(t, suite.NewTestWorkflowEnvironment())
	var got spec.ProviderVerify
	pipelinetest.Handle1(w, nameVerify, func(_ workflow.Context, p spec.ProviderVerify) (spec.ProviderVerifyResult, error) {
		got = p
		return spec.ProviderVerifyResult{OK: false, Scope: "folder:b1g", Permissions: []spec.Permission{{Name: "compute.instances.list", Granted: false}}, Error: "missing rights"}, nil
	})
	wf := pipelinetest.Workflow(w, VerifyPipelineID, Verify)
	w.Env.ExecuteWorkflow(wf, spec.ProviderVerify{Provider: spec.ProviderYandex, Settings: json.RawMessage(`{"folder_id":"b1g"}`), CredentialsSecret: "yc-sa-key", DryRun: true})
	require.NoError(t, w.Env.GetWorkflowError())
	var res spec.ProviderVerifyResult
	require.NoError(t, w.Env.GetWorkflowResult(&res))
	require.False(t, res.OK)
	require.Equal(t, "missing rights", res.Error)
	require.Equal(t, "yc-sa-key", got.CredentialsSecret)
	require.True(t, got.DryRun)
	// One probe activity on the run queue, then the SDK's own cleanup.
	calls := w.Calls()
	require.Len(t, calls, 2)
	require.Equal(t, nameVerify, calls[0].Name)
	require.Equal(t, "run/test-"+VerifyPipelineID, calls[0].TaskQueue)
}

func TestSimulatedQuotas(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	w := pipelinetest.Install(t, suite.NewTestWorkflowEnvironment())
	pipelinetest.Handle1(w, nameQuotas, func(_ workflow.Context, p spec.Quotas) (spec.QuotasResult, error) {
		return spec.QuotasResult{ObservedAt: time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC), Quotas: []spec.Quota{{Name: "compute.instanceCores.count", Limit: 32, Used: 8, Unit: "cores", Zone: p.Location}}}, nil
	})
	wf := pipelinetest.Workflow(w, QuotasPipelineID, Quotas)
	w.Env.ExecuteWorkflow(wf, spec.Quotas{Provider: spec.ProviderAWS, Settings: json.RawMessage(`{"region":"eu-central-1"}`), CredentialsSecret: "aws-keys", Location: "eu-central-1a"})
	require.NoError(t, w.Env.GetWorkflowError())
	var res spec.QuotasResult
	require.NoError(t, w.Env.GetWorkflowResult(&res))
	require.Len(t, res.Quotas, 1)
	require.Equal(t, "eu-central-1a", res.Quotas[0].Zone)
}

func TestSimulatedQuotasUnavailable(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	w := pipelinetest.Install(t, suite.NewTestWorkflowEnvironment())
	expected := spec.QuotasResult{ObservedAt: time.Now().UTC(), Quotas: []spec.Quota{}, Scope: "cloud:b1g", UnavailableReason: "permission_denied"}
	pipelinetest.Handle1(w, nameQuotas, func(_ workflow.Context, _ spec.Quotas) (spec.QuotasResult, error) { return expected, nil })
	w.Env.ExecuteWorkflow(pipelinetest.Workflow(w, QuotasPipelineID, Quotas), spec.Quotas{Provider: spec.ProviderYandex, CredentialsSecret: "yc-sa-key"})
	require.NoError(t, w.Env.GetWorkflowError())
	var res spec.QuotasResult
	require.NoError(t, w.Env.GetWorkflowResult(&res))
	require.Equal(t, expected, res)
	require.Len(t, w.Calls(), 2, "one probe plus cleanup; permission denial is not retried")
}

package providerconfig

import (
	"encoding/json"
	"github.com/graphene-ci/pipeline/pkg/pipelinetest"
	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"testing"
)

func TestSimulatedConfigurationLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name, action               string
		verifyOK, configureFailure bool
		wantVerify, wantConfigure  int
	}{
		{"ensure", "ensure", true, false, 1, 1},
		{"denied", "ensure", false, false, 1, 0},
		{"delete does not read cloud credentials", "delete", false, false, 0, 1},
		{"cleanup failure propagates", "delete", false, true, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			w := pipelinetest.Install(t, suite.NewTestWorkflowEnvironment())
			checks, configs := 0, 0
			pipelinetest.Handle1(w, VerifyActivity, func(_ workflow.Context, _ spec.ProviderConfig) (spec.ProviderVerifyResult, error) {
				checks++
				return spec.ProviderVerifyResult{OK: tc.verifyOK}, nil
			})
			pipelinetest.Handle1(w, ConfigureActivity, func(_ workflow.Context, p spec.ProviderConfig) (spec.ProviderVerifyResult, error) {
				configs++
				require.Equal(t, tc.action, p.Action)
				if tc.configureFailure {
					return spec.ProviderVerifyResult{}, temporal.NewNonRetryableApplicationError("in use", "in-use", nil)
				}
				return spec.ProviderVerifyResult{OK: true}, nil
			})
			p, _, _, _ := fixture(t)
			p.Action = tc.action
			p.Settings = json.RawMessage(`{}`)
			w.Env.ExecuteWorkflow(pipelinetest.Workflow(w, PipelineID, Run), p)
			if tc.configureFailure {
				require.Error(t, w.Env.GetWorkflowError())
			} else {
				require.NoError(t, w.Env.GetWorkflowError())
			}
			require.Equal(t, tc.wantVerify, checks)
			require.Equal(t, tc.wantConfigure, configs)
		})
	}
}

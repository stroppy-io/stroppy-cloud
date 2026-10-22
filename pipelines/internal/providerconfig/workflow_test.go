package providerconfig

import (
	"encoding/json"
	"testing"

	"github.com/graphene-ci/pipeline/pkg/pipelinetest"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestSimulatedConfigurationLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name, action                       string
		verifyOK, configureFailure         bool
		networkFailure                     bool
		wantVerify, wantConfigure, wantNet int
		// order is the activities in the order they ran.
		order []string
	}{
		{"ensure", "ensure", true, false, false, 1, 1, 1, []string{"verify", "configure", "network"}},
		{"denied", "ensure", false, false, false, 1, 0, 0, []string{"verify"}},
		{"network failure fails ensure", "ensure", true, false, true, 1, 1, 1, []string{"verify", "configure", "network"}},
		{"delete does not read cloud credentials", "delete", false, false, false, 0, 1, 1, []string{"network", "configure"}},
		{"cleanup failure propagates", "delete", false, true, false, 0, 1, 1, []string{"network", "configure"}},
		{"network in use keeps the config", "delete", false, false, true, 0, 0, 1, []string{"network"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			w := pipelinetest.Install(t, suite.NewTestWorkflowEnvironment())
			checks, configs, nets := 0, 0, 0
			var order []string
			pipelinetest.Handle1(w, VerifyActivity, func(_ workflow.Context, _ spec.ProviderConfig) (spec.ProviderVerifyResult, error) {
				checks++
				order = append(order, "verify")
				return spec.ProviderVerifyResult{OK: tc.verifyOK}, nil
			})
			pipelinetest.Handle1(w, ConfigureActivity, func(_ workflow.Context, p spec.ProviderConfig) (spec.ProviderVerifyResult, error) {
				configs++
				order = append(order, "configure")
				require.Equal(t, tc.action, p.Action)
				if tc.configureFailure {
					return spec.ProviderVerifyResult{}, temporal.NewNonRetryableApplicationError("in use", "in-use", nil)
				}
				return spec.ProviderVerifyResult{OK: true}, nil
			})
			pipelinetest.Handle1(w, NetworkActivity, func(_ workflow.Context, p spec.ProviderConfig) (spec.ProviderVerifyResult, error) {
				nets++
				order = append(order, "network")
				require.Equal(t, tc.action, p.Action)
				if tc.networkFailure {
					return spec.ProviderVerifyResult{}, temporal.NewNonRetryableApplicationError("still in use by Subnet", "in-use", nil)
				}
				return spec.ProviderVerifyResult{OK: true}, nil
			})
			p, _, _, _ := fixture(t)
			p.Action = tc.action
			p.Settings = json.RawMessage(`{}`)
			w.Env.ExecuteWorkflow(pipelinetest.Workflow(w, PipelineID, Run), p)
			if tc.configureFailure || tc.networkFailure {
				require.Error(t, w.Env.GetWorkflowError())
			} else {
				require.NoError(t, w.Env.GetWorkflowError())
			}
			require.Equal(t, tc.wantVerify, checks)
			require.Equal(t, tc.wantConfigure, configs)
			require.Equal(t, tc.wantNet, nets)
			require.Equal(t, tc.order, order)
		})
	}
}

package providerconfig

import (
	"context"
	"time"

	"github.com/graphene-ci/pipeline/pkg/pipeline"
	"github.com/graphene-ci/pipeline/pkg/wire"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/cloud"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

const (
	PipelineID        = "stroppy-provider-config"
	ConfigureActivity = "stroppy.provider.configure"
	VerifyActivity    = "stroppy.provider.config-verify"
)

func Run(ctx pipeline.Context, p spec.ProviderConfig) (spec.ProviderVerifyResult, error) {
	if ctx.Recording() {
		ctx.RecordActivity(ConfigureActivity, Configure)
		ctx.RecordActivity(VerifyActivity, verify)
		return spec.ProviderVerifyResult{}, nil
	}
	normalized, err := spec.NormalizeProviderConfig(p)
	if err != nil {
		return spec.ProviderVerifyResult{}, err
	}
	p = normalized
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{TaskQueue: wire.RunQueue(ctx.RunId()), StartToCloseTimeout: 3 * time.Minute, ScheduleToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3, InitialInterval: 2 * time.Second}})
	var out spec.ProviderVerifyResult
	if p.Action == "ensure" {
		if err := workflow.ExecuteActivity(actx, VerifyActivity, p).Get(ctx, &out); err != nil {
			return out, err
		}
		if !out.OK {
			return out, nil
		}
	}
	var configured spec.ProviderVerifyResult
	err = workflow.ExecuteActivity(actx, ConfigureActivity, p).Get(ctx, &configured)
	if p.Action == "delete" {
		out = configured
	}
	return out, err
}

func verify(ctx context.Context, p spec.ProviderConfig) (spec.ProviderVerifyResult, error) {
	client, err := cloud.For(p.Provider)
	if err != nil {
		return spec.ProviderVerifyResult{}, err
	}
	creds, err := cloud.Credentials(ctx, p.CredentialsSecret)
	if err != nil {
		return spec.ProviderVerifyResult{}, err
	}
	return client.Verify(ctx, p.Settings, creds, true)
}

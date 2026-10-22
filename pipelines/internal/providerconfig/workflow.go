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
	NetworkActivity   = "stroppy.provider.network"
)

func Run(ctx pipeline.Context, p spec.ProviderConfig) (spec.ProviderVerifyResult, error) {
	if ctx.Recording() {
		ctx.RecordActivity(ConfigureActivity, Configure)
		ctx.RecordActivity(VerifyActivity, verify)
		ctx.RecordActivity(NetworkActivity, ConfigureNetwork)
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
	// The profile network waits on the cloud: its own activity, with a
	// heartbeat, longer than the configuration writes.
	nctx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{TaskQueue: wire.RunQueue(ctx.RunId()), StartToCloseTimeout: 20 * time.Minute, HeartbeatTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3, InitialInterval: 5 * time.Second}})
	var configured spec.ProviderVerifyResult
	if p.Action == "delete" {
		// Network first: it references the ProviderConfig and its
		// credentials until the cloud has deleted it.
		if err := workflow.ExecuteActivity(nctx, NetworkActivity, p).Get(ctx, &configured); err != nil {
			return configured, err
		}
		err = workflow.ExecuteActivity(actx, ConfigureActivity, p).Get(ctx, &configured)
		return configured, err
	}
	if err := workflow.ExecuteActivity(actx, ConfigureActivity, p).Get(ctx, &configured); err != nil {
		return out, err
	}
	// The network needs the ProviderConfig written just above.
	err = workflow.ExecuteActivity(nctx, NetworkActivity, p).Get(ctx, &configured)
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

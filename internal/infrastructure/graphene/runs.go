package graphene

import (
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"

	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"
)

// Pipeline ids of the product pipelines (one image, four binaries).
const (
	PipelineRun            = "stroppy-run"
	PipelineSuite          = "stroppy-suite"
	PipelineProviderVerify = "stroppy-provider-verify"
	PipelineQuotas         = "stroppy-quotas"
	PipelineProviderConfig = "stroppy-provider-config"
)

// StartRun starts a run of a pipeline in the namespace of ctx.
func (c *Client) StartRun(ctx context.Context, runID, pipeline string, params any, labels map[string]string) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	_, err = c.Runs.StartRun(ctx, connect.NewRequest(&managementv1.StartRunRequest{
		RunId: runID, Pipeline: pipeline, Params: raw, Labels: labels,
	}))
	if err != nil {
		return fmt.Errorf("graphene: start %s/%s: %w", pipeline, runID, err)
	}
	return nil
}

// RunClose is how a run ended: the result it left and, when it did not
// complete, the failure. A failed run still carries what it collected —
// the pipeline puts the partial result into the failure's details.
func (c *Client) RunClose(ctx context.Context, runID string) (json.RawMessage, string, error) {
	resp, err := c.Runs.RunResult(ctx, connect.NewRequest(&managementv1.RunResultRequest{RunId: runID}))
	if err != nil {
		return nil, "", fmt.Errorf("graphene: result of %s: %w", runID, err)
	}
	return resp.Msg.GetResult(), resp.Msg.GetError(), nil
}

// RunResult blocks until the run finishes and decodes its result. A run
// that did not complete is an error here: its partial result is RunClose's.
func (c *Client) RunResult(ctx context.Context, runID string, out any) error {
	raw, failure, err := c.RunClose(ctx, runID)
	if err != nil {
		return err
	}
	if failure != "" {
		return fmt.Errorf("graphene: run %s did not complete: %s", runID, failure)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("graphene: result of %s: decode: %w", runID, err)
	}
	return nil
}

// RunOnce starts a run and waits for its result — the probe pipelines.
func (c *Client) RunOnce(ctx context.Context, runID, pipeline string, params, out any, labels map[string]string) error {
	if err := c.StartRun(ctx, runID, pipeline, params, labels); err != nil {
		return err
	}
	return c.RunResult(ctx, runID, out)
}

// RunStatus is the current status string of a run.
func (c *Client) RunStatus(ctx context.Context, runID string) (string, error) {
	resp, err := c.Runs.GetRun(ctx, connect.NewRequest(&managementv1.GetRunRequest{RunId: runID}))
	if err != nil {
		return "", fmt.Errorf("graphene: get %s: %w", runID, err)
	}
	return resp.Msg.GetStatus(), nil
}

// CancelRun asks for a graceful cancel.
func (c *Client) CancelRun(ctx context.Context, runID string) error {
	_, err := c.Runs.CancelRun(ctx, connect.NewRequest(&managementv1.CancelRunRequest{RunId: runID}))
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("graphene: cancel %s: %w", runID, err)
	}
	return nil
}

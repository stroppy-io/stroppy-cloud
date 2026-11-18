package run

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Submit retries the immutable request under the SAME Graphene identity.
// Graphene 0.2.24 makes this idempotent, including failed executions. Never
// retry a forgotten historical identity: Temporal deduplication has a finite
// retention horizon. Older ambiguous requests remain pending for investigation.
func Submit(ctx context.Context, g Graphene, createdAt time.Time, id, pipeline string, params any, labels map[string]string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if !createdAt.IsZero() && time.Since(createdAt) > 24*time.Hour {
		if _, err := g.RunStatus(ctx, id); err != nil {
			return fmt.Errorf("submission older than 24h requires reconciliation, automatic start disabled: %w", err)
		}
		return nil
	}
	canonical, err := canonicalParams(params)
	if err != nil {
		return err
	}
	return g.StartRun(ctx, id, pipeline, canonical, labels)
}

// canonicalParams is one byte form of the params, whoever holds them: the
// queue's spec is marshaled in Go, the projector's is the stored jsonb, and
// Graphene may tell a repeated start from another request by the bytes.
// Re-marshaling the decoded value sorts keys and settles whitespace and
// string escapes; numbers keep their digits (a uint64 seed does not fit a
// float64).
func canonicalParams(params any) (json.RawMessage, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("submission params: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("submission params: %w", err)
	}
	if raw, err = json.Marshal(v); err != nil {
		return nil, fmt.Errorf("submission params: %w", err)
	}
	return raw, nil
}

// submittedAt is when the run was first handed to Graphene: its admission
// (a queued run may wait for days before), else its creation.
func (r Run) submittedAt() time.Time {
	if r.AdmittedAt != nil {
		return *r.AdmittedAt
	}
	return r.CreatedAt
}

func runLabels(r Run) map[string]string {
	labels := make(map[string]string, len(r.Labels)+1)
	for k, v := range r.Labels {
		labels[k] = v
	}
	labels["stroppy.io/run"] = r.ID.String()
	return labels
}

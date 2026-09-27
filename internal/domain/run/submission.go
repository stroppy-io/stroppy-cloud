package run

import (
	"context"
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
	return g.StartRun(ctx, id, pipeline, params, labels)
}

func runLabels(r Run) map[string]string {
	labels := make(map[string]string, len(r.Labels)+1)
	for k, v := range r.Labels {
		labels[k] = v
	}
	labels["stroppy.io/run"] = r.ID.String()
	return labels
}

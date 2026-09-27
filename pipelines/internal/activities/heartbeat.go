package activities

import (
	"context"

	"go.temporal.io/sdk/activity"
)

type heartbeatContextKey struct{}

// Temporal retains the first heartbeat context for its batching timer. A short
// Docker/auth probe must not cancel the activity when that timer fires later.
// Bind once at the activity entrypoint and preserve that parent across probes.
func bindHeartbeatContext(ctx context.Context) context.Context {
	if ctx.Value(heartbeatContextKey{}) != nil {
		return ctx
	}
	return context.WithValue(ctx, heartbeatContextKey{}, ctx)
}

func heartbeatContext(ctx context.Context) context.Context {
	if parent, ok := ctx.Value(heartbeatContextKey{}).(context.Context); ok {
		return parent
	}
	return ctx
}

func recordHeartbeat(ctx context.Context, details ...any) {
	activity.RecordHeartbeat(heartbeatContext(ctx), details...)
}

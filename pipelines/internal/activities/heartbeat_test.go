package activities

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHeartbeatOutlivesProbeButHonorsActivityCancellation(t *testing.T) {
	parent, stop := context.WithTimeout(t.Context(), time.Minute)
	defer stop()
	activityCtx := bindHeartbeatContext(parent)
	probe, cancelProbe := context.WithCancel(activityCtx)
	// Readiness calls RunSegment with a shorter attempt context. Nested entry
	// binding must retain the outer activity, not the readiness attempt.
	probe = bindHeartbeatContext(probe)
	batched := heartbeatContext(probe)
	cancelProbe()
	require.ErrorIs(t, probe.Err(), context.Canceled)
	require.NoError(t, batched.Err(), "a deferred SDK heartbeat must survive probe completion")
	deadline, ok := batched.Deadline()
	require.True(t, ok)
	parentDeadline, _ := parent.Deadline()
	require.Equal(t, parentDeadline, deadline)
	stop()
	require.ErrorIs(t, batched.Err(), context.Canceled, "must not detach from activity cancellation")
}

func TestHeartbeatWithoutBindingRetainsOriginalContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, heartbeatContext(ctx).Err(), context.Canceled)
}

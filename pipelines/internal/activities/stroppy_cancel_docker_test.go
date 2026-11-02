package activities

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
)

// Explicitly opt in; uses a local Docker daemon only. A cancelled segment
// must not leave its load running.
func TestCancelledSegmentStopsStroppyContainer(t *testing.T) {
	image := os.Getenv("STROPPY_CANCEL_TEST_IMAGE")
	if image == "" {
		t.Skip("set STROPPY_CANCEL_TEST_IMAGE to an image with sleep, e.g. busybox:1.36")
	}
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	require.NoError(t, err)
	const name = "stroppy-00-cancel-docker"
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		require.NoError(t, cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true}))
		require.NoError(t, cli.Close())
	})
	// The pull is outside the deadline: the test cancels a running load.
	_, err = PullImage(t.Context(), PullImageRequest{Image: image})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err = runStroppy(ctx, stroppyContainer{Name: name, LogName: "cancel", Image: image, Args: []string{"sleep", "600"}, Dir: t.TempDir()})
	require.NoError(t, err)
	state, err := cli.ContainerInspect(t.Context(), name)
	require.NoError(t, err)
	require.False(t, state.State.Running, "the load outlived its activity")
}

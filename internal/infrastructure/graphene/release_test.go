package graphene

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"
	"github.com/graphene-ci/graphene/pkg/proto/management/v1/managementv1connect"
)

// slowStand answers a stand command only after the deletion it started,
// or fails at once with err.
type slowStand struct {
	managementv1connect.ResourcesAPIClient
	err error
}

func (s slowStand) Invoke(ctx context.Context, _ *connect.Request[managementv1.InvokeRequest]) (*connect.Response[managementv1.InvokeResponse], error) {
	if s.err != nil {
		return nil, s.err
	}
	<-ctx.Done()
	return nil, connect.NewError(connect.CodeDeadlineExceeded, ctx.Err())
}

func TestKeepReleaseUnderWayIsAccepted(t *testing.T) {
	prev := releaseAnswer
	releaseAnswer = 20 * time.Millisecond
	t.Cleanup(func() { releaseAnswer = prev })

	c := &Client{Resources: slowStand{}}
	if err := c.KeepRelease(context.Background(), "k8s.vpc/net"); err != nil {
		t.Fatalf("a release still deleting the stand is under way: %v", err)
	}
	c = &Client{Resources: slowStand{err: connect.NewError(connect.CodeUnavailable, errors.New("door down"))}}
	if err := c.KeepRelease(context.Background(), "k8s.vpc/net"); err == nil {
		t.Fatal("an unreachable stand must fail the release")
	}
}

package graphene

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"
	"github.com/graphene-ci/graphene/pkg/proto/management/v1/managementv1connect"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

type recoveryRunsAPI struct {
	managementv1connect.RunsAPIClient
	err error
}

func (g recoveryRunsAPI) GetRun(context.Context, *connect.Request[managementv1.GetRunRequest]) (*connect.Response[managementv1.GetRunResponse], error) {
	return nil, g.err
}
func (g recoveryRunsAPI) StartRun(context.Context, *connect.Request[managementv1.StartRunRequest]) (*connect.Response[managementv1.StartRunResponse], error) {
	return nil, g.err
}

func TestRunRecoveryErrorMeaning(t *testing.T) {
	for _, code := range []connect.Code{connect.CodeNotFound, connect.CodeUnavailable, connect.CodeDeadlineExceeded, connect.CodeCanceled, connect.CodePermissionDenied, connect.CodeInvalidArgument, connect.CodeAlreadyExists, connect.CodeInternal, connect.CodeUnknown} {
		t.Run(code.String(), func(t *testing.T) {
			c := &Client{Runs: recoveryRunsAPI{err: connect.NewError(code, errors.New("injected"))}}
			_, err := c.RunStatus(context.Background(), "r")
			if errors.Is(err, run.ErrRemoteNotFound) != (code == connect.CodeNotFound) || connect.CodeOf(err) != code {
				t.Fatalf("status lost its meaning: %v", err)
			}
			err = c.StartRun(context.Background(), "r", "stroppy-run", map[string]any{}, nil)
			rejected := code == connect.CodeInvalidArgument || code == connect.CodeAlreadyExists
			if errors.Is(err, run.ErrStartRejected) != rejected || connect.CodeOf(err) != code {
				t.Fatalf("submission outcome misclassified: %v", err)
			}
		})
	}
}

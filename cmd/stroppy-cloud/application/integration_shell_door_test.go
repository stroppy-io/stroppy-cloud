//go:build integration

package application

import (
	"context"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"
	"github.com/graphene-ci/graphene/pkg/proto/management/v1/managementv1connect"
)

// fakeAgents is Graphene's machine-shell door with an echo shell: input
// comes back as output, a resize as "[cols x rows]", a close ends it.
type fakeAgents struct {
	managementv1connect.UnimplementedAgentsAPIHandler
	mu       sync.Mutex
	sessions map[string]chan []byte
}

func newFakeAgents() *fakeAgents { return &fakeAgents{sessions: map[string]chan []byte{}} }

func (f *fakeAgents) Pty(ctx context.Context, req *connect.Request[managementv1.PtyRequest], stream *connect.ServerStream[managementv1.PtyChunk]) error {
	id := "pty-" + req.Msg.GetAgent()
	in := make(chan []byte, 16)
	f.mu.Lock()
	f.sessions[id] = in
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		delete(f.sessions, id)
		f.mu.Unlock()
	}()
	if err := stream.Send(&managementv1.PtyChunk{Body: &managementv1.PtyChunk_Opened_{Opened: &managementv1.PtyChunk_Opened{SessionId: id}}}); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case b := <-in:
			if b == nil {
				return stream.Send(&managementv1.PtyChunk{Body: &managementv1.PtyChunk_Closed_{Closed: &managementv1.PtyChunk_Closed{ExitCode: 0, Message: "bye"}}})
			}
			if err := stream.Send(&managementv1.PtyChunk{Body: &managementv1.PtyChunk_Data{Data: b}}); err != nil {
				return err
			}
		}
	}
}

func (f *fakeAgents) PtyInput(_ context.Context, req *connect.Request[managementv1.PtyInputRequest]) (*connect.Response[managementv1.PtyInputResponse], error) {
	f.mu.Lock()
	in, ok := f.sessions[req.Msg.GetSessionId()]
	f.mu.Unlock()
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("no session %s", req.Msg.GetSessionId()))
	}
	switch body := req.Msg.GetBody().(type) {
	case *managementv1.PtyInputRequest_Data:
		in <- body.Data
	case *managementv1.PtyInputRequest_Resize_:
		in <- fmt.Appendf(nil, "[%dx%d]", body.Resize.GetCols(), body.Resize.GetRows())
	case *managementv1.PtyInputRequest_Close:
		in <- nil
	}
	return connect.NewResponse(&managementv1.PtyInputResponse{}), nil
}

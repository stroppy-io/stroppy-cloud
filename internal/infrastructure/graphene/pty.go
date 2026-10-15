package graphene

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/shell"
)

// Pty is the machine-shell port over AgentsAPI.Pty/PtyInput.
type Pty struct{ c *Client }

// NewPty builds the port.
func NewPty(c *Client) *Pty { return &Pty{c: c} }

// Open starts the shell; the stream lives as long as ctx or Close.
func (p *Pty) Open(ctx context.Context, namespace, agent string, cols, rows uint32) (shell.Stream, error) {
	ctx, cancel := context.WithCancel(WithNamespace(ctx, namespace))
	st, err := p.c.Agents.Pty(ctx, connect.NewRequest(&managementv1.PtyRequest{Agent: agent, Cols: cols, Rows: rows}))
	if err != nil {
		cancel()
		return nil, err
	}
	return &ptyStream{st: st, cancel: cancel}, nil
}

// Input lands one frame on a live session.
func (p *Pty) Input(ctx context.Context, namespace, sessionID string, in shell.Input) error {
	req := &managementv1.PtyInputRequest{SessionId: sessionID}
	switch {
	case in.Close:
		req.Body = &managementv1.PtyInputRequest_Close{Close: true}
	case in.Cols > 0 && in.Rows > 0:
		req.Body = &managementv1.PtyInputRequest_Resize_{Resize: &managementv1.PtyInputRequest_Resize{Cols: in.Cols, Rows: in.Rows}}
	default:
		req.Body = &managementv1.PtyInputRequest_Data{Data: in.Data}
	}
	_, err := p.c.Agents.PtyInput(WithNamespace(ctx, namespace), connect.NewRequest(req))
	return err
}

type ptyStream struct {
	st     *connect.ServerStreamForClient[managementv1.PtyChunk]
	cancel context.CancelFunc
}

func (s *ptyStream) Recv() (shell.Chunk, error) {
	if !s.st.Receive() {
		if err := s.st.Err(); err != nil {
			return shell.Chunk{}, err
		}
		return shell.Chunk{}, errors.New("shell stream ended")
	}
	switch body := s.st.Msg().GetBody().(type) {
	case *managementv1.PtyChunk_Opened_:
		return shell.Chunk{SessionID: body.Opened.GetSessionId()}, nil
	case *managementv1.PtyChunk_Closed_:
		return shell.Chunk{Closed: &shell.Closed{ExitCode: body.Closed.GetExitCode(), Message: body.Closed.GetMessage()}}, nil
	case *managementv1.PtyChunk_Data:
		return shell.Chunk{Data: body.Data}, nil
	}
	return shell.Chunk{}, nil
}

func (s *ptyStream) Close() error {
	s.cancel()
	return s.st.Close()
}

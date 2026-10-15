// Package shell opens a terminal on a machine of a run (§16.6 /shell):
// Graphene's AgentsAPI.Pty behind a port. A shell exists while the stand
// does — a live run or a kept stand — and only members who may run tests
// open one; Graphene records each opening in the agent's history.
package shell

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Chunk is one piece of what the shell says: the session id first, then
// raw output, then how it ended.
type Chunk struct {
	SessionID string
	Data      []byte
	Closed    *Closed
}

// Closed is how a shell ended.
type Closed struct {
	ExitCode int32
	Message  string
}

// Input is one frame into a session: keystrokes, a resize or the close.
type Input struct {
	Data       []byte
	Cols, Rows uint32
	Close      bool
}

// Stream is an open shell's output; Close ends the shell.
type Stream interface {
	Recv() (Chunk, error)
	Close() error
}

// Pty is the port to Graphene's machine shells.
type Pty interface {
	Open(ctx context.Context, namespace, agent string, cols, rows uint32) (Stream, error)
	Input(ctx context.Context, namespace, sessionID string, in Input) error
}

// Service opens shells on run machines.
type Service struct{ pty Pty }

// NewService wires the port; nil answers unavailable.
func NewService(pty Pty) *Service { return &Service{pty: pty} }

// Session is an open shell with the namespace its input goes to.
type Session struct {
	Stream
	pty       Pty
	namespace string
}

// Input lands one frame on the session.
func (s Session) Input(ctx context.Context, sessionID string, in Input) error {
	return s.pty.Input(ctx, s.namespace, sessionID, in)
}

// Machines lists the machines of a run a shell can open on.
func Machines(r run.Run) []string {
	var rs spec.Run
	if json.Unmarshal(r.RunSpec, &rs) != nil {
		return nil
	}
	out := make([]string, 0, len(rs.Machines))
	for _, m := range rs.Machines {
		out = append(out, m.Name)
	}
	return out
}

// Open starts a shell on a machine of the run. The caller has checked the
// actor may operate the run.
func (s *Service) Open(ctx context.Context, r run.Run, machine string, cols, rows uint32) (Session, error) {
	if s.pty == nil {
		return Session{}, errs.New(errs.CodeUnavailable, "machine shells are not connected").Because("pty_streams_are_not_connected_yet")
	}
	if r.Status.Terminal() && !r.StandKept {
		return Session{}, errs.Conflict("the run's machines are gone").Because("shell_stand_gone")
	}
	if !slices.Contains(Machines(r), machine) {
		return Session{}, errs.Newf(errs.CodeNotFound, "machine %s is not in the run", machine).Because("shell_machine_unknown", "machine", machine)
	}
	if cols == 0 || rows == 0 {
		cols, rows = 120, 32
	}
	var rs spec.Run
	_ = json.Unmarshal(r.RunSpec, &rs) //nolint:errcheck // parsed above by Machines
	st, err := s.pty.Open(ctx, r.GrapheneNamespace, spec.AgentName(rs.RunID, machine), cols, rows)
	if err != nil {
		return Session{}, errs.Wrap(errs.CodeUnavailable, "open shell", err).Because("shell_open_failed", "machine", machine)
	}
	return Session{Stream: st, pty: s.pty, namespace: r.GrapheneNamespace}, nil
}

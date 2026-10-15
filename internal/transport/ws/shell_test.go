package ws

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/shell"
)

type okVerifier struct{ revoked *atomic.Bool }

func (v okVerifier) Verify(context.Context, string) (auth.Actor, error) {
	if v.revoked.Load() {
		return auth.Actor{}, errors.New("revoked")
	}
	return auth.Actor{}, nil
}

// openStream says its session id, then nothing until closed.
type openStream struct {
	opened bool
	done   chan struct{}
}

func (s *openStream) Recv() (shell.Chunk, error) {
	if !s.opened {
		s.opened = true
		return shell.Chunk{SessionID: "s1"}, nil
	}
	<-s.done
	return shell.Chunk{}, errors.New("closed")
}

func (s *openStream) Close() error {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	return nil
}

// shellSource opens silent sessions; MayShell follows allowed.
type shellSource struct{ allowed *atomic.Bool }

func (shellSource) Snapshot(context.Context, auth.Actor, string) (any, string, error) {
	return nil, "", nil
}
func (shellSource) Events(context.Context, auth.Actor, string, string) ([]any, string, error) {
	return nil, "", nil
}
func (shellSource) IsEvents(string) bool { return false }
func (shellSource) IsShell(topic string) bool {
	return strings.HasPrefix(topic, "agent.pty/")
}
func (shellSource) OpenShell(context.Context, auth.Actor, string, uint32, uint32) (shell.Session, error) {
	return shell.Session{Stream: &openStream{done: make(chan struct{})}}, nil
}
func (s shellSource) MayShell(context.Context, auth.Actor, string) error {
	if !s.allowed.Load() {
		return errs.Forbidden("requires role member").Because("requires_role", "role", "member")
	}
	return nil
}

func TestShellClosesWhenAccessIsLost(t *testing.T) {
	for _, tc := range []struct {
		name   string
		revoke func(role, session *atomic.Bool)
		want   string
	}{
		{"role lowered", func(role, _ *atomic.Bool) { role.Store(false) }, "requires_role"},
		{"session revoked", func(_, session *atomic.Bool) { session.Store(true) }, "shell_session_ended"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prev := shellRecheck
			shellRecheck = 50 * time.Millisecond
			t.Cleanup(func() { shellRecheck = prev })
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			allowed, revoked := &atomic.Bool{}, &atomic.Bool{}
			allowed.Store(true)
			h := New(okVerifier{revoked: revoked}, shellSource{allowed: allowed}, nil)
			h.Base = ctx
			server := httptest.NewServer(h)
			defer server.Close()
			conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"?token=t", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.CloseNow() //nolint:errcheck // best-effort on exit
			if err := wsjson.Write(ctx, conn, Frame{Type: "subscribe", SubID: "sh", Topic: "agent.pty/r/db-1"}); err != nil {
				t.Fatal(err)
			}
			next := func(typ string) Frame {
				for {
					var f Frame
					if err := wsjson.Read(ctx, conn, &f); err != nil {
						t.Fatalf("waiting for %s: %v", typ, err)
					}
					if f.Type == typ {
						return f
					}
				}
			}
			next("ack")
			tc.revoke(allowed, revoked)
			if f := next("error"); !strings.Contains(string(f.Payload), tc.want) {
				t.Fatalf("error frame %s", f.Payload)
			}
		})
	}
}

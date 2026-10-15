// Package ws is the one WebSocket of the API (§10, §16.11): a client
// subscribes to topics and receives events; a topic is polled from the
// server's own projection (no Graphene fan-out to browsers), so what the
// REST endpoints show and what the socket pushes never disagree.
//
// Wire: JSON frames {type: subscribe|unsubscribe|input|event|error|ack,
// topic, sub_id, cursor?, payload}. Topics: run.overview/{id},
// run.events/{id} (cursor = last event id), suite_run/{id},
// tenant.runs/{slug}, and the one two-way topic agent.pty/{run}/{machine}:
// subscribe carries {cols, rows}, events carry {session_id} | {data}
// (base64) | {closed: {exit_code, message}}, input frames carry
// {data} (text) | {cols, rows} | {close: true}.
//
// doc: github.com/coder/websocket — Accept, wsjson.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/shell"
)

// Verifier resolves a bearer token (header or ?token= for browsers).
type Verifier interface {
	Verify(ctx context.Context, token string) (auth.Actor, error)
}

// Source answers a topic: the payload plus a version that changes when
// the payload does (polling sends only on change). Events topics return
// the entries after a cursor and the new cursor.
type Source interface {
	Snapshot(ctx context.Context, actor auth.Actor, topic string) (payload any, version string, err error)
	Events(ctx context.Context, actor auth.Actor, topic string, cursor string) (events []any, next string, err error)
	// IsEvents reports whether a topic is an events stream (cursor-based).
	IsEvents(topic string) bool
}

// Shells is the optional two-way side of a Source: a terminal on a
// machine, opened by subscribing and fed by input frames. MayShell is
// asked again every shellRecheck while it is open: a revoked session, a
// lowered role or a gone stand closes it.
type Shells interface {
	IsShell(topic string) bool
	OpenShell(ctx context.Context, actor auth.Actor, topic string, cols, rows uint32) (shell.Session, error)
	MayShell(ctx context.Context, actor auth.Actor, topic string) error
}

// shellRecheck is how often an open terminal's access is checked again.
var shellRecheck = 15 * time.Second

// Frame is one message either way.
type Frame struct {
	Type    string          `json:"type"`
	Topic   string          `json:"topic,omitempty"`
	SubID   string          `json:"sub_id,omitempty"`
	Cursor  string          `json:"cursor,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Handler serves the socket.
type Handler struct {
	verifier Verifier
	source   Source
	log      *xlog.Logger
	// Poll is the topic refresh interval.
	Poll time.Duration
	// Origins are the allowed browser origins (empty = same host only).
	Origins []string
	// Base is the process context sessions derive from (shutdown ends them).
	Base context.Context
}

// New builds the handler.
func New(v Verifier, s Source, log *xlog.Logger) *Handler {
	return &Handler{verifier: v, source: s, log: log, Poll: 2 * time.Second}
}

const (
	maxSubscriptions = 32
	writeTimeout     = 10 * time.Second
)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	actor, err := h.verifier.Verify(r.Context(), token)
	if err != nil {
		http.Error(w, "token rejected", http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.Origins})
	if err != nil {
		return
	}
	defer conn.CloseNow() //nolint:errcheck // best-effort on exit
	// The request context ends with the hijack; the session lives on the
	// process context until the client goes away.
	base := h.Base
	if base == nil {
		base = context.Background()
	}
	ctx := auth.WithActor(base, actor)
	s := &session{h: h, conn: conn, actor: actor, token: token, subs: map[string]*subscription{}}
	s.run(ctx)
}

// session is one connection with its subscriptions.
type session struct {
	h     *Handler
	conn  *websocket.Conn
	actor auth.Actor
	// token is re-verified while a terminal is open.
	token string
	// ctx is the session context (frames after a subscription ended still
	// go through it).
	ctx  context.Context
	mu   sync.Mutex
	subs map[string]*subscription
	wmu  sync.Mutex
}

type subscription struct {
	id     string
	topic  string
	cursor string
	cancel context.CancelFunc
	// shell is set on a terminal subscription.
	shell     *shell.Session
	sessionID string
}

// shellSize is the subscribe payload of a terminal.
type shellSize struct {
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

// shellInput is an input frame's payload.
type shellInput struct {
	Data  string `json:"data,omitempty"`
	Cols  uint32 `json:"cols,omitempty"`
	Rows  uint32 `json:"rows,omitempty"`
	Close bool   `json:"close,omitempty"`
}

// shellEvent is what a terminal says.
type shellEvent struct {
	SessionID string       `json:"session_id,omitempty"`
	Data      []byte       `json:"data,omitempty"`
	Closed    *shellClosed `json:"closed,omitempty"`
}

type shellClosed struct {
	ExitCode int32  `json:"exit_code"`
	Message  string `json:"message,omitempty"`
}

func (s *session) run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.ctx = ctx
	for {
		var f Frame
		if err := wsjson.Read(ctx, s.conn, &f); err != nil {
			if s.h.log != nil {
				s.h.log.Debug("ws: read", xlog.Error("error", err))
			}
			return
		}
		switch f.Type {
		case "subscribe":
			s.subscribe(ctx, f)
		case "unsubscribe":
			s.unsubscribe(f.SubID)
		case "input":
			s.input(ctx, f)
		case "ping":
			s.send(ctx, Frame{Type: "pong", SubID: f.SubID})
		default:
			s.fail(ctx, f.SubID, errs.Invalid("unknown frame type "+f.Type))
		}
	}
}

func (s *session) subscribe(ctx context.Context, f Frame) {
	if f.SubID == "" || f.Topic == "" {
		s.fail(ctx, f.SubID, errs.Invalid("sub_id and topic are required").Because("sub_id_and_topic_are_required"))
		return
	}
	s.mu.Lock()
	if _, dup := s.subs[f.SubID]; dup || len(s.subs) >= maxSubscriptions {
		s.mu.Unlock()
		s.fail(ctx, f.SubID, errs.Conflict("subscription exists or too many").Because("subscription_exists_or_too_many"))
		return
	}
	// Cancelled by unsubscribe or the session end.
	sctx, cancel := context.WithCancel(ctx)
	sub := &subscription{id: f.SubID, topic: f.Topic, cursor: f.Cursor, cancel: cancel}
	s.subs[f.SubID] = sub
	s.mu.Unlock()
	if shells, ok := s.h.source.(Shells); ok && shells.IsShell(f.Topic) {
		s.openShell(ctx, sctx, shells, sub, f.Payload)
		return
	}
	// First answer synchronously: access errors reach the client at once.
	if err := s.tick(sctx, sub, true); err != nil {
		cancel()
		s.unsubscribe(sub.id)
		s.fail(ctx, sub.id, err)
		return
	}
	s.send(ctx, Frame{Type: "ack", SubID: sub.id, Topic: sub.topic})
	go func() {
		defer cancel()
		s.poll(sctx, sub)
	}()
}

// openShell starts the terminal: the opening chunk is read before the
// ack, so a refused shell answers as an error frame at once.
func (s *session) openShell(ctx, sctx context.Context, shells Shells, sub *subscription, payload json.RawMessage) {
	var size shellSize
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &size); err != nil {
			s.drop(ctx, sub, errs.Invalid("shell size is not {cols, rows}").Because("shell_size_invalid"))
			return
		}
	}
	sess, err := shells.OpenShell(sctx, s.actor, sub.topic, size.Cols, size.Rows)
	if err != nil {
		s.drop(ctx, sub, err)
		return
	}
	first, err := sess.Recv()
	if err != nil {
		_ = sess.Close()
		s.drop(ctx, sub, errs.Wrap(errs.CodeUnavailable, "open shell", err).Because("shell_open_failed", "machine", sub.topic))
		return
	}
	s.mu.Lock()
	sub.shell, sub.sessionID = &sess, first.SessionID
	s.mu.Unlock()
	s.send(ctx, Frame{Type: "ack", SubID: sub.id, Topic: sub.topic})
	_ = s.sendPayload(ctx, sub, "", shellEvent{SessionID: first.SessionID}) //nolint:errcheck // a payload of plain fields
	go s.watchShell(sctx, shells, sub)
	go func() {
		defer s.unsubscribe(sub.id)
		defer sess.Close()
		for {
			c, err := sess.Recv()
			if err != nil {
				if sctx.Err() == nil {
					_ = s.sendPayload(ctx, sub, "", shellEvent{Closed: &shellClosed{ExitCode: -1, Message: err.Error()}}) //nolint:errcheck // plain fields
				}
				return
			}
			ev := shellEvent{Data: c.Data}
			if c.Closed != nil {
				ev = shellEvent{Closed: &shellClosed{ExitCode: c.Closed.ExitCode, Message: c.Closed.Message}}
			}
			_ = s.sendPayload(ctx, sub, "", ev) //nolint:errcheck // plain fields
			if c.Closed != nil {
				return
			}
		}
	}()
}

// watchShell closes the terminal once the actor may no longer hold it.
func (s *session) watchShell(ctx context.Context, shells Shells, sub *subscription) {
	t := time.NewTicker(shellRecheck)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			err := shells.MayShell(ctx, s.actor, sub.topic)
			if err == nil {
				if _, verr := s.h.verifier.Verify(ctx, s.token); verr != nil {
					err = errs.New(errs.CodeUnauthenticated, "session ended").Because("shell_session_ended")
				}
			}
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				s.drop(s.ctx, sub, err)
				return
			}
		}
	}
}

// drop ends a subscription that failed to start.
func (s *session) drop(ctx context.Context, sub *subscription, err error) {
	s.unsubscribe(sub.id)
	s.fail(ctx, sub.id, err)
}

// input lands keystrokes, a resize or the close on a terminal.
func (s *session) input(ctx context.Context, f Frame) {
	s.mu.Lock()
	sub, ok := s.subs[f.SubID]
	var sess *shell.Session
	var id string
	if ok {
		sess, id = sub.shell, sub.sessionID
	}
	s.mu.Unlock()
	if sess == nil {
		s.fail(ctx, f.SubID, errs.NotFound("shell"))
		return
	}
	var in shellInput
	if err := json.Unmarshal(f.Payload, &in); err != nil {
		s.fail(ctx, f.SubID, errs.Invalid("input is not {data} | {cols, rows} | {close}").Because("shell_input_invalid"))
		return
	}
	if err := sess.Input(ctx, id, shell.Input{Data: []byte(in.Data), Cols: in.Cols, Rows: in.Rows, Close: in.Close}); err != nil {
		s.fail(ctx, f.SubID, errs.Wrap(errs.CodeUnavailable, "shell input", err).Because("shell_input_failed"))
	}
}

func (s *session) unsubscribe(id string) {
	s.mu.Lock()
	sub, ok := s.subs[id]
	if ok {
		delete(s.subs, id)
	}
	s.mu.Unlock()
	if ok {
		sub.cancel()
	}
}

func (s *session) poll(ctx context.Context, sub *subscription) {
	t := time.NewTicker(s.h.Poll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.tick(ctx, sub, false); err != nil {
				if ctx.Err() != nil {
					return
				}
				if s.h.log != nil {
					s.h.log.Warn("ws: topic failed", xlog.String("topic", sub.topic), xlog.Error("error", err))
				}
				s.unsubscribe(sub.id)
				s.fail(s.ctx, sub.id, err)
				return
			}
		}
	}
}

// tick sends what changed since the last tick; first forces a send.
func (s *session) tick(ctx context.Context, sub *subscription, first bool) error {
	if s.h.source.IsEvents(sub.topic) {
		events, next, err := s.h.source.Events(ctx, s.actor, sub.topic, sub.cursor)
		if err != nil {
			return err
		}
		for _, e := range events {
			if err := s.sendPayload(ctx, sub, next, e); err != nil {
				return err
			}
		}
		sub.cursor = next
		return nil
	}
	payload, version, err := s.h.source.Snapshot(ctx, s.actor, sub.topic)
	if err != nil {
		return err
	}
	if !first && version == sub.cursor {
		return nil
	}
	sub.cursor = version
	return s.sendPayload(ctx, sub, version, payload)
}

func (s *session) sendPayload(ctx context.Context, sub *subscription, cursor string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	s.send(ctx, Frame{Type: "event", Topic: sub.topic, SubID: sub.id, Cursor: cursor, Payload: raw})
	return nil
}

func (s *session) fail(ctx context.Context, subID string, err error) {
	code := errs.CodeOf(err)
	detail := err.Error()
	var domain *errs.Error
	if errors.As(err, &domain) {
		detail = domain.Detail
	}
	reason, params := errs.ReasonOf(err)
	raw, _ := json.Marshal(map[string]any{"code": string(code), "detail": detail, "reason": reason, "params": params}) //nolint:errcheck // plain values
	s.send(ctx, Frame{Type: "error", SubID: subID, Payload: raw})
}

func (s *session) send(ctx context.Context, f Frame) {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	if ctx.Err() != nil {
		return
	}
	// Unsubscribing may cancel ctx during a write. The websocket library closes
	// the entire connection on write cancellation, so use the session lifetime.
	wctx, cancel := context.WithTimeout(s.ctx, writeTimeout)
	defer cancel()
	if err := wsjson.Write(wctx, s.conn, f); err != nil && s.h.log != nil {
		s.h.log.Debug("ws: write", xlog.Error("error", err))
	}
}

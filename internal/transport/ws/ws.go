// Package ws is the one WebSocket of the API (§10, §16.11): a client
// subscribes to topics and receives events; a topic is polled from the
// server's own projection (no Graphene fan-out to browsers), so what the
// REST endpoints show and what the socket pushes never disagree.
//
// Wire: JSON frames {type: subscribe|unsubscribe|event|error|ack, topic,
// sub_id, cursor?, payload}. Topics: run.overview/{id}, run.events/{id}
// (cursor = last event id), suite_run/{id}, tenant.runs/{slug}.
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
	s := &session{h: h, conn: conn, actor: actor, subs: map[string]*subscription{}}
	s.run(ctx)
}

// session is one connection with its subscriptions.
type session struct {
	h     *Handler
	conn  *websocket.Conn
	actor auth.Actor
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
		case "ping":
			s.send(ctx, Frame{Type: "pong", SubID: f.SubID})
		default:
			s.fail(ctx, f.SubID, errs.Invalid("unknown frame type "+f.Type))
		}
	}
}

func (s *session) subscribe(ctx context.Context, f Frame) {
	if f.SubID == "" || f.Topic == "" {
		s.fail(ctx, f.SubID, errs.Invalid("sub_id and topic are required"))
		return
	}
	s.mu.Lock()
	if _, dup := s.subs[f.SubID]; dup || len(s.subs) >= maxSubscriptions {
		s.mu.Unlock()
		s.fail(ctx, f.SubID, errs.Conflict("subscription exists or too many"))
		return
	}
	// Cancelled by unsubscribe or the session end.
	sctx, cancel := context.WithCancel(ctx)
	sub := &subscription{id: f.SubID, topic: f.Topic, cursor: f.Cursor, cancel: cancel}
	s.subs[f.SubID] = sub
	s.mu.Unlock()
	// First answer synchronously: access errors reach the client at once.
	if err := s.tick(sctx, sub, true); err != nil {
		s.unsubscribe(sub.id)
		s.fail(ctx, sub.id, err)
		return
	}
	s.send(ctx, Frame{Type: "ack", SubID: sub.id, Topic: sub.topic})
	go s.poll(sctx, sub)
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
	raw, _ := json.Marshal(map[string]string{"code": string(code), "detail": detail}) //nolint:errcheck // map
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

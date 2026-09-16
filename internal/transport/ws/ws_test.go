package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestCanceledSubscriptionPreservesConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer conn.CloseNow() //nolint:errcheck // best-effort on exit
		s := &session{h: &Handler{}, conn: conn, ctx: ctx}
		subCtx, unsubscribe := context.WithCancel(ctx)
		unsubscribe()
		s.send(subCtx, Frame{Type: "event", SubID: "canceled"})
		s.send(ctx, Frame{Type: "pong", SubID: "active"})
	}))
	defer server.Close()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow() //nolint:errcheck // best-effort on exit
	var got Frame
	if err := wsjson.Read(ctx, conn, &got); err != nil {
		t.Fatalf("connection must survive subscription cancellation: %v", err)
	}
	if got.Type != "pong" || got.SubID != "active" {
		t.Fatalf("unexpected frame: %+v", got)
	}
}

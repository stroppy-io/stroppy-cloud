//go:build integration

package application

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/stroppy-io/stroppy-cloud/internal/transport/ws"
)

// wsClient dials the socket with a bearer in the query (browser style)
// and reads frames with a deadline.
type wsClient struct {
	t    *testing.T
	conn *websocket.Conn
	ctx  context.Context
}

func dialWS(t *testing.T, e *e2e, token string) *wsClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	t.Cleanup(cancel)
	url := "ws" + strings.TrimPrefix(e.ts.URL, "http") + "/api/v1/ws?token=" + token
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return &wsClient{t: t, conn: conn, ctx: ctx}
}

func (c *wsClient) send(f ws.Frame) {
	c.t.Helper()
	if err := wsjson.Write(c.ctx, c.conn, f); err != nil {
		c.t.Fatalf("write: %v", err)
	}
}

// next reads frames until one matches (type and sub id), within d.
func (c *wsClient) next(d time.Duration, typ, subID string) ws.Frame {
	c.t.Helper()
	// The read context is not cancelled after the read: coder/websocket
	// closes the connection when the read context ends.
	ctx, cancel := context.WithTimeout(c.ctx, d)
	_ = cancel
	for {
		var f ws.Frame
		if err := wsjson.Read(ctx, c.conn, &f); err != nil {
			c.t.Fatalf("waiting for %s/%s: %v", typ, subID, err)
		}
		if f.Type == typ && (subID == "" || f.SubID == subID) {
			return f
		}
	}
}

func TestE2EWebSocket(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, tn := runFixture(t, e)

	t.Run("rejects missing and bad tokens", func(t *testing.T) {
		r := e.req(http.MethodGet, "/api/v1/ws", nil, "")
		if r.Status != http.StatusUnauthorized {
			t.Fatalf("no token: %d", r.Status)
		}
		r = e.req(http.MethodGet, "/api/v1/ws?token=stc_nope", nil, "")
		if r.Status != http.StatusUnauthorized {
			t.Fatalf("bad token: %d", r.Status)
		}
	})

	// The pipeline pauses right after its first phase started.
	e.graphene.hold("phase.started")
	var launched runView
	e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{"name": "live"}, tok), http.StatusCreated, &launched)
	e.graphene.hold("")

	t.Run("overview and events follow the projection", func(t *testing.T) {
		c := dialWS(t, e, tok)
		c.send(ws.Frame{Type: "subscribe", SubID: "ov", Topic: "run.overview/" + launched.ID})
		first := c.next(5*time.Second, "event", "ov")
		var ov struct {
			Status string `json:"status"`
			Phase  string `json:"phase"`
		}
		_ = json.Unmarshal(first.Payload, &ov)
		if ov.Status != "pending" {
			t.Fatalf("first overview %+v", ov)
		}
		c.next(5*time.Second, "ack", "ov")
		c.send(ws.Frame{Type: "subscribe", SubID: "ev", Topic: "run.events/" + launched.ID})
		c.next(5*time.Second, "ack", "ev")

		e.app.services.Projector.Tick(e.ctx)
		deadline := time.Now().Add(10 * time.Second)
		sawRunning, sawEvent := false, false
		for time.Now().Before(deadline) && !(sawRunning && sawEvent) {
			f := c.next(5*time.Second, "event", "")
			switch f.SubID {
			case "ov":
				_ = json.Unmarshal(f.Payload, &ov)
				if ov.Status == "running" && ov.Phase == "provisioning" {
					sawRunning = true
				}
			case "ev":
				var evt struct {
					Kind string `json:"kind"`
				}
				_ = json.Unmarshal(f.Payload, &evt)
				if evt.Kind == "phase.started" {
					sawEvent = true
				}
				if f.Cursor == "" {
					t.Fatalf("event without cursor %+v", f)
				}
			}
		}
		if !sawRunning || !sawEvent {
			t.Fatalf("running=%v event=%v", sawRunning, sawEvent)
		}
		c.send(ws.Frame{Type: "unsubscribe", SubID: "ev"})
		c.send(ws.Frame{Type: "ping", SubID: "p"})
		c.next(5*time.Second, "pong", "p")

		e.graphene.release(launched.ID)
		for {
			f := c.next(10*time.Second, "event", "ov")
			_ = json.Unmarshal(f.Payload, &ov)
			if ov.Status == "completed" {
				break
			}
		}
	})

	t.Run("tenant.runs, suite_run, access and unknown topics", func(t *testing.T) {
		c := dialWS(t, e, tok)
		c.send(ws.Frame{Type: "subscribe", SubID: "tr", Topic: "tenant.runs/" + tn.Slug})
		f := c.next(5*time.Second, "event", "tr")
		var list struct {
			Data []map[string]any `json:"data"`
		}
		_ = json.Unmarshal(f.Payload, &list)
		if len(list.Data) != 0 {
			t.Fatalf("live list %+v", list)
		}
		c.send(ws.Frame{Type: "subscribe", SubID: "bad", Topic: "agent.pty/" + launched.ID + "/db-1"})
		errFrame := c.next(5*time.Second, "error", "bad")
		if !strings.Contains(string(errFrame.Payload), "unavailable") {
			t.Fatalf("pty error %s", errFrame.Payload)
		}
		c.send(ws.Frame{Type: "subscribe", SubID: "nope", Topic: "run.overview/00000000-0000-0000-0000-000000000000"})
		errFrame = c.next(5*time.Second, "error", "nope")
		if !strings.Contains(string(errFrame.Payload), "not_found") {
			t.Fatalf("missing run error %s", errFrame.Payload)
		}

		// Another tenant's member cannot see the run.
		stranger := e.person(slug("stranger")+"@example.com", "Stranger")
		other := e.tenant(stranger, slug("other"))
		stok := e.token(stranger, other)
		sc := dialWS(t, e, stok)
		sc.send(ws.Frame{Type: "subscribe", SubID: "x", Topic: "run.overview/" + launched.ID})
		errFrame = sc.next(5*time.Second, "error", "x")
		if !strings.Contains(string(errFrame.Payload), "not_found") {
			t.Fatalf("stranger error %s", errFrame.Payload)
		}
	})
}

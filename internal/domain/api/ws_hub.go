package api

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap/zapcore"
)

// wsMessage is the envelope sent to WebSocket clients.
type wsMessage struct {
	Type    string `json:"type"` // "log", "report", "agent_log"
	RunID   string `json:"run_id,omitempty"`
	NodeID  string `json:"node_id,omitempty"`
	Payload any    `json:"payload"`
}

type wsClient struct {
	conn        *websocket.Conn
	filterRunID string // empty = receive all
}

// wsHub manages WebSocket clients and broadcasts messages to them.
// It also implements dag.LogSink so executor logs stream directly to UI.
type wsHub struct {
	mu      sync.RWMutex
	clients []*wsClient
}

func newWSHub() *wsHub {
	return &wsHub{}
}

func (h *wsHub) addClient(conn *websocket.Conn, filterRunID string) {
	client := &wsClient{conn: conn, filterRunID: filterRunID}

	h.mu.Lock()
	h.clients = append(h.clients, client)
	h.mu.Unlock()

	// Read pump: just drain client messages and detect disconnect.
	go func() {
		defer h.removeClient(client)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (h *wsHub) removeClient(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, cl := range h.clients {
		if cl == c {
			h.clients = append(h.clients[:i], h.clients[i+1:]...)
			c.conn.Close()
			return
		}
	}
}

func (h *wsHub) broadcast(msg wsMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, c := range h.clients {
		if c.filterRunID != "" && msg.RunID != "" && c.filterRunID != msg.RunID {
			continue
		}
		c.conn.WriteMessage(websocket.TextMessage, data)
	}
}

// WriteLog implements dag.LogSink — streams executor node logs to WebSocket clients.
func (h *wsHub) WriteLog(_ context.Context, executionID string, nodeID string, entry zapcore.Entry, fields []zapcore.Field) {
	h.broadcast(wsMessage{
		Type:   "log",
		RunID:  executionID,
		NodeID: nodeID,
		Payload: map[string]any{
			"level":   entry.Level.String(),
			"message": entry.Message,
			"time":    entry.Time,
		},
	})
}

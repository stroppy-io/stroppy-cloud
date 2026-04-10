package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// AgentServer polls the orchestration server for commands and executes them.
// It does NOT listen for inbound connections — all communication is agent→server.
type AgentServer struct {
	executor   *Executor
	serverAddr string
	machineID  string
	token      string // JWT token for authenticating with the server
	httpClient *http.Client

	// Log dedup + batching.
	logMu      sync.Mutex
	logBuf     []LogLine
	lastLine   string // previous line text for dedup
	repeatCnt  int    // consecutive repeats of lastLine
	lastAction string
	lastCmdID  string
	logTicker  *time.Ticker
	logDone    chan struct{}
}

// NewAgentServer creates a new polling-based agent.
func NewAgentServer(serverAddr string, machineID string, _ int) *AgentServer {
	s := &AgentServer{
		executor:   NewExecutor(),
		serverAddr: serverAddr,
		machineID:  machineID,
		httpClient: &http.Client{Timeout: 90 * time.Second},
		logTicker:  time.NewTicker(200 * time.Millisecond),
		logDone:    make(chan struct{}),
	}

	s.executor.SetLogCallback(func(commandID, action, line, stream string) {
		s.bufferLogLine(commandID, action, line, stream)
	})

	// Background flusher — sends batched logs every 200ms.
	go s.logFlusher()

	return s
}

// bufferLogLine deduplicates repeated lines and buffers for batch send.
func (s *AgentServer) bufferLogLine(commandID, action, line, stream string) {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if line == s.lastLine && action == s.lastAction {
		s.repeatCnt++
		return
	}

	// Flush previous repeated line.
	if s.repeatCnt > 0 {
		s.logBuf = append(s.logBuf, LogLine{
			CommandID: s.lastCmdID,
			MachineID: s.machineID,
			Action:    s.lastAction,
			Line:      fmt.Sprintf("[repeated %d times] %s", s.repeatCnt, s.lastLine),
			Stream:    stream,
		})
	}

	s.lastLine = line
	s.lastAction = action
	s.lastCmdID = commandID
	s.repeatCnt = 0

	s.logBuf = append(s.logBuf, LogLine{
		CommandID: commandID,
		MachineID: s.machineID,
		Action:    action,
		Line:      line,
		Stream:    stream,
	})
}

// logFlusher runs in background, sends batched logs to server every 200ms.
func (s *AgentServer) logFlusher() {
	for {
		select {
		case <-s.logTicker.C:
			s.flushLogs()
		case <-s.logDone:
			s.flushLogs() // final flush
			return
		}
	}
}

func (s *AgentServer) flushLogs() {
	s.logMu.Lock()
	if len(s.logBuf) == 0 && s.repeatCnt == 0 {
		s.logMu.Unlock()
		return
	}
	// Flush any pending repeated line.
	if s.repeatCnt > 0 {
		s.logBuf = append(s.logBuf, LogLine{
			CommandID: s.lastCmdID,
			MachineID: s.machineID,
			Action:    s.lastAction,
			Line:      fmt.Sprintf("[repeated %d times] %s", s.repeatCnt, s.lastLine),
			Stream:    "stdout",
		})
		s.repeatCnt = 0
	}
	batch := s.logBuf
	s.logBuf = nil
	s.logMu.Unlock()

	if s.serverAddr == "" || len(batch) == 0 {
		return
	}

	data, err := json.Marshal(batch)
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, s.serverAddr+"/api/agent/logs-batch", bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	s.setAuth(req)
	resp, err := s.httpClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// SetToken sets the JWT token for server authentication.
func (s *AgentServer) SetToken(token string) {
	s.token = token
}

// setAuth adds Authorization header if token is set.
func (s *AgentServer) setAuth(req *http.Request) {
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
}

// Run starts the poll loop. Blocks until ctx is cancelled.
func (s *AgentServer) Run(ctx context.Context) error {
	log.Printf("agent: starting poll loop (server=%s, machine=%s)", s.serverAddr, s.machineID)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		cmd, err := s.poll(ctx)
		if err != nil {
			log.Printf("agent: poll error: %v (retrying in 2s)", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(2 * time.Second):
			}
			continue
		}

		if cmd == nil {
			// No pending command — long-poll returned empty. Loop immediately.
			continue
		}

		// Execute command and report back.
		report := s.executor.Run(ctx, *cmd)
		report.CommandID = cmd.ID

		if err := s.sendReport(ctx, report); err != nil {
			log.Printf("agent: failed to send report: %v", err)
		}
	}
}

// poll calls POST /api/agent/poll on the server. Returns nil if no command pending.
func (s *AgentServer) poll(ctx context.Context) (*Command, error) {
	body := struct {
		MachineID string `json:"machine_id"`
	}{MachineID: s.machineID}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.serverAddr+"/api/agent/poll", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	s.setAuth(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil // no command pending
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("poll: server returned %d: %s", resp.StatusCode, string(respBody))
	}

	var cmd Command
	if err := json.NewDecoder(resp.Body).Decode(&cmd); err != nil {
		return nil, fmt.Errorf("poll: decode command: %w", err)
	}

	return &cmd, nil
}

// sendReport posts the command result back to the server.
func (s *AgentServer) sendReport(ctx context.Context, report Report) error {
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.serverAddr+"/api/agent/report", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	s.setAuth(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// StopLogFlusher stops the background log flusher (call on shutdown).
func (s *AgentServer) StopLogFlusher() {
	s.logTicker.Stop()
	close(s.logDone)
}

// Register calls back to the orchestration server to announce this agent.
func (s *AgentServer) Register() error {
	if s.serverAddr == "" {
		return nil
	}

	body := struct {
		MachineID string `json:"machine_id"`
	}{MachineID: s.machineID}

	data, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, s.serverAddr+"/api/agent/register", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("agent register: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	s.setAuth(req)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("agent register: %w", err)
	}
	resp.Body.Close()

	log.Printf("agent: registered with server %s (machine_id=%s)", s.serverAddr, s.machineID)
	return nil
}

// Executor returns the underlying executor for shutdown management.
func (s *AgentServer) Executor() *Executor { return s.executor }

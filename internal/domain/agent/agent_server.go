package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

// AgentServer is the HTTP server running on agent machines.
type AgentServer struct {
	executor   *Executor
	serverAddr string
	machineID  string
	port       int
}

// NewAgentServer creates a new agent HTTP server.
func NewAgentServer(serverAddr string, machineID string, port int) *AgentServer {
	s := &AgentServer{
		executor:   NewExecutor(),
		serverAddr: serverAddr,
		machineID:  machineID,
		port:       port,
	}

	// Wire real-time log streaming: every shell output line is POSTed to the
	// orchestration server which forwards it to WebSocket clients and VictoriaLogs.
	s.executor.SetLogCallback(func(commandID, line, stream string) {
		s.sendLogLine(commandID, line, stream)
	})

	return s
}

// sendLogLine posts a single log line to the orchestration server.
// It is fire-and-forget so shell execution is never blocked.
func (s *AgentServer) sendLogLine(commandID, line, stream string) {
	if s.serverAddr == "" {
		return
	}
	ll := LogLine{
		CommandID: commandID,
		MachineID: s.machineID,
		Line:      line,
		Stream:    stream,
	}
	data, err := json.Marshal(ll)
	if err != nil {
		return
	}
	go func() {
		resp, err := http.Post(s.serverAddr+"/api/agent/log", "application/json", bytes.NewReader(data))
		if err == nil {
			resp.Body.Close()
		}
	}()
}

// Router returns the chi router with agent endpoints.
func (s *AgentServer) Router() http.Handler {
	r := chi.NewRouter()
	r.Get("/health", s.health)
	r.Post("/execute", s.execute)
	return r
}

func (s *AgentServer) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "ok",
		"machine_id": s.machineID,
	})
}

func (s *AgentServer) execute(w http.ResponseWriter, r *http.Request) {
	var cmd Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	report := s.executor.Run(r.Context(), cmd)

	w.Header().Set("Content-Type", "application/json")
	if report.Status == ReportFailed {
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(report)
}

// Register calls back to the orchestration server to announce this agent.
// It posts a registration request with the machine ID, host, and port.
// Executor returns the underlying executor for shutdown management.
func (s *AgentServer) Executor() *Executor { return s.executor }

func (s *AgentServer) Register() error {
	if s.serverAddr == "" {
		return nil // no server to register with
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("agent register: get hostname: %w", err)
	}

	body := struct {
		MachineID string `json:"machine_id"`
		Host      string `json:"host"`
		Port      int    `json:"port"`
	}{
		MachineID: s.machineID,
		Host:      hostname,
		Port:      s.port,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("agent register: marshal: %w", err)
	}

	url := s.serverAddr + "/api/agent/register"
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("agent register: POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("agent register: server returned %s", resp.Status)
	}

	log.Printf("agent: registered with server %s (machine_id=%s)", s.serverAddr, s.machineID)
	return nil
}

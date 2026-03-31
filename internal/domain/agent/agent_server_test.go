package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	srv := NewAgentServer("http://server:8080", "test-machine", 9090)
	router := srv.Router()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", resp["status"])
	}
	if resp["machine_id"] != "test-machine" {
		t.Errorf("expected machine_id=test-machine, got %q", resp["machine_id"])
	}
}

func TestHealthEndpoint_ContentType(t *testing.T) {
	srv := NewAgentServer("", "m1", 9090)
	router := srv.Router()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestExecuteEndpoint_InvalidJSON(t *testing.T) {
	srv := NewAgentServer("", "m1", 9090)
	router := srv.Router()

	req := httptest.NewRequest(http.MethodPost, "/execute", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestExecuteEndpoint_UnknownAction(t *testing.T) {
	srv := NewAgentServer("", "m1", 9090)
	router := srv.Router()

	cmd := Command{
		ID:     "cmd-1",
		Action: "nonexistent_action",
	}
	body, _ := json.Marshal(cmd)

	req := httptest.NewRequest(http.MethodPost, "/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// The executor should return a failed report for unknown actions.
	// The server returns 500 for failed reports.
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for unknown action, got %d", w.Code)
	}

	var report Report
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.Status != ReportFailed {
		t.Errorf("expected status failed, got %q", report.Status)
	}
	if report.CommandID != "cmd-1" {
		t.Errorf("expected command_id cmd-1, got %q", report.CommandID)
	}
}

func TestExecuteEndpoint_ReturnsJSON(t *testing.T) {
	srv := NewAgentServer("", "m1", 9090)
	router := srv.Router()

	cmd := Command{ID: "cmd-2", Action: "nonexistent"}
	body, _ := json.Marshal(cmd)

	req := httptest.NewRequest(http.MethodPost, "/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestTargetAddr(t *testing.T) {
	target := Target{
		ID:        "vm-1",
		Host:      "10.0.0.1",
		AgentPort: 9090,
	}
	expected := "http://10.0.0.1:9090"
	if got := target.Addr(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestTargetAddr_DifferentPort(t *testing.T) {
	target := Target{
		ID:        "vm-2",
		Host:      "myhost",
		AgentPort: 8888,
	}
	expected := "http://myhost:8888"
	if got := target.Addr(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

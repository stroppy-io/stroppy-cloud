package victoria

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LogsClient sends log entries to VictoriaLogs via the JSON-line ingestion API.
type LogsClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewLogsClient creates a VictoriaLogs client for log ingestion and querying.
func NewLogsClient(baseURL string) *LogsClient {
	return &LogsClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// logEntry is the JSON-line format expected by VictoriaLogs /insert/jsonline.
type logEntry struct {
	Msg       string `json:"_msg"`
	Time      string `json:"_time"`
	MachineID string `json:"machine_id,omitempty"`
	CommandID string `json:"command_id,omitempty"`
	RunID     string `json:"run_id,omitempty"`
	Stream    string `json:"stream,omitempty"`
}

// BaseURL returns the configured VictoriaLogs base URL (for proxied queries).
func (c *LogsClient) BaseURL() string { return c.baseURL }

// Ingest sends a single log line to VictoriaLogs.
// It is safe to call from multiple goroutines.
func (c *LogsClient) Ingest(machineID, commandID, runID, stream, line string) error {
	entry := logEntry{
		Msg:       line,
		Time:      time.Now().UTC().Format(time.RFC3339Nano),
		MachineID: machineID,
		CommandID: commandID,
		RunID:     runID,
		Stream:    stream,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("vlogs marshal: %w", err)
	}
	// JSON-line requires a trailing newline.
	data = append(data, '\n')

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/insert/jsonline", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("vlogs build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/stream+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vlogs post: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("vlogs: status %d", resp.StatusCode)
	}
	return nil
}

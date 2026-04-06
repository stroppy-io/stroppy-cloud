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
	token      string // bearer token for vmauth (empty = no auth)
	httpClient *http.Client
}

// NewLogsClient creates a VictoriaLogs client for log ingestion and querying.
// token may be empty to disable bearer auth.
func NewLogsClient(baseURL, token string) *LogsClient {
	return &LogsClient{
		baseURL:    baseURL,
		token:      token,
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

// IngestWithAccount sends a single log line to VictoriaLogs for a specific tenant account.
func (c *LogsClient) IngestWithAccount(accountID int32, machineID, commandID, runID, stream, line string) error {
	return c.ingest(accountID, machineID, commandID, runID, stream, line)
}

// Ingest sends a single log line to VictoriaLogs (default account 0).
// It is safe to call from multiple goroutines.
func (c *LogsClient) Ingest(machineID, commandID, runID, stream, line string) error {
	return c.ingest(0, machineID, commandID, runID, stream, line)
}

func (c *LogsClient) ingest(accountID int32, machineID, commandID, runID, stream, line string) error {
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
	if accountID > 0 {
		req.Header.Set("AccountID", fmt.Sprintf("%d", accountID))
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

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

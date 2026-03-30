package victoria

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client queries a VictoriaMetrics (or Prometheus-compatible) API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a VictoriaMetrics client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// --- Prometheus-compatible response types ---

// QueryResult is the top-level response from /api/v1/query or /api/v1/query_range.
type QueryResult struct {
	Status string     `json:"status"`
	Data   ResultData `json:"data"`
}

// ResultData holds the result type and series.
type ResultData struct {
	ResultType string   `json:"resultType"`
	Result     []Series `json:"result"`
}

// Series is a single time series with labels and values.
type Series struct {
	Metric map[string]string `json:"metric"`
	Values []SamplePair      `json:"values,omitempty"` // query_range
	Value  []any             `json:"value,omitempty"`  // instant query
}

// SamplePair is a [timestamp, value] pair.
type SamplePair [2]any

// Timestamp returns the unix timestamp.
func (s SamplePair) Timestamp() float64 {
	if f, ok := s[0].(float64); ok {
		return f
	}
	return 0
}

// Val returns the string-encoded value.
func (s SamplePair) Val() string {
	if v, ok := s[1].(string); ok {
		return v
	}
	return ""
}

// QueryInstant executes a PromQL instant query.
func (c *Client) QueryInstant(ctx context.Context, query string, ts time.Time) (*QueryResult, error) {
	params := url.Values{
		"query": {query},
		"time":  {fmt.Sprintf("%d", ts.Unix())},
	}
	return c.doQuery(ctx, "/api/v1/query", params)
}

// QueryRange executes a PromQL range query.
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResult, error) {
	params := url.Values{
		"query": {query},
		"start": {fmt.Sprintf("%d", start.Unix())},
		"end":   {fmt.Sprintf("%d", end.Unix())},
		"step":  {step.String()},
	}
	return c.doQuery(ctx, "/api/v1/query_range", params)
}

func (c *Client) doQuery(ctx context.Context, path string, params url.Values) (*QueryResult, error) {
	reqURL := fmt.Sprintf("%s%s?%s", c.baseURL, path, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("victoria: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("victoria: query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("victoria: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("victoria: status %d: %s", resp.StatusCode, body)
	}

	var result QueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("victoria: decode: %w", err)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("victoria: query failed: %s", body)
	}

	return &result, nil
}

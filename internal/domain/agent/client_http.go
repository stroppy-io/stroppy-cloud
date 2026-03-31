package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
)

const (
	healthTimeout    = 60 * time.Second // max time to wait for an agent to become healthy
	healthInterval   = 2 * time.Second  // initial polling interval
	healthMaxBackoff = 16 * time.Second // cap on exponential backoff
)

// HTTPClient sends commands to agent HTTP servers.
type HTTPClient struct {
	httpClient *http.Client
}

// NewHTTPClient creates a client for communicating with remote agents.
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		httpClient: &http.Client{Timeout: 10 * time.Minute}, // long timeout for installs
	}
}

// Send dispatches a command to a single agent and waits for the report.
// It first waits for the agent to become healthy, then POSTs the command.
func (c *HTTPClient) Send(nc *dag.NodeContext, target Target, cmd Command) error {
	log := nc.Log().With(
		zap.String("target", target.ID),
		zap.String("action", string(cmd.Action)),
	)

	log.Info("waiting for agent to become healthy", zap.String("addr", target.Addr()))
	if err := c.waitForAgent(nc, target); err != nil {
		return fmt.Errorf("agent %s not healthy: %w", target.ID, err)
	}
	log.Info("agent is healthy, sending command")

	body, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("agent: marshal command: %w", err)
	}

	url := target.Addr() + "/execute"
	req, err := http.NewRequestWithContext(nc, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("agent: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("agent: POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("agent: read response from %s: %w", target.ID, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent %s returned HTTP %d: %s", target.ID, resp.StatusCode, string(respBody))
	}

	var report Report
	if err := json.Unmarshal(respBody, &report); err != nil {
		return fmt.Errorf("agent: decode report from %s: %w", target.ID, err)
	}

	if report.Status == ReportFailed {
		log.Error("command failed on agent", zap.String("error", report.Error))
		return fmt.Errorf("agent %s: command %s failed: %s", target.ID, cmd.ID, report.Error)
	}

	log.Info("command completed",
		zap.String("status", string(report.Status)),
		zap.String("output", report.Output),
	)
	return nil
}

// SendAll dispatches the same command to all targets in parallel.
// It returns the first error encountered (fail-fast).
func (c *HTTPClient) SendAll(nc *dag.NodeContext, targets []Target, cmd Command) error {
	if len(targets) == 0 {
		return nil
	}
	if len(targets) == 1 {
		return c.Send(nc, targets[0], cmd)
	}

	ctx, cancel := context.WithCancel(nc)
	defer cancel()

	var (
		once     sync.Once
		firstErr error
		wg       sync.WaitGroup
	)

	// Build a child NodeContext that respects our cancel.
	childNC := nc.WithContext(ctx)

	for _, t := range targets {
		wg.Add(1)
		go func(target Target) {
			defer wg.Done()
			if err := c.Send(childNC, target, cmd); err != nil {
				once.Do(func() {
					firstErr = err
					cancel() // signal other goroutines to abort
				})
			}
		}(t)
	}

	wg.Wait()
	return firstErr
}

// waitForAgent polls the agent's /health endpoint until it responds 200
// or the timeout is reached. Uses exponential backoff starting at healthInterval.
func (c *HTTPClient) waitForAgent(ctx context.Context, target Target) error {
	url := target.Addr() + "/health"
	deadline := time.Now().Add(healthTimeout)
	interval := healthInterval

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("agent %s did not become healthy within %s", target.ID, healthTimeout)
		}

		reqCtx, reqCancel := context.WithTimeout(ctx, 5*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		if err != nil {
			reqCancel()
			return fmt.Errorf("agent: create health request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		reqCancel()
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		// Check if parent context is cancelled before sleeping.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}

		// Exponential backoff with cap.
		interval *= 2
		if interval > healthMaxBackoff {
			interval = healthMaxBackoff
		}
	}
}

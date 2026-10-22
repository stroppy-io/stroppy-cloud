package run

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// ConnectivityOutcomes is how a dial of the pipeline's connectivity check
// ends (pipelines/workflows.NetOutcomes); the contract's
// ConnectivityOutcome is the same list.
var ConnectivityOutcomes = []string{"ok", "refused", "timeout", "other"}

// ConnectivityState is the run's connectivity check (network.checked):
// the latest report replaces the earlier one, since it carries every path.
type ConnectivityState struct {
	Stage     string                 `json:"stage,omitempty"`
	CheckedAt time.Time              `json:"checked_at"`
	Total     int                    `json:"total"`
	Failed    int                    `json:"failed"`
	Critical  int                    `json:"critical"`
	Pairs     []ConnectivityPair     `json:"pairs"`
	Snapshots []ConnectivitySnapshot `json:"snapshots"`
}

// ConnectivityPair is one dialed path.
type ConnectivityPair struct {
	From      string  `json:"from"`
	To        string  `json:"to"`
	Port      int     `json:"port"`
	Address   string  `json:"address,omitempty"`
	Container string  `json:"container,omitempty"`
	Critical  bool    `json:"critical"`
	Outcome   string  `json:"outcome"`
	LatencyMs float64 `json:"latency_ms"`
	Attempts  int     `json:"attempts,omitempty"`
	Error     string  `json:"error,omitempty"`
}

// OK reports a connected path.
func (p ConnectivityPair) OK() bool { return p.Outcome == "ok" }

// ConnectivitySnapshot is the network snapshot of one machine: an artifact
// ref ("artifact/<id>").
type ConnectivitySnapshot struct {
	Machine  string `json:"machine"`
	Artifact string `json:"artifact"`
}

// connectivityOf decodes a network.checked payload. An outcome this server
// does not know reads as other.
func connectivityOf(p map[string]any, at time.Time) (ConnectivityState, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return ConnectivityState{}, err
	}
	var st ConnectivityState
	if err := json.Unmarshal(raw, &st); err != nil {
		return ConnectivityState{}, err
	}
	st.CheckedAt = at
	for i := range st.Pairs {
		if !slices.Contains(ConnectivityOutcomes, st.Pairs[i].Outcome) {
			st.Pairs[i].Outcome = "other"
		}
	}
	if st.Pairs == nil {
		st.Pairs = []ConnectivityPair{}
	}
	if st.Snapshots == nil {
		st.Snapshots = []ConnectivitySnapshot{}
	}
	return st, nil
}

// reason names the failed paths for the timeline, critical ones first.
func (c ConnectivityState) reason() string {
	var parts []string
	for _, critical := range []bool{true, false} {
		for _, p := range c.Pairs {
			if !p.OK() && p.Critical == critical && len(parts) < 3 {
				parts = append(parts, fmt.Sprintf("%s:%d from %s (%s)", p.To, p.Port, p.From, p.Outcome))
			}
		}
	}
	if more := c.Failed - len(parts); more > 0 {
		parts = append(parts, fmt.Sprintf("+%d", more))
	}
	return strings.Join(parts, "; ")
}

// failedOnly is the timeline payload: the counts, the snapshots and the
// failed paths — the full matrix lives in the overview.
func failedOnly(p map[string]any) map[string]any {
	out := make(map[string]any, len(p))
	for k, v := range p {
		out[k] = v
	}
	pairs, _ := p["pairs"].([]any) //nolint:errcheck // absent = none
	failed := []any{}
	for _, x := range pairs {
		if m, ok := x.(map[string]any); ok && m["outcome"] != "ok" {
			failed = append(failed, m)
		}
	}
	out["pairs"] = failed
	return out
}

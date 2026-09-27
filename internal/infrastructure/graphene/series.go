package graphene

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/observe"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// seriesOf maps a Prometheus matrix into series with aggregates.
func seriesOf(raw json.RawMessage, key, title, unit string) ([]observe.Series, error) {
	var body struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Values [][2]any          `json:"values"`
			} `json:"result"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	if body.Status != "success" {
		return nil, fmt.Errorf("%s", body.Error)
	}
	out := make([]observe.Series, 0, len(body.Data.Result))
	for _, r := range body.Data.Result {
		// Machine carries the agent; the service names the machine.
		s := observe.Series{Key: key, Title: title, Unit: unit, Machine: firstOf(r.Metric, spec.AttrAgent, "graphene_agent", "instance")}
		var values []float64
		for _, v := range r.Values {
			ts, ok := v[0].(float64)
			str, ok2 := v[1].(string)
			if !ok || !ok2 {
				continue
			}
			f, err := strconv.ParseFloat(str, 64)
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				continue
			}
			s.Points = append(s.Points, [2]float64{ts, f})
			values = append(values, f)
		}
		if len(values) > 0 {
			s.Min, s.Max, s.Last = values[0], values[0], values[len(values)-1]
			sum := 0.0
			for _, f := range values {
				sum += f
				s.Min = min(s.Min, f)
				s.Max = max(s.Max, f)
			}
			s.Avg = sum / float64(len(values))
			sorted := append([]float64(nil), values...)
			sort.Float64s(sorted)
			s.P95 = sorted[(len(sorted)-1)*95/100]
		}
		out = append(out, s)
	}
	return out, nil
}

func firstOf(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := m[k]; v != "" {
			return v
		}
	}
	return ""
}

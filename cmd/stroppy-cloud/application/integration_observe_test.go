//go:build integration

package application

import (
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/transport/ws"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestE2EObserve(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixture(t, e)
	var r runView
	e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{"name": "observed"}, tok), http.StatusCreated, &r)
	if r = finishRun(t, e, base, tok, r.ID); r.Status != "completed" {
		t.Fatalf("run %+v", r)
	}
	fr, _ := e.graphene.runOf(r.ID)
	var result spec.Result
	if err := json.Unmarshal(fr.rec.Result, &result); err != nil {
		t.Fatal(err)
	}

	type logPage struct {
		Data []struct {
			Time      time.Time         `json:"time"`
			Message   string            `json:"message"`
			Level     string            `json:"level"`
			Role      string            `json:"role"`
			Machine   string            `json:"machine"`
			Container string            `json:"container"`
			Fields    map[string]string `json:"fields"`
		} `json:"data"`
		Older *string `json:"older"`
		Newer *string `json:"newer"`
	}
	t.Run("typed logs: the run's components, translated back to the plan", func(t *testing.T) {
		var page logPage
		e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/logs?role=db&q=ready&limit=50", nil, tok), http.StatusOK, &page)
		found := false
		for _, l := range page.Data {
			if l.Role != "db" || l.Machine != "db-1" {
				t.Fatalf("role filter leaked %+v", l)
			}
			if strings.Contains(l.Message, "database system is ready to accept connections") && l.Container == "db-1-postgres" {
				found = true
			}
		}
		if !found {
			t.Fatalf("postgres readiness not in logs %+v", page)
		}
		e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/logs?container=db-1-postgres&limit=3", nil, tok), http.StatusOK, &page)
		if len(page.Data) != 3 || page.Older == nil || page.Data[0].Time.Before(page.Data[2].Time) {
			t.Fatalf("container page %+v", page)
		}
		older := *page.Older
		e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/logs?container=db-1-postgres&limit=3&cursor="+older+"&direction=older", nil, tok), http.StatusOK, &page)
		for _, l := range page.Data {
			if l.Container != "db-1-postgres" || l.Time.Format(time.RFC3339Nano) >= older {
				t.Fatalf("older page %+v after %s", l, older)
			}
		}
		// The segment window holds Stroppy's own output on the runner.
		e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/logs?segment=main&limit=200", nil, tok), http.StatusOK, &page)
		stroppy := false
		for _, l := range page.Data {
			if strings.HasPrefix(l.Message, "running main:") && l.Machine == "runner-1" && l.Role == "runner" {
				stroppy = true
			}
		}
		if !stroppy {
			t.Fatalf("segment logs %+v", page)
		}
		e.want(e.req(http.MethodPost, base+"/runs/"+r.ID+"/logs:raw", map[string]any{"query": `_msg:"checkpoint complete"`}, tok), http.StatusOK, &page)
		if len(page.Data) == 0 || page.Data[0].Container != "db-1-postgres" {
			t.Fatalf("raw logs %+v", page)
		}
		e.problem(e.req(http.MethodPost, base+"/runs/"+r.ID+"/logs:raw", map[string]any{"query": ""}, tok), http.StatusUnprocessableEntity, "invalid")
		var facets struct {
			Data []struct {
				Field  string `json:"field"`
				Values []struct {
					Value string `json:"value"`
					Count int    `json:"count"`
				} `json:"values"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/logs:facets", nil, tok), http.StatusOK, &facets)
		got := map[string][]string{}
		for _, f := range facets.Data {
			for _, v := range f.Values {
				got[f.Field] = append(got[f.Field], v.Value)
			}
		}
		if !slices.Contains(got["machine"], "db-1") || !slices.Contains(got["machine"], "runner-1") || !slices.Contains(got["role"], "db") ||
			!slices.Contains(got["container"], "db-1-postgres") || !slices.Contains(got["level"], "info") {
			t.Fatalf("facets %+v", got)
		}
	})

	now := time.Now().UTC()
	t.Run("metrics: every catalog expression over the real stores", func(t *testing.T) {
		type metricsView struct {
			Window struct {
				Start   time.Time `json:"start"`
				Segment string    `json:"segment"`
			} `json:"window"`
			Series []struct {
				Key        string      `json:"key"`
				Machine    string      `json:"machine"`
				Role       string      `json:"role"`
				Points     [][]float64 `json:"points"`
				Aggregates struct {
					Avg  float64 `json:"avg"`
					Max  float64 `json:"max"`
					Last float64 `json:"last"`
				} `json:"aggregates"`
			} `json:"series"`
			Errors []map[string]string `json:"errors"`
		}
		var all metricsView
		e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/metrics", nil, tok), http.StatusOK, &all)
		if len(all.Errors) != 0 {
			t.Fatalf("catalog expressions failed: %+v", all.Errors)
		}
		keys := map[string]int{}
		for _, s := range all.Series {
			if len(s.Points) == 0 {
				t.Fatalf("empty series %+v", s)
			}
			keys[s.Key]++
		}
		for _, k := range []string{
			"tps", "latency_p95_ms", "iterations_total", "node_cpu_usage", "node_memory_used_bytes", "node_disk_io_bytes", "node_network_bytes",
			"pg_stat_database_xact_commit", "pg_stat_database_blks_hit_ratio", "pg_stat_activity_count", "pg_locks_count",
		} {
			if keys[k] == 0 {
				t.Fatalf("no series for %s: %v", k, keys)
			}
		}
		var m metricsView
		e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/metrics?keys=tps,node_cpu_usage,pg_stat_database_xact_commit&segment=main", nil, tok), http.StatusOK, &m)
		byKey := map[string][]string{}
		for _, s := range m.Series {
			byKey[s.Key] = append(byKey[s.Key], s.Machine+"/"+s.Role)
			switch s.Key {
			case "tps":
				if math.Abs(s.Aggregates.Max-result.Summary.TPS)/result.Summary.TPS > 0.15 {
					t.Fatalf("tps series %v vs result %v", s.Aggregates, result.Summary.TPS)
				}
			case "node_cpu_usage":
				if s.Aggregates.Max < 50 {
					t.Fatalf("cpu under load %+v", s)
				}
			}
		}
		slices.Sort(byKey["node_cpu_usage"])
		if m.Window.Segment != "main" || !slices.Equal(byKey["node_cpu_usage"], []string{"db-1/db", "runner-1/runner"}) || !slices.Equal(byKey["pg_stat_database_xact_commit"], []string{"db-1/db"}) {
			t.Fatalf("metrics %+v", byKey)
		}
		e.problem(e.req(http.MethodGet, base+"/runs/"+r.ID+"/metrics?keys=nope", nil, tok), http.StatusUnprocessableEntity, "invalid")
		e.problem(e.req(http.MethodGet, base+"/runs/"+r.ID+"/metrics?segment=missing", nil, tok), http.StatusNotFound, "not_found")

		var raw struct {
			Status string `json:"status"`
			Data   struct {
				Result []struct {
					Metric map[string]string `json:"metric"`
					Values [][2]any          `json:"values"`
				} `json:"result"`
			} `json:"data"`
		}
		// Native Stroppy counters end at the result's numbers.
		e.want(e.req(http.MethodPost, base+"/runs/"+r.ID+"/metrics:raw", map[string]any{"query": `stroppy_iterations_total`, "start": now.Add(-time.Hour), "end": now}, tok), http.StatusOK, &raw)
		if raw.Status != "success" || len(raw.Data.Result) != 1 || raw.Data.Result[0].Metric["stroppy_segment"] != "main" {
			t.Fatalf("native raw %+v", raw)
		}
		values := raw.Data.Result[0].Values
		last, _ := strconv.ParseFloat(values[len(values)-1][1].(string), 64)
		if last != result.Segments[0].Metrics["iterations_total"].Value {
			t.Fatalf("native iterations %v != result %v", last, result.Segments[0].Metrics["iterations_total"].Value)
		}
		// A scoped query sees this run's components only.
		e.want(e.req(http.MethodPost, base+"/runs/"+r.ID+"/metrics:raw", map[string]any{"query": `sum by ("graphene.agent") (node_load1)`, "start": now.Add(-time.Hour), "end": now}, tok), http.StatusOK, &raw)
		if len(raw.Data.Result) != 2 {
			t.Fatalf("scoped raw %+v", raw)
		}
		e.problem(e.req(http.MethodPost, base+"/runs/"+r.ID+"/metrics:raw", map[string]any{"query": "rate(", "start": now.Add(-time.Hour), "end": now}, tok), http.StatusUnprocessableEntity, "invalid")
	})

	t.Run("grafana session links the dashboards", func(t *testing.T) {
		var gs struct {
			Dashboards []struct {
				ID         string `json:"id"`
				URL        string `json:"url"`
				PerMachine bool   `json:"per_machine"`
			} `json:"dashboards"`
		}
		e.want(e.req(http.MethodPost, base+"/runs/"+r.ID+"/grafana-session", nil, tok), http.StatusOK, &gs)
		if len(gs.Dashboards) != 2 || !strings.HasPrefix(gs.Dashboards[0].URL, "/grafana/d/stroppy-run?") || !strings.Contains(gs.Dashboards[0].URL, "var-run_id="+r.ID) || !gs.Dashboards[1].PerMachine {
			t.Fatalf("grafana %+v", gs)
		}
	})

	t.Run("ws log tail and metrics topics", func(t *testing.T) {
		c := dialWS(t, e, tok)
		c.send(ws.Frame{Type: "subscribe", SubID: "logs", Topic: "run.logs/" + r.ID})
		f := c.next(5*time.Second, "event", "logs")
		if !strings.Contains(string(f.Payload), `"message"`) || f.Cursor == "" {
			t.Fatalf("log tail %+v", f)
		}
		c.send(ws.Frame{Type: "subscribe", SubID: "m", Topic: "run.metrics/" + r.ID})
		f = c.next(5*time.Second, "event", "m")
		if !strings.Contains(string(f.Payload), `"series"`) {
			t.Fatalf("metrics topic %+v", f)
		}
	})
}

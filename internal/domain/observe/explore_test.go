package observe

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

type exploreRuns struct{ r run.Run }

func (f exploreRuns) Get(context.Context, auth.Actor, uuid.UUID, uuid.UUID) (run.Run, error) {
	return f.r, nil
}

// exploreStore records the PromQL it is asked and answers canned matrices.
type exploreStore struct {
	queries []string
	answer  func(query string) string
}

func (s *exploreStore) Query(context.Context, MetricQuery) ([]Series, []error, error) {
	return nil, nil, nil
}

func (s *exploreStore) Raw(_ context.Context, _ Scope, query string, _, _ time.Time, _ time.Duration) (json.RawMessage, error) {
	// The kind probes: pg_stat_database_xact_commit only grows, pg_up flips once.
	switch {
	case strings.HasPrefix(query, "max by (__name__) (resets("):
		return json.RawMessage(matrix(`{"metric":{"__name__":"pg_stat_database_xact_commit"},"values":[[1800000600,"0"]]}`, `{"metric":{"__name__":"pg_up"},"values":[[1800000600,"0"]]}`)), nil
	case strings.HasPrefix(query, "max by (__name__) (changes("):
		return json.RawMessage(matrix(`{"metric":{"__name__":"pg_stat_database_xact_commit"},"values":[[1800000600,"30"]]}`, `{"metric":{"__name__":"pg_up"},"values":[[1800000600,"1"]]}`)), nil
	}
	s.queries = append(s.queries, query)
	return json.RawMessage(s.answer(query)), nil
}

func exploreRun(t *testing.T) run.Run {
	t.Helper()
	runID := "fa7f5b0c-00c3-4a10-ab14-f1a1dbafc9f7"
	spec, _ := json.Marshal(spec.Run{RunID: runID, Machines: []spec.Machine{{Name: "db-1", Role: "db"}, {Name: "runner-1", Role: "runner"}}, Containers: []spec.Container{{Name: "db-1-node-exporter", Machine: "db-1"}, {Name: "db-1-postgres-exporter", Machine: "db-1"}}})
	started := time.Unix(1_800_000_000, 0).UTC()
	finished := started.Add(10 * time.Minute)
	return run.Run{ID: uuid.MustParse(runID), GrapheneNamespace: "t-acme", RunSpec: spec, StartedAt: &started, FinishedAt: &finished, Status: run.StatusCompleted}
}

func matrix(rows ...string) string {
	return `{"status":"success","data":{"resultType":"matrix","result":[` + strings.Join(rows, ",") + `]}}`
}

func TestMetricNamesGroupEveryStoredName(t *testing.T) {
	store := &exploreStore{answer: func(string) string {
		return matrix(
			`{"metric":{"__name__":"node_cpu_seconds_total","graphene.agent":"fa7f5b0c-db-1","graphene.entity":"docker/fa7f5b0c-00c3-4a10-ab14-f1a1dbafc9f7-db-1-node-exporter","graphene.role":"machine"},"values":[[1800000060,"8"],[1800000120,"16"]]}`,
			`{"metric":{"__name__":"pg_up","graphene.agent":"fa7f5b0c-db-1","graphene.entity":"docker/fa7f5b0c-00c3-4a10-ab14-f1a1dbafc9f7-db-1-postgres-exporter","graphene.role":"machine"},"values":[[1800000060,"1"]]}`,
			`{"metric":{"__name__":"stroppy_iterations_total","graphene.agent":"fa7f5b0c-runner-1","graphene.role":"workload","service.name":"stroppy"},"values":[[1800000060,"1"]]}`,
			`{"metric":{"__name__":"pg_stat_database_xact_commit","graphene.agent":"fa7f5b0c-db-1","graphene.entity":"docker/fa7f5b0c-00c3-4a10-ab14-f1a1dbafc9f7-db-1-postgres-exporter","graphene.role":"machine"},"values":[[1800000060,"3"]]}`,
			`{"metric":{"__name__":"docker.container.cpu.percent","graphene.agent":"fa7f5b0c-db-1","graphene.entity":"docker/fa7f5b0c-00c3-4a10-ab14-f1a1dbafc9f7-db-1-node-exporter"},"values":[[1800000060,"2"]]}`,
		)
	}}
	r := exploreRun(t)
	svc := NewService(exploreRuns{r}, nil, store, nil)
	names, _, err := svc.MetricNames(context.Background(), auth.Actor{}, uuid.New(), r.ID, "", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]MetricName{}
	for _, n := range names {
		by[n.Name] = n
	}
	cpu := by["node_cpu_seconds_total"]
	if cpu.Origin != OriginExporter || cpu.Kind != KindCounter || cpu.Series != 16 || len(cpu.Machines) != 1 || cpu.Machines[0] != "db-1" || cpu.Roles[0] != "db" || cpu.Containers[0] != "db-1-node-exporter" {
		t.Fatalf("node cpu %+v", cpu)
	}
	// An exporter counter without _total is told by its data.
	if by["pg_stat_database_xact_commit"].Kind != KindCounter {
		t.Fatalf("xact_commit %+v", by["pg_stat_database_xact_commit"])
	}
	if by["pg_up"].Kind != KindGauge || by["pg_up"].Containers[0] != "db-1-postgres-exporter" {
		t.Fatalf("pg_up %+v", by["pg_up"])
	}
	if w := by["stroppy_iterations_total"]; w.Origin != OriginWorkload || w.Machines[0] != "runner-1" || len(w.Containers) != 0 {
		t.Fatalf("workload %+v", w)
	}
	if by["docker.container.cpu.percent"].Origin != OriginContainer {
		t.Fatalf("container stats %+v", by["docker.container.cpu.percent"])
	}
}

func TestMetricSeriesJoinsPipelineDuplicates(t *testing.T) {
	store := &exploreStore{answer: func(string) string {
		return matrix(
			`{"metric":{"__name__":"node_load1","graphene.agent":"fa7f5b0c-db-1","graphene.activity":"docker.container.observe"},"values":[[1800000060,"3"],[1800000120,"4"]]}`,
			`{"metric":{"__name__":"node_load1","graphene.agent":"fa7f5b0c-db-1","graphene.activity":"docker.container.remove"},"values":[[1800000180,"2"]]}`,
		)
	}}
	r := exploreRun(t)
	svc := NewService(exploreRuns{r}, nil, store, nil)
	res, err := svc.MetricSeries(context.Background(), auth.Actor{}, uuid.New(), r.ID, SeriesQuery{Name: "node_load1", Fn: "value"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || len(res.Series[0].Points) != 3 || res.Series[0].Last != 2 {
		t.Fatalf("series %+v", res.Series)
	}
}

func TestMetricSeriesBuildsTheQuery(t *testing.T) {
	store := &exploreStore{answer: func(q string) string {
		return matrix(
			`{"metric":{"graphene.agent":"fa7f5b0c-db-1","mode":"user"},"values":[[1800000060,"0.5"],[1800000120,"1.5"]]}`,
			`{"metric":{"graphene.agent":"fa7f5b0c-db-1","mode":"idle"},"values":[[1800000060,"3"],[1800000120,"5"]]}`,
		)
	}}
	r := exploreRun(t)
	svc := NewService(exploreRuns{r}, nil, store, nil)
	res, err := svc.MetricSeries(context.Background(), auth.Actor{}, uuid.New(), r.ID, SeriesQuery{Name: "node_cpu_seconds_total", By: []string{"machine", "mode"}, Machines: []string{"db-1"}})
	if err != nil {
		t.Fatal(err)
	}
	// A counter is drawn as its rate, grouped, scoped to the machine's agent.
	q := store.queries[0]
	for _, part := range []string{`sum by ("graphene.agent", "mode")`, `rate({__name__="node_cpu_seconds_total","graphene.agent"=~"fa7f5b0c-db-1"}[`} {
		if !strings.Contains(q, part) {
			t.Fatalf("query %s lacks %s", q, part)
		}
	}
	if res.Fn != "rate" || res.Total != 2 || res.Series[0].Labels["mode"] != "idle" || res.Series[0].Machine != "db-1" || res.Series[0].Role != "db" {
		t.Fatalf("series %+v", res)
	}
	if res.Series[0].Avg != 4 || res.Series[0].Last != 5 || res.Series[0].Points[0][0] != 1_800_000_060_000 {
		t.Fatalf("aggregates/points %+v", res.Series[0])
	}
	// Labels outside the metric's own set are refused, not passed to PromQL.
	if _, err := svc.MetricSeries(context.Background(), auth.Actor{}, uuid.New(), r.ID, SeriesQuery{Name: "x", By: []string{`a"}) or vector(1`}}); err == nil {
		t.Fatal("injected label accepted")
	}
	if _, err := svc.MetricSeries(context.Background(), auth.Actor{}, uuid.New(), r.ID, SeriesQuery{Name: "x", By: []string{"graphene.namespace"}}); err == nil {
		t.Fatal("internal label accepted")
	}
}

//go:build integration

package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	collogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	logs "go.opentelemetry.io/proto/otlp/logs/v1"
	metrics "go.opentelemetry.io/proto/otlp/metrics/v1"
	resource "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

/*
TELEMETRY: what a simulated run would have left in the installation's
VictoriaLogs and VictoriaMetrics, written the way it gets there — OTLP to
the stores' own OpenTelemetry endpoints, as Graphene's collector forwards
it (graphene docker-compose: /insert/opentelemetry/v1/logs,
/opentelemetry/v1/metrics). The stores do the attribute → field/label
conversion themselves, exactly as in production.

Shapes follow what the live runs recorded (pipelines/live): every record
carries graphene.namespace/run/agent (resource) and graphene.entity/
activity/attempt (record); container logs are stream=container lines of
docker.container.observe; scraped exporter series keep their own names
and labels; Stroppy's native series spell the run labels graphene_run,
graphene_namespace and carry stroppy_segment + instance. The numbers are
consistent with the run: the native counters end at the segment result's
values, databases work harder while a segment runs.
*/

// telemetryStep is the scrape interval.
const telemetryStep = 15 * time.Second

// telemetry writes one simulated run's logs and metrics.
type telemetry struct {
	logsURL, metricsURL string
	namespace           string
	r                   *doorRun
	rs                  spec.Run
	// busy tells whether a workload segment runs at t (0..1 load).
	segments []simSegment
	logs     []*logs.ResourceLogs
	metrics  []*metrics.ResourceMetrics
}

// pushTelemetry writes a run's telemetry to the stores and flushes them.
func pushTelemetry(ctx context.Context, logsURL, metricsURL string, r *doorRun) error {
	if r.rec == nil || r.pipeline != "stroppy-run" {
		return nil
	}
	t := &telemetry{logsURL: logsURL, metricsURL: metricsURL, namespace: r.namespace, r: r, segments: r.rec.Segments}
	if err := json.Unmarshal(r.params, &t.rs); err != nil {
		return err
	}
	t.build()
	return t.send(ctx)
}

func attr(k, v string) *common.KeyValue {
	return &common.KeyValue{Key: k, Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: v}}}
}

func attrs(kv ...string) []*common.KeyValue {
	out := make([]*common.KeyValue, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, attr(kv[i], kv[i+1]))
	}
	return out
}

// load is how hard the databases work at t: segments drive them.
func (t *telemetry) load(at time.Duration) float64 {
	for _, s := range t.segments {
		if at >= s.Start && at <= s.End {
			return 0.85
		}
	}
	return 0.04
}

// jitter is a deterministic wobble in [-1, 1].
func jitter(key string, i int) float64 {
	h := fnv.New32a()
	_, _ = fmt.Fprintf(h, "%s/%d", key, i)
	return float64(h.Sum32()%2001)/1000 - 1
}

// container is one Docker record of the run and when it lived.
type simContainer struct {
	spec       spec.Container
	entity     string
	agent      string
	start, end time.Duration
}

func (t *telemetry) containers() []simContainer {
	byEntity := map[string]spec.Container{}
	for _, c := range t.rs.Containers {
		byEntity[spec.ContainerEntity(t.rs.RunID, c.Name)] = c
	}
	var out []simContainer
	for _, res := range t.r.rec.Resources {
		c, ok := byEntity[string(res.Ref)]
		if !ok || res.Ready < 0 {
			continue
		}
		end := t.r.rec.Duration
		if res.Deleted >= 0 {
			end = res.Deleted
		}
		out = append(out, simContainer{spec: c, entity: string(res.Ref), agent: string(res.Agent), start: res.Ready, end: end})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].entity < out[j].entity })
	return out
}

func (t *telemetry) at(d time.Duration) uint64 { return uint64(t.r.rec.Start.Add(d).UnixNano()) }

func (t *telemetry) resource(agent string) *resource.Resource {
	return &resource.Resource{Attributes: attrs(
		spec.AttrNamespace, t.namespace, spec.AttrRun, t.r.id, spec.AttrAgent, agent, "graphene.role", "machine",
	)}
}

// --- logs ---------------------------------------------------------------------

func (t *telemetry) logLine(agent, entity, activity string, at time.Duration, severity logs.SeverityNumber, text string, extra ...string) (string, *logs.LogRecord) {
	rec := &logs.LogRecord{
		TimeUnixNano: t.at(at), ObservedTimeUnixNano: t.at(at), SeverityNumber: severity, SeverityText: severityText(severity),
		Body:       &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: text}},
		Attributes: attrs(append([]string{"graphene.activity", activity, "graphene.attempt", "1", "stream", "container"}, extra...)...),
	}
	if entity != "" {
		rec.Attributes = append(rec.Attributes, attr(spec.AttrEntity, entity))
	}
	return agent, rec
}

func severityText(s logs.SeverityNumber) string {
	switch {
	case s >= logs.SeverityNumber_SEVERITY_NUMBER_ERROR:
		return "ERROR"
	case s >= logs.SeverityNumber_SEVERITY_NUMBER_WARN:
		return "WARN"
	}
	return "INFO"
}

// containerLines is what a component prints over its life.
func containerLines(c simContainer) (boot []string, periodic func(i int, busy bool) (string, logs.SeverityNumber)) {
	image := c.spec.Image
	info := logs.SeverityNumber_SEVERITY_NUMBER_INFO
	switch {
	case strings.Contains(image, "node-exporter"):
		return []string{`level=INFO source=node_exporter.go:216 msg="Starting node_exporter" version="(version=1.12.1)"`, `level=INFO source=tls_config.go:354 msg="Listening on" address=[::]:9100`},
			func(int, bool) (string, logs.SeverityNumber) { return "", 0 }
	case strings.Contains(image, "exporter"):
		return []string{`level=INFO msg="Starting exporter"`, `level=INFO msg="Listening on" address=[::]:9187`},
			func(int, bool) (string, logs.SeverityNumber) { return "", 0 }
	case strings.Contains(image, "postgres") || strings.Contains(image, "orioledb"):
		return []string{"PostgreSQL init process complete; ready for start up.", "LOG:  starting PostgreSQL 17.6 on x86_64-pc-linux-gnu", "LOG:  listening on IPv4 address \"0.0.0.0\", port 5432", "LOG:  database system is ready to accept connections"},
			func(i int, busy bool) (string, logs.SeverityNumber) {
				switch {
				case busy && i%7 == 3:
					return "ERROR:  could not serialize access due to concurrent update", logs.SeverityNumber_SEVERITY_NUMBER_ERROR
				case i%4 == 0:
					return fmt.Sprintf("LOG:  checkpoint complete: wrote %d buffers (%.1f%%)", 120+i*37, 0.4+float64(i%9)), info
				}
				return "", 0
			}
	case strings.Contains(image, "mysql") || strings.Contains(image, "mariadb") || strings.Contains(image, "percona"):
		return []string{"[System] [MY-010116] [Server] /usr/sbin/mysqld starting as process 1", "[System] [MY-010931] [Server] /usr/sbin/mysqld: ready for connections. port: 3306"},
			func(i int, busy bool) (string, logs.SeverityNumber) {
				if busy && i%9 == 4 {
					return "[Warning] [MY-010055] [Server] IP address could not be resolved", logs.SeverityNumber_SEVERITY_NUMBER_WARN
				}
				return "", 0
			}
	case strings.Contains(image, "cockroach"):
		return []string{"CockroachDB node starting at " + time.Now().UTC().Format(time.RFC3339), "node startup completed"},
			func(i int, _ bool) (string, logs.SeverityNumber) {
				if i%6 == 0 {
					return "I250921 health.go:162 runtime stats: 1.2 GiB RSS, 412 goroutines", info
				}
				return "", 0
			}
	}
	return []string{c.spec.Name + " started"}, func(int, bool) (string, logs.SeverityNumber) { return "", 0 }
}

func (t *telemetry) buildLogs() {
	byAgent := map[string][]*logs.LogRecord{}
	add := func(agent string, rec *logs.LogRecord) { byAgent[agent] = append(byAgent[agent], rec) }
	for _, c := range t.containers() {
		boot, periodic := containerLines(c)
		for i, line := range boot {
			add(t.logLine(c.agent, c.entity, "docker.container.observe", c.start-time.Duration(len(boot)-i)*time.Second, logs.SeverityNumber_SEVERITY_NUMBER_INFO, line))
		}
		for i, at := 0, c.start+30*time.Second; at < c.end; i, at = i+1, at+30*time.Second {
			if line, sev := periodic(i, t.load(at) > 0.5); line != "" {
				add(t.logLine(c.agent, c.entity, "docker.container.observe", at, sev, line))
			}
		}
		if t.r.rec.Scenario.FailHealthcheck != "" && strings.HasSuffix(c.spec.Name, t.r.rec.Scenario.FailHealthcheck) {
			add(t.logLine(c.agent, c.entity, "docker.container.observe", c.start+5*time.Second, logs.SeverityNumber_SEVERITY_NUMBER_FATAL, "FATAL:  could not open configuration file: Permission denied"))
		}
	}
	// The workload: Stroppy's output streamed by the segment activity.
	for _, s := range t.segments {
		for i, at := 0, s.Start+10*time.Second; at < s.End; i, at = i+1, at+10*time.Second {
			done := float64(at-s.Start) / float64(s.End-s.Start)
			line := fmt.Sprintf("running %s: %3.0f%% iterations=%.0f", s.Name, done*100, s.Metrics["iterations_total"].Value*done)
			add(t.logLine(s.Agent, "", "stroppy.segment.run", at, logs.SeverityNumber_SEVERITY_NUMBER_INFO, line, "stream", "stroppy"))
		}
		add(t.logLine(s.Agent, "", "stroppy.segment.run", s.End, logs.SeverityNumber_SEVERITY_NUMBER_INFO, "=== bench summary === "+s.Name, "stream", "stroppy"))
	}
	agents := make([]string, 0, len(byAgent))
	for a := range byAgent {
		agents = append(agents, a)
	}
	sort.Strings(agents)
	for _, a := range agents {
		t.logs = append(t.logs, &logs.ResourceLogs{Resource: t.resource(a), ScopeLogs: []*logs.ScopeLogs{{
			Scope: &common.InstrumentationScope{Name: "graphene.obs"}, LogRecords: byAgent[a],
		}}})
	}
}

// --- metrics ------------------------------------------------------------------

// series collects the data points of one resource.
type series struct {
	t       *telemetry
	metrics []*metrics.Metric
	base    []*common.KeyValue
}

func (s *series) gauge(name string, labels []string, points func(at time.Duration, i int) float64, from, to time.Duration) {
	var dps []*metrics.NumberDataPoint
	for i, at := 0, from; at <= to; i, at = i+1, at+telemetryStep {
		dps = append(dps, &metrics.NumberDataPoint{TimeUnixNano: s.t.at(at), Attributes: append(attrs(labels...), s.base...), Value: &metrics.NumberDataPoint_AsDouble{AsDouble: points(at, i)}})
	}
	s.metrics = append(s.metrics, &metrics.Metric{Name: name, Data: &metrics.Metric_Gauge{Gauge: &metrics.Gauge{DataPoints: dps}}})
}

// counter integrates rate(at) (per second) into a cumulative sum.
func (s *series) counter(name string, labels []string, rate func(at time.Duration) float64, from, to time.Duration) {
	var dps []*metrics.NumberDataPoint
	total := 0.0
	for at := from; at <= to; at += telemetryStep {
		total += rate(at) * telemetryStep.Seconds()
		dps = append(dps, &metrics.NumberDataPoint{StartTimeUnixNano: s.t.at(from), TimeUnixNano: s.t.at(at), Attributes: append(attrs(labels...), s.base...), Value: &metrics.NumberDataPoint_AsDouble{AsDouble: math.Round(total)}})
	}
	s.metrics = append(s.metrics, &metrics.Metric{Name: name, Data: &metrics.Metric_Sum{Sum: &metrics.Sum{IsMonotonic: true, AggregationTemporality: metrics.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE, DataPoints: dps}}})
}

// cumulative is a counter given by its value over time.
func (s *series) cumulative(name string, value func(at time.Duration) float64, from, to time.Duration) {
	var dps []*metrics.NumberDataPoint
	point := func(at time.Duration) {
		dps = append(dps, &metrics.NumberDataPoint{StartTimeUnixNano: s.t.at(from), TimeUnixNano: s.t.at(at), Attributes: s.base, Value: &metrics.NumberDataPoint_AsDouble{AsDouble: value(at)}})
	}
	at := from
	for ; at < to; at += telemetryStep {
		point(at)
	}
	// The last export is the process's final flush, at its exit.
	point(to)
	s.metrics = append(s.metrics, &metrics.Metric{Name: name, Data: &metrics.Metric_Sum{Sum: &metrics.Sum{IsMonotonic: true, AggregationTemporality: metrics.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE, DataPoints: dps}}})
}

// histogram accumulates observations distributed like a latency with
// the given median, into cumulative buckets.
func (s *series) histogram(name string, labels []string, rate func(at time.Duration) float64, median float64, bounds []float64, from, to time.Duration) {
	var dps []*metrics.HistogramDataPoint
	counts := make([]float64, len(bounds)+1)
	total, sum := 0.0, 0.0
	for at := from; at <= to; at += telemetryStep {
		n := rate(at) * telemetryStep.Seconds()
		// A log-normal-ish spread: most around the median, a tail to 5x.
		shares := make([]float64, len(bounds)+1)
		for i := range shares {
			lo, hi := 0.0, math.Inf(1)
			if i > 0 {
				lo = bounds[i-1]
			}
			if i < len(bounds) {
				hi = bounds[i]
			}
			shares[i] = cdf(hi, median) - cdf(lo, median)
		}
		for i := range counts {
			counts[i] += n * shares[i]
		}
		total += n
		sum += n * median * 1.2
		bc := make([]uint64, len(counts))
		for i, c := range counts {
			bc[i] = uint64(math.Round(c))
		}
		dps = append(dps, &metrics.HistogramDataPoint{
			StartTimeUnixNano: s.t.at(from), TimeUnixNano: s.t.at(at), Attributes: append(attrs(labels...), s.base...),
			Count: uint64(math.Round(total)), Sum: proto.Float64(sum), BucketCounts: bc, ExplicitBounds: bounds,
		})
	}
	s.metrics = append(s.metrics, &metrics.Metric{Name: name, Data: &metrics.Metric_Histogram{Histogram: &metrics.Histogram{
		AggregationTemporality: metrics.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE, DataPoints: dps,
	}}})
}

// cdf of a log-normal with the given median and sigma 0.6.
func cdf(x, median float64) float64 {
	if x <= 0 {
		return 0
	}
	if math.IsInf(x, 1) {
		return 1
	}
	return 0.5 * math.Erfc(-(math.Log(x/median))/(0.6*math.Sqrt2))
}

func (t *telemetry) machine(name string) spec.Machine {
	for _, m := range t.rs.Machines {
		if m.Name == name {
			return m
		}
	}
	return spec.Machine{CPU: 2, MemoryGB: 4}
}

func (t *telemetry) buildMetrics() {
	byAgent := map[string]*series{}
	get := func(agent string) *series {
		if s := byAgent[agent]; s != nil {
			return s
		}
		s := &series{t: t}
		byAgent[agent] = s
		return s
	}
	for _, c := range t.containers() {
		s := get(c.agent)
		s.base = attrs(spec.AttrEntity, c.entity, "graphene.activity", "docker.container.observe", "graphene.attempt", "1")
		from, to := c.start, c.end
		load := t.load
		key := c.entity
		m := t.machine(c.spec.Machine)
		image := c.spec.Image
		replica := strings.Contains(c.spec.Name, "replica")
		switch {
		case strings.Contains(image, "node-exporter"):
			for cpu := 0; cpu < max(1, m.CPU); cpu++ {
				id := fmt.Sprint(cpu)
				s.counter("node_cpu_seconds_total", []string{"cpu", id, "mode", "idle"}, func(at time.Duration) float64 { return 1 - load(at) }, from, to)
				s.counter("node_cpu_seconds_total", []string{"cpu", id, "mode", "user"}, func(at time.Duration) float64 { return load(at) * 0.7 }, from, to)
				s.counter("node_cpu_seconds_total", []string{"cpu", id, "mode", "system"}, func(at time.Duration) float64 { return load(at) * 0.25 }, from, to)
				s.counter("node_cpu_seconds_total", []string{"cpu", id, "mode", "iowait"}, func(at time.Duration) float64 { return load(at) * 0.05 }, from, to)
			}
			total := float64(max(1, m.MemoryGB)) * (1 << 30)
			s.gauge("node_memory_MemTotal_bytes", nil, func(time.Duration, int) float64 { return total }, from, to)
			s.gauge("node_memory_MemAvailable_bytes", nil, func(at time.Duration, i int) float64 { return total * (0.85 - 0.45*load(at) + 0.01*jitter(key, i)) }, from, to)
			for _, dev := range []string{"vda", "vdb"} {
				s.counter("node_disk_read_bytes_total", []string{"device", dev}, func(at time.Duration) float64 { return 40e6 * load(at) }, from, to)
				s.counter("node_disk_written_bytes_total", []string{"device", dev}, func(at time.Duration) float64 { return 90e6 * load(at) }, from, to)
			}
			for _, dev := range []string{"eth0", "lo"} {
				s.counter("node_network_receive_bytes_total", []string{"device", dev}, func(at time.Duration) float64 { return 25e6 * load(at) }, from, to)
				s.counter("node_network_transmit_bytes_total", []string{"device", dev}, func(at time.Duration) float64 { return 20e6 * load(at) }, from, to)
			}
			s.gauge("node_load1", nil, func(at time.Duration, i int) float64 { return float64(m.CPU) * load(at) * (1 + 0.1*jitter(key, i)) }, from, to)
			s.gauge("node_filesystem_avail_bytes", []string{"mountpoint", "/", "device", "/dev/vda2", "fstype", "ext4"}, func(time.Duration, int) float64 { return 12e9 }, from, to)
			s.gauge("node_uname_info", []string{"nodename", c.spec.Machine}, func(time.Duration, int) float64 { return 1 }, from, to)
		case strings.Contains(image, "postgres-exporter"):
			tps := t.tps()
			s.gauge("pg_up", nil, func(time.Duration, int) float64 { return 1 }, from, to)
			s.counter("pg_stat_database_xact_commit", []string{"datname", "postgres"}, func(at time.Duration) float64 { return tps * load(at) / 0.85 }, from, to)
			s.counter("pg_stat_database_xact_rollback", []string{"datname", "postgres"}, func(at time.Duration) float64 { return tps * 0.01 * load(at) }, from, to)
			s.counter("pg_stat_database_blks_hit", []string{"datname", "postgres"}, func(at time.Duration) float64 { return 90000 * load(at) }, from, to)
			s.counter("pg_stat_database_blks_read", []string{"datname", "postgres"}, func(at time.Duration) float64 { return 900 * load(at) }, from, to)
			for _, state := range []string{"active", "idle"} {
				s.gauge("pg_stat_activity_count", []string{"datname", "postgres", "state", state}, func(at time.Duration, i int) float64 {
					return math.Round(10 * load(at) * (1 + 0.2*jitter(key+state, i)))
				}, from, to)
			}
			s.gauge("pg_locks_count", []string{"datname", "postgres", "mode", "rowexclusivelock"}, func(at time.Duration, i int) float64 { return math.Round(40 * load(at)) }, from, to)
			lag := 0.0
			if replica {
				lag = 0.02
			}
			s.gauge("pg_replication_lag_seconds", nil, func(at time.Duration, i int) float64 { return lag * load(at) * 10 }, from, to)
			s.gauge("pg_replication_is_replica", nil, func(time.Duration, int) float64 { return map[bool]float64{true: 1}[replica] }, from, to)
		case strings.Contains(image, "mysqld-exporter"):
			tps := t.tps()
			s.gauge("mysql_up", nil, func(time.Duration, int) float64 { return 1 }, from, to)
			s.counter("mysql_global_status_queries", nil, func(at time.Duration) float64 { return 4 * tps * load(at) / 0.85 }, from, to)
			s.gauge("mysql_global_status_threads_connected", nil, func(at time.Duration, i int) float64 { return math.Round(2 + 8*load(at)) }, from, to)
			s.counter("mysql_global_status_innodb_buffer_pool_read_requests", nil, func(at time.Duration) float64 { return 80000 * load(at) }, from, to)
			s.counter("mysql_global_status_innodb_buffer_pool_reads", nil, func(at time.Duration) float64 { return 400 * load(at) }, from, to)
			if replica {
				s.gauge("mysql_slave_status_seconds_behind_master", nil, func(at time.Duration, i int) float64 { return math.Round(2 * load(at)) }, from, to)
			}
		case strings.Contains(image, "cockroach") && c.spec.Scrape != "":
			tps := t.tps()
			s.counter("sql_txn_commit_count", []string{"node_id", c.spec.Machine}, func(at time.Duration) float64 { return tps * load(at) / 0.85 }, from, to)
			s.histogram("sql_service_latency", []string{"node_id", c.spec.Machine}, func(at time.Duration) float64 { return 4 * tps * load(at) }, 1.5e6, []float64{5e5, 1e6, 2e6, 4e6, 8e6, 16e6, 32e6}, from, to)
			nodes := 0
			for _, other := range t.rs.Containers {
				if strings.Contains(other.Image, "cockroach") && other.Scrape != "" {
					nodes++
				}
			}
			s.gauge("liveness_livenodes", []string{"node_id", c.spec.Machine}, func(time.Duration, int) float64 { return float64(nodes) }, from, to)
		case strings.Contains(image, "picodata") && c.spec.Scrape != "":
			s.gauge("pico_instance_state", []string{"state", "Online"}, func(time.Duration, int) float64 { return 1 }, from, to)
			s.gauge("pico_raft_leader_id", nil, func(time.Duration, int) float64 { return 1 }, from, to)
		}
	}
	// Stroppy's native series: one writer per segment, ending at the result.
	for _, seg := range t.segments {
		s := get(seg.Agent)
		s.base = attrs(spec.LabelNativeRun, t.r.id, spec.LabelNativeNamespace, t.namespace, spec.LabelNativeSegment, seg.Name, "instance", fmt.Sprintf("%08x-stroppy-%d", fnvOf(t.r.id), seg.Index))
		span := (seg.End - seg.Start).Seconds()
		metricValue := func(k string) float64 { return seg.Metrics[k].Value }
		iterations := metricValue("iterations_total")
		steady := func(total float64) func(time.Duration) float64 {
			return func(at time.Duration) float64 {
				if at <= seg.Start || at > seg.End {
					return 0
				}
				return total / span
			}
		}
		// The counters reach the result's totals exactly at the segment's end.
		progress := func(total float64) func(time.Duration) float64 {
			return func(at time.Duration) float64 {
				return math.Round(total * math.Min(1, math.Max(0, float64(at-seg.Start)/float64(seg.End-seg.Start))))
			}
		}
		from, to := seg.Start, seg.End
		s.cumulative("stroppy_iterations_total", progress(iterations), from, to)
		s.cumulative("stroppy_failed_iterations_total", progress(metricValue("failed_iterations_total")), from, to)
		s.cumulative("stroppy_failed_queries_total", progress(metricValue("failed_queries_total")), from, to)
		s.cumulative("stroppy_terminal_errors_total", progress(metricValue("terminal_errors_total")), from, to)
		s.cumulative("stroppy_retry_attempts_total", progress(metricValue("retry_attempts_total")), from, to)
		s.histogram("stroppy_iteration_duration_milliseconds", nil, steady(iterations), metricValue("iteration_duration_p50"), []float64{0.5, 1, 2.5, 5, 10, 25, 50, 100}, from, to)
		s.histogram("stroppy_run_query_duration_milliseconds", nil, steady(metricValue("run_query_duration_count")), metricValue("run_query_duration_p50"), []float64{0.25, 0.5, 1, 2.5, 5, 10}, from, to)
	}
	agents := make([]string, 0, len(byAgent))
	for a := range byAgent {
		agents = append(agents, a)
	}
	sort.Strings(agents)
	for _, a := range agents {
		t.metrics = append(t.metrics, &metrics.ResourceMetrics{Resource: t.resource(a), ScopeMetrics: []*metrics.ScopeMetrics{{
			Scope: &common.InstrumentationScope{Name: "graphene.obs"}, Metrics: byAgent[a].metrics,
		}}})
	}
}

func fnvOf(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

// tps is what the databases committed: the segments' throughput.
func (t *telemetry) tps() float64 {
	for _, s := range t.segments {
		if v := s.Metrics["tps"].Value; v > 0 {
			return v
		}
	}
	return 1000
}

func (t *telemetry) build() {
	t.buildLogs()
	t.buildMetrics()
}

// --- transport ----------------------------------------------------------------

func postProto(ctx context.Context, url string, msg proto.Message) error {
	raw, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("%s: %d %s", url, res.StatusCode, body)
	}
	return nil
}

func (t *telemetry) send(ctx context.Context) error {
	if len(t.logs) > 0 {
		if err := postProto(ctx, t.logsURL+"/insert/opentelemetry/v1/logs", &collogs.ExportLogsServiceRequest{ResourceLogs: t.logs}); err != nil {
			return err
		}
	}
	if len(t.metrics) > 0 {
		if err := postProto(ctx, t.metricsURL+"/opentelemetry/v1/metrics", &colmetrics.ExportMetricsServiceRequest{ResourceMetrics: t.metrics}); err != nil {
			return err
		}
	}
	return flushStores(ctx, t.logsURL, t.metricsURL)
}

// flushStores makes what was written searchable now.
func flushStores(ctx context.Context, logsURL, metricsURL string) error {
	for _, url := range []string{logsURL + "/internal/force_flush", metricsURL + "/internal/force_flush"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if err != nil {
			return err
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		_ = res.Body.Close()
	}
	return nil
}

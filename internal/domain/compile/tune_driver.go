package compile

import (
	"encoding/json"
	"sort"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// tuneDriver sizes the client's connection pool under auto tuning (see
// poolSize). Left unset, pgx caps the pool at max(4, runner CPUs) and
// database/sql keeps two idle connections, so the pool follows the runner
// machine instead of the database: 256 virtual users queued on 16
// connections of a 16-vCPU runner in front of a 32-vCPU database. A pool
// key the workload sets itself wins.
//
// doc: https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool#ParseConfig (pool_max_conns default)
// doc: https://pkg.go.dev/database/sql#DB.SetMaxIdleConns (two idle connections by default)
func (c *compilation) tuneDriver(driver map[string]any, driverType string) {
	if c.tuning() != tuningAuto {
		return
	}
	conns := c.poolSize()
	switch driverType {
	case "postgres", "picodata", "crdb":
		c.clientAuto = append(c.clientAuto, setPool(driver, "postgres", map[string]int{"maxConns": conns, "minConns": conns})...)
	case "mysql", "ydb":
		c.clientAuto = append(c.clientAuto, setPool(driver, "sql", map[string]int{"maxOpenConns": conns, "maxIdleConns": conns})...)
	}
}

// connectionsPerCPU is how many connections a database vCPU serves before
// more only queue inside it. CockroachDB sizes 4 active connections per
// vCPU and PostgreSQL's wiki starts at 2 per core, but a TPC-C transaction
// here is ~18 statements with a network round trip each, so a connection
// idles between them: PostgreSQL 17 on 8 vCPU did 1049 tps with 32
// connections and 1280 tps with 64 (64 VUs, stroppy 6.2.1, stand runs of
// 2026-10-27). Eight per vCPU keeps the cores busy and still stops 10k
// virtual users from becoming 10k backends.
//
// doc: https://docs.cockroachlabs.com/docs/stable/connection-pooling
// doc: https://wiki.postgresql.org/wiki/Number_Of_Database_Connections
const connectionsPerCPU = 8

// poolSize is the client's connections: one per virtual user or load
// worker, up to connectionsPerCPU per vCPU of the database machines. More
// virtual users than that wait for a connection on the client, as an
// application behind a pool does; the report shows that wait. Without
// database machines (managed, external) the virtual users decide alone.
func (c *compilation) poolSize() int {
	want := c.peakConnections()
	cpus := 0
	for _, m := range c.machinesOf(topology.RoleDB) {
		cpus += c.hostOf(m).CPU
	}
	if cpus == 0 {
		return want
	}
	return min(want, connectionsPerCPU*cpus)
}

// peakConnections is the most connections one segment holds at once: its
// virtual users while it measures, its load workers while it loads.
func (c *compilation) peakConnections() int {
	peak := c.peakVUs()
	for _, raw := range c.in.Workload.Segments {
		var s spec.Segment
		if json.Unmarshal(raw, &s) == nil {
			peak = max(peak, int(numberOf(s.Workload.Params["load_workers"])))
		}
	}
	return peak
}

// setPool sets the pool keys the workload left unset, in its pool block or
// in the driver's own block (postgres or sql), and names the ones it set.
func setPool(driver map[string]any, family string, values map[string]int) []string {
	pool, ok := driver["pool"].(map[string]any)
	if !ok {
		pool = map[string]any{}
	}
	var own map[string]any
	if block, ok := driver[family].(map[string]any); ok {
		own = block
	}
	var set []string
	for key, v := range values {
		if _, ok := pool[key]; ok {
			continue
		}
		if _, ok := own[key]; ok {
			continue
		}
		pool[key] = v
		set = append(set, "pool."+key)
	}
	sort.Strings(set)
	driver["pool"] = pool
	return set
}

// reportedDriverKeys are the driver options a run's report may show. The
// driver map also carries the connection's credentials (authUser,
// authPassword, authToken, serviceAccountKeyFile, caCert…), and the report
// reaches public shares, so only these known, secret-free keys go in.
var reportedDriverKeys = map[string]bool{
	"pool": true, "postgres": true, "sql": true, "bulkSize": true,
	"defaultInsertMethod": true, "insertProgress": true, "tlsInsecureSkipVerify": true,
}

// clientSetup is the driver the runner gets, for the run's report: the
// secret-free driver options.
func (c *compilation) clientSetup() *run.ClientSetup {
	w := c.out.Spec.Workload
	if w.DriverType == "" {
		return nil
	}
	settings := map[string]any{}
	for k, v := range w.Driver {
		if reportedDriverKeys[k] {
			settings[k] = v
		}
	}
	return &run.ClientSetup{DriverType: w.DriverType, Settings: settings, Auto: c.clientAuto}
}

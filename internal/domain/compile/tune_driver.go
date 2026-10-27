package compile

import (
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// tuneDriver sizes the client's connection pool under auto tuning: every
// virtual user and every load worker holds a connection of its own. Left
// unset, pgx caps the pool at max(4, runner CPUs) and database/sql keeps
// two idle connections, so 256 virtual users queue on 16 connections of
// the runner and the run measures the client instead of the database. A
// pool key the workload sets itself wins.
//
// doc: https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool#ParseConfig (pool_max_conns default)
// doc: https://pkg.go.dev/database/sql#DB.SetMaxIdleConns (two idle connections by default)
func (c *compilation) tuneDriver(driver map[string]any, driverType, dsn string) string {
	if c.tuning() != tuningAuto {
		return dsn
	}
	conns := c.peakConnections()
	switch driverType {
	case "crdb":
		// Stroppy 6.2.1 drops the pool and postgres blocks for the crdb
		// driver type (stroppy-io/stroppy#234); pgx reads the URL as well.
		tuned := poolURL(dsn, conns)
		if tuned != dsn {
			c.clientAuto = append(c.clientAuto, "url.pool_max_conns", "url.pool_min_conns")
		}
		return tuned
	case "postgres", "picodata":
		c.clientAuto = append(c.clientAuto, setPool(driver, "postgres", map[string]int{"maxConns": conns, "minConns": conns})...)
	case "mysql", "ydb":
		c.clientAuto = append(c.clientAuto, setPool(driver, "sql", map[string]int{"maxOpenConns": conns, "maxIdleConns": conns})...)
	}
	return dsn
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
	pool, _ := driver["pool"].(map[string]any) //nolint:errcheck // absent = new block
	own, _ := driver[family].(map[string]any)  //nolint:errcheck // absent = nothing set
	if pool == nil {
		pool = map[string]any{}
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
// secret-free driver options, and for CockroachDB the pool the URL carries.
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
	if _, query, ok := strings.Cut(w.URL, "?"); ok {
		if values, err := url.ParseQuery(query); err == nil && values.Has("pool_max_conns") {
			settings["url"] = map[string]any{"pool_max_conns": values.Get("pool_max_conns"), "pool_min_conns": values.Get("pool_min_conns")}
		}
	}
	return &run.ClientSetup{DriverType: w.DriverType, Settings: settings, Auto: c.clientAuto}
}

// poolURL adds pgx's pool size to a DSN that does not name one. Only the
// query is parsed, so unresolved ${ip:role:...} hosts stay valid.
func poolURL(dsn string, conns int) string {
	base, query, _ := strings.Cut(dsn, "?")
	values, err := url.ParseQuery(query)
	if err != nil || values.Has("pool_max_conns") {
		return dsn
	}
	values.Set("pool_max_conns", strconv.Itoa(conns))
	values.Set("pool_min_conns", strconv.Itoa(conns))
	return base + "?" + values.Encode()
}

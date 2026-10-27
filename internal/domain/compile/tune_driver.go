package compile

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

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
		return poolURL(dsn, conns)
	case "postgres", "picodata":
		setPool(driver, "postgres", map[string]int{"maxConns": conns, "minConns": conns})
	case "mysql", "ydb":
		setPool(driver, "sql", map[string]int{"maxOpenConns": conns, "maxIdleConns": conns})
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
// in the driver's own block (postgres or sql).
func setPool(driver map[string]any, family string, values map[string]int) {
	pool, _ := driver["pool"].(map[string]any) //nolint:errcheck // absent = new block
	own, _ := driver[family].(map[string]any)  //nolint:errcheck // absent = nothing set
	if pool == nil {
		pool = map[string]any{}
	}
	for key, v := range values {
		if _, set := pool[key]; set {
			continue
		}
		if _, set := own[key]; set {
			continue
		}
		pool[key] = v
	}
	driver["pool"] = pool
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

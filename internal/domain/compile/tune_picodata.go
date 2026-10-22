package compile

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// --- Picodata auto tuning -----------------------------------------------------
//
// docs/tuning/picodata.md. memtx keeps the whole dataset in one arena per
// instance, so the arena is what the machine's RAM is for; the SQL limits
// are cluster-wide settings applied once by ALTER SYSTEM.

// picoMemtxStepMB rounds the auto arena down.
const picoMemtxStepMB = 256

// picoMemtxMinMB is the engine's minimum arena (32M), kept as the floor
// of an auto value on a machine too small for the formula.
//
// doc: https://docs.picodata.io/picodata/26.2/reference/config/#instance_memtx_memory
const picoMemtxMinMB = 32

// picoAutoMemtxMB sizes instance.memtx.memory of each of perMachine
// instances on a machine with memoryMB of RAM:
//
//	((RAM − max(2 GB, 8% RAM)) / perMachine − system_memory − 0.25 GB) / 1.15
//
// rounded down to 256 MB. The OS reserve covers the kernel, Docker, the
// exporters and the agent; system_memory is a separate arena of every
// instance (26.x; 25.3 keeps system spaces in memtx.memory); 0.25 GB is
// the resident vinyl quota and 15% the instance's memory beyond the arena
// (Rust heap of the SQL engine, fibers, portals, the snapshot read view).
// Sizing says real tuples take 5–20% more than their estimate.
//
// doc: https://docs.picodata.io/picodata/stable/admin/sizing/
// doc: https://docs.picodata.io/picodata/26.2/reference/config/#instance_memtx_system_memory
func picoAutoMemtxMB(memoryMB, perMachine, systemMB int) int {
	reserve := max(2048, memoryMB*8/100)
	perInstance := (memoryMB - reserve) / max(1, perMachine)
	arena := int(float64(perInstance-systemMB-256) / 1.15)
	arena -= arena % picoMemtxStepMB
	return max(picoMemtxMinMB, arena)
}

// picoSizeMB reads a Picodata byte size (integer, or K/M/G/T suffixed,
// 1K = 1024) as megabytes; 0 when it does not parse.
func picoSizeMB(v any) int {
	s, ok := v.(string)
	if !ok || s == "" {
		return int(numberOf(v)) >> 20
	}
	mult := 1.0 / (1 << 20)
	switch s[len(s)-1] {
	case 'K':
		mult = 1.0 / 1024
	case 'M':
		mult = 1
	case 'G':
		mult = 1024
	case 'T':
		mult = 1 << 20
	}
	n, err := strconv.ParseFloat(strings.TrimRight(s, "KMGT"), 64)
	if err != nil {
		return 0
	}
	return int(n * mult)
}

// picoSetting is one cluster-wide setting of the init sidecar's
// ALTER SYSTEM.
type picoSetting struct {
	// param is the db.picodata.params@1 field; the setting is the same
	// name unless setting is set.
	param, setting string
	// scale turns the param into the setting's unit.
	scale int
	// auto is the value under tuning auto (0 = none).
	auto int
	// since is the first series that has the setting ("" = every one).
	since string
}

// The auto values of the SQL limits (Picodata: 5000 rows, 50 plans,
// 5 MB of plans, 50 queries).
//
// doc: https://docs.picodata.io/picodata/26.2/reference/db_config/
const (
	picoMotionRowAuto     = 50000
	picoCacheCountAuto    = 200
	picoCacheSizeAutoMB   = 32
	picoConcurrencyFloor  = 50
	picoConcurrencyFactor = 1.5
)

// picoSettings are the cluster-wide SQL limits: the param wins, tuning
// auto fills the rest, a series without the setting skips it (and refuses
// an explicit value). sql_vdbe_opcode_max stays as before: its param has a
// default, so it is always applied.
//
// sql_runtime_concurrency_max caps the queries one instance runs at once
// (default 50, 26.1+): every virtual user of a segment holds one query,
// so the cap must be above the VUs one SQL endpoint receives; 1.5× leaves
// room for the router's own subqueries to storages. sql_motion_row_max is
// raised under auto whatever the workload: it only bounds one query's
// virtual table, so queries below 5000 rows behave the same, and
// validation and batch inserts of large scales stop failing at the limit;
// the memory it can take stays bounded by the concurrency cap.
//
// doc: https://docs.picodata.io/picodata/26.2/reference/db_config/#sql_runtime_concurrency_max
// doc: https://docs.picodata.io/picodata/26.2/reference/db_config/#sql_motion_row_max
// doc: https://docs.picodata.io/picodata/26.2/reference/db_config/#sql_storage_cache_count_max
// doc: https://docs.picodata.io/picodata/26.2/reference/db_config/#sql_storage_cache_size_max
// doc: https://docs.picodata.io/picodata/25.3/reference/db_config/ — no sql_runtime_concurrency_max
func (c *compilation) picoSettings(p map[string]any, endpoints int) ([]string, error) {
	version := strParam(p, "version", "")
	auto := c.tuning() == tuningAuto
	concurrency := max(picoConcurrencyFloor, int(math.Ceil(picoConcurrencyFactor*float64(c.peakVUs())/float64(max(1, endpoints)))))
	stmts := []string{fmt.Sprintf("sql_vdbe_opcode_max = %d", intParam(p, "sql_vdbe_opcode_max", 0))}
	paramsSchema := ""
	if kind, ok := c.in.Catalog.Database(c.in.Database.Kind); ok {
		paramsSchema = kind.ParamsSchema
	}
	for _, s := range []picoSetting{
		{param: "sql_runtime_concurrency_max", auto: concurrency, since: "26.1"},
		{param: "sql_storage_cache_count_max", auto: picoCacheCountAuto},
		{param: "sql_storage_cache_size_max_mb", setting: "sql_storage_cache_size_max", scale: 1 << 20, auto: picoCacheSizeAutoMB},
		{param: "sql_motion_row_max", auto: picoMotionRowAuto},
	} {
		v := intParam(p, s.param, 0)
		supported := s.since == "" || picoSeriesAtLeast(version, s.since)
		switch {
		case v > 0 && !supported:
			return nil, fmt.Errorf("picodata %s has no %s", version, s.param)
		case v == 0 && (!auto || !supported):
			continue
		case v == 0:
			v = s.auto
			c.recordTuned(topology.RoleDB, paramsSchema, s.param, v)
		}
		name, scale := s.param, 1
		if s.setting != "" {
			name = s.setting
		}
		if s.scale > 0 {
			scale = s.scale
		}
		stmts = append(stmts, fmt.Sprintf("%s = %d", name, v*scale))
	}
	return stmts, nil
}

// picoSeriesAtLeast compares YY.MINOR series.
func picoSeriesAtLeast(version, since string) bool {
	parse := func(s string) (int, int) {
		major, minor, _ := strings.Cut(s, ".")
		a, _ := strconv.Atoi(major) //nolint:errcheck // malformed = 0
		b, _ := strconv.Atoi(minor) //nolint:errcheck // malformed = 0
		return a, b
	}
	va, vb := parse(version)
	sa, sb := parse(since)
	return va > sa || (va == sa && vb >= sb)
}

// picoInitScript applies the settings in one admin session; pico.sql
// returns nil and the error instead of raising, so every statement is
// asserted and a refused setting fails the init container.
//
// doc: https://docs.picodata.io/picodata/26.2/reference/api/#pico_sql
// doc: https://docs.picodata.io/picodata/26.2/reference/sql/alter_system/
func picoInitScript(stmts []string, adminSocket string) string {
	lua := make([]string, 0, len(stmts)+1)
	for _, s := range stmts {
		lua = append(lua, "assert(pico.sql([[ALTER SYSTEM SET "+s+"]]))")
	}
	lua = append(lua, `return "STROPPY_SQL_CONFIGURED"`)
	return fmt.Sprintf(`printf '%%s\n' '\lua' '%s' | picodata admin %s | grep -qx -- '- STROPPY_SQL_CONFIGURED'
`, strings.Join(lua, "; "), adminSocket)
}

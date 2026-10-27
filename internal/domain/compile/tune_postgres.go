package compile

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// The auto profile of PostgreSQL and OrioleDB (docs/tuning/postgres.md,
// docs/tuning/orioledb.md). Memory and I/O follow the machine of each
// server; the counters a standby must hold at least at the primary's value
// (connections, worker processes) follow the cluster and the workload, so
// every member gets the same.
//
// doc: https://www.postgresql.org/docs/18/runtime-config-resource.html
// doc: https://www.postgresql.org/docs/18/runtime-config-wal.html
// doc: https://www.postgresql.org/docs/18/hot-standby.html#HOT-STANDBY-ADMIN — standby ≥ primary

const (
	// pgSharedBuffersShare: "a reasonable starting value for shared_buffers
	// is 25% of the memory"; 40% is the A/B candidate (docs/tuning R3).
	pgSharedBuffersShare = 0.25
	// pgCacheShare: effective_cache_size, the planner's view of
	// shared_buffers plus the OS page cache (allocates nothing).
	pgCacheShare = 0.75
	// pgShareStep rounds the memory shares.
	pgShareStep = 128
	// pgWorkMemMB: TPC-C runs point lookups; 16MB leaves room for the
	// occasional sort without multiplying by every backend.
	pgWorkMemMB = 16
	// pgMaintenanceMaxMB caps maintenance_work_mem (CREATE INDEX after the
	// load); autovacuum_work_mem takes half of it, at most 1GB (up to 16 a
	// vacuum never uses more).
	pgMaintenanceMaxMB = 2048
	pgAutovacuumMaxMB  = 1024
	// pgShmMaxMB caps /dev/shm, where parallel query and parallel index
	// builds keep their dynamic shared memory (Docker gives 64MB).
	pgShmMaxMB = 8192
	// pgWALKeepMB keeps WAL for a plain streaming replica that has no slot.
	pgWALKeepMB = 2048
	// pgSSDIOConcurrency is the SSD prefetch depth up to 17; from 18 one
	// process never runs more than io_max_concurrency I/Os (at most 64 by
	// default), so a higher effective_io_concurrency buys nothing.
	pgSSDIOConcurrency   = 200
	pg18SSDIOConcurrency = 64
	// pgAuxProcs covers the auxiliary processes in OrioleDB's undo floor
	// (checkpointer, walwriter, bgwriter, startup, archiver, ...).
	pgAuxProcs = 10
)

// pgCounters are the cluster-wide values of the PostgreSQL family: one set
// for every member, so a standby never runs below its primary.
type pgCounters struct {
	MaxConnections int
	MaxWorkers     int
	WALKeepMB      int
}

// pgCluster computes the counters from the workload and the largest
// database machine of the cluster.
func (c *compilation) pgCluster(oriole bool) pgCounters {
	cpu := 0
	for _, role := range []string{topology.RoleDB, topology.RoleDBReplica} {
		for _, m := range c.machinesOf(role) {
			cpu = max(cpu, c.hostOf(m).CPU)
		}
	}
	// Every virtual user holds a connection; +20 for exporters, Patroni and
	// a psql, rounded up to 50 and never below the upstream 100.
	// doc: runtime-config-connection.html#GUC-MAX-CONNECTIONS
	out := pgCounters{MaxConnections: max(100, ceilTo(c.poolSize()+20, 50)), MaxWorkers: max(8, cpu)}
	if oriole {
		// OrioleDB's bgwriters and recovery pools take worker slots too; with
		// too few it silently falls back to single-process recovery.
		// doc: https://www.orioledb.com/docs/usage/configuration — recovery_pool_size
		out.MaxWorkers = max(8, 2*cpu)
	}
	if len(c.machinesOf(topology.RoleDBReplica)) > 0 {
		out.WALKeepMB = pgWALKeepMB
	}
	return out
}

// tunePostgres puts the auto profile of one PostgreSQL/OrioleDB server on
// the overlay of its postgresql.conf and asks its machine for the huge
// pages its shared memory takes.
//
//nolint:funlen // one flat table of settings, each with its reason
func (c *compilation) tunePostgres(role, machine, schemaID string, oriole bool, overlay map[string]any) {
	if c.tuning() != tuningAuto {
		return
	}
	h := c.hostOf(machine)
	if h.MemoryMB == 0 {
		return
	}
	t := c.tune(role, schemaID, overlay)
	effective := c.effective(role, schemaID)
	final := func(key string) int {
		if v, ok := overlay[key]; ok {
			return int(numberOf(v))
		}
		return int(numberOf(effective[key]))
	}
	ram := h.MemoryMB
	cluster := c.pgCluster(oriole)
	major := schemaMajor(schemaID)

	// Memory.
	shared := share(ram, pgSharedBuffersShare, pgShareStep)
	if oriole {
		shared = c.orioleShared(ram)
	}
	t.set("shared_buffers", shared)
	t.set("effective_cache_size", share(ram, pgCacheShare, pgShareStep))
	t.set("work_mem", pgWorkMemMB)
	t.set("maintenance_work_mem", min(pgMaintenanceMaxMB, share(ram, 1.0/16, 64)))
	t.set("autovacuum_work_mem", min(pgAutovacuumMaxMB, max(1, final("maintenance_work_mem")/2)))
	t.set("huge_pages", "try")

	// WAL: checkpoints by the timer, not by WAL volume (a forced checkpoint
	// restarts full-page images and multiplies WAL on a slow disk).
	// doc: https://www.postgresql.org/docs/18/wal-configuration.html
	if h.DiskGB > 0 {
		t.set("max_wal_size", clamp(h.DiskGB*1024*4/100, 2048, 32768))
		t.set("min_wal_size", final("max_wal_size")/4)
	}
	t.set("checkpoint_timeout", 900)
	t.set("checkpoint_completion_target", 0.9)
	// Every image the recipes run is built --with-lz4: PGDG packages
	// (official Debian postgres and the Patroni image on top of it), the
	// official Alpine build and both orioledb/orioledb Dockerfiles.
	// doc: runtime-config-wal.html#GUC-WAL-COMPRESSION
	t.set("wal_compression", "lz4")
	if cluster.WALKeepMB > 0 {
		t.set("wal_keep_size", cluster.WALKeepMB)
	}

	// Planner and I/O on SSD-class disks.
	if !h.Disk.IsZero() && !strings.Contains(h.DiskType, "hdd") {
		// doc: runtime-config-query.html#GUC-RANDOM-PAGE-COST — "1.1 for SSD"-class storage
		t.set("random_page_cost", 1.1)
		depth := pgSSDIOConcurrency
		if major >= 18 {
			depth = pg18SSDIOConcurrency
		}
		t.set("effective_io_concurrency", depth)
		t.set("maintenance_io_concurrency", depth)
	}
	// OLTP: JIT and parallel plans only add latency to short indexed queries.
	t.set("jit", "off")
	t.set("max_parallel_workers_per_gather", 0)

	// Cluster-wide counters; a standby takes at least the primary's value
	// when the primary's is set by hand.
	primaryID := c.schemaFor(topology.RoleDB, schemaPrefix(schemaID))
	primary := c.tune(topology.RoleDB, primaryID, map[string]any{})
	floor := func(key string, v int) int {
		if role == topology.RoleDBReplica && primary.has(key) {
			v = max(v, int(numberOf(c.effective(topology.RoleDB, primaryID)[key])))
		}
		return v
	}
	t.set("max_connections", floor("max_connections", cluster.MaxConnections))
	t.set("max_worker_processes", floor("max_worker_processes", cluster.MaxWorkers))
	t.set("max_parallel_workers", min(max(1, h.CPU), final("max_worker_processes")))

	// Autovacuum: TPC-C's hot tables bloat within minutes.
	// doc: https://www.postgresql.org/docs/18/runtime-config-autovacuum.html
	t.set("autovacuum_naptime", 10)
	t.set("autovacuum_vacuum_scale_factor", 0.05)
	t.set("autovacuum_analyze_scale_factor", 0.02)
	t.set("autovacuum_max_workers", 5)
	t.set("autovacuum_vacuum_cost_limit", 2000)

	if oriole {
		c.tuneOriole(t, role, h.CPU, ram, final)
	}

	// Huge pages for the whole shared memory segment (OrioleDB allocates its
	// pools in it), 10% over for the rest of the segment.
	// doc: https://www.postgresql.org/docs/18/kernel-resources.html#LINUX-HUGE-PAGES
	if hp, _ := t.overlayOr(effective, "huge_pages").(string); hp != "off" { //nolint:errcheck // absent = try
		segment := final("shared_buffers")
		if oriole {
			segment += final("orioledb_main_buffers") + final("orioledb_undo_buffers")
		}
		c.tuneHost(machine, hostTuning{HugePages: int(math.Ceil(1.10*float64(segment)/2)) + 64})
	}
}

// overlayOr is a key's value as rendered: the overlay over the effective config.
func (t tuner) overlayOr(effective map[string]any, key string) any {
	if v, ok := t.overlay[key]; ok {
		return v
	}
	return effective[key]
}

// orioleShared is shared_buffers of an OrioleDB server: heap pages only
// (the catalog) when OrioleDB is the default engine, an equal split with
// main_buffers when heap tables carry data too.
//
// doc: https://www.orioledb.com/docs/usage/configuration — "Memory configuration"
func (c *compilation) orioleShared(ram int) int {
	if boolParam(c.params(), "default_table_access_method") {
		return 256
	}
	return share(ram, 0.125, pgShareStep)
}

// tuneOriole adds the orioledb.* settings on top of the PostgreSQL ones.
//
// doc: https://www.orioledb.com/docs/usage/configuration
// doc: https://github.com/orioledb/orioledb/blob/main/src/orioledb.c — GUC minimums
func (c *compilation) tuneOriole(t tuner, role string, cpu, ram int, final func(string) int) {
	p := c.params()
	orioleOnly := boolParam(p, "default_table_access_method")
	// The page cache of OrioleDB tables: a quarter of RAM is the guide's
	// start for OrioleDB-only data, 30% leaves the rest to undo, backends
	// and the page cache; with heap tables in play the two caches split 1/8.
	main := share(ram, 0.30, pgShareStep)
	if !orioleOnly {
		main = share(ram, 0.125, pgShareStep)
	}
	if fixed := intParam(p, "main_buffers_mb", 0); fixed > 0 {
		main = fixed
	}
	t.set("orioledb_main_buffers", main)
	// Undo ring: 1/16 of the page cache, at most 1GB, never below the
	// engine's minimum of 16 blocks of 8kB per process slot.
	procs := final("max_connections") + final("max_worker_processes") + final("max_wal_senders") + pgAuxProcs
	if v := final("autovacuum_worker_slots"); v > 0 {
		procs += v // 18 sizes the autovacuum slots apart from max_workers
	} else {
		procs += final("autovacuum_max_workers")
	}
	undoFloor := int(math.Ceil(float64(procs*16*8) / 1024 * 1.10))
	t.set("orioledb_undo_buffers", max(undoFloor, min(1024, final("orioledb_main_buffers")/16)))
	// One MB holds ~65 thousand xid statuses; TPC-C's rate overflows 1MB.
	t.set("orioledb_xid_buffers", 8)
	// Allocated per backend; TPC-C uses no temporary tables.
	t.set("orioledb_temp_buffers", 8)
	// "More than one on machines with many cores"; the disk is the ceiling.
	t.set("orioledb_bgwriter_num_workers", clamp(cpu/4, 1, 4))
	if orioleOnly {
		// "Set 1.0 when only OrioleDB tables are used".
		t.set("orioledb_checkpoint_completion_ratio", 1.0)
	}
	if role == topology.RoleDBReplica {
		// A standby replays WAL all the time: 50–100% of the cores for the
		// recovery pools ("recovery_pool_size"), mostly for rows.
		total := max(2, cpu*3/4)
		pool := int(math.Ceil(float64(total) * 2 / 3))
		t.set("orioledb_recovery_pool_size", pool)
		t.set("orioledb_recovery_idx_pool_size", max(1, total-pool))
		t.set("orioledb_recovery_queue_size", 32)
	}
}

// pgShmMB sizes /dev/shm of a PostgreSQL container under auto (0 = Docker's).
func (c *compilation) pgShmMB(machine string) int64 {
	if c.tuning() != tuningAuto {
		return 0
	}
	return int64(min(c.hostOf(machine).MemoryMB/4, pgShmMaxMB))
}

// patroniDCSParameters are the PostgreSQL parameters Patroni takes only from
// the DCS: from a local config (custom_conf included) they "take no effect".
// Each is the db role's value: set by hand, or computed under auto.
//
// doc: https://patroni.readthedocs.io/en/latest/patroni_configuration.html#important-rules
func (c *compilation) patroniDCSParameters() map[string]any {
	confID := c.schemaFor(topology.RoleDB, "cfg.postgresql.conf@")
	explicit := c.explicitConfig(topology.RoleDB, confID)
	effective := c.effective(topology.RoleDB, confID)
	out := map[string]any{}
	for _, key := range []string{"max_connections", "max_worker_processes", "max_locks_per_transaction", "max_wal_senders", "max_replication_slots", "wal_keep_size", "max_prepared_transactions"} {
		if explicit[key] {
			out[key] = effective[key]
		}
	}
	if c.tuning() == tuningAuto {
		cluster := c.pgCluster(false)
		auto := map[string]any{
			"max_connections": cluster.MaxConnections, "max_worker_processes": cluster.MaxWorkers,
			"max_locks_per_transaction": effective["max_locks_per_transaction"], "max_wal_senders": effective["max_wal_senders"],
			"max_replication_slots": effective["max_replication_slots"],
		}
		if cluster.WALKeepMB > 0 {
			auto["wal_keep_size"] = cluster.WALKeepMB
		}
		for k, v := range auto {
			if _, ok := out[k]; !ok && v != nil {
				out[k] = v
			}
		}
	}
	for k, v := range out {
		s := fmt.Sprint(v)
		if f, ok := v.(float64); ok {
			s = strconv.FormatFloat(f, 'f', -1, 64)
		}
		if k == "wal_keep_size" {
			s += "MB" // a bare number would be megabytes too, the unit keeps it readable
		}
		out[k] = s
	}
	return out
}

// pgSyncStandbys is synchronous_standby_names for n synchronous replicas
// without Patroni: a quorum over the replicas' application names (each
// replica connects with its machine name). Names other than plain
// identifiers must be double-quoted.
//
// doc: runtime-config-replication.html#GUC-SYNCHRONOUS-STANDBY-NAMES
func (c *compilation) pgSyncStandbys(n int) string {
	names := []string{}
	for _, m := range c.machinesOf(topology.RoleDBReplica) {
		names = append(names, strconv.Quote(m))
	}
	if n <= 0 || len(names) == 0 {
		return ""
	}
	return fmt.Sprintf("ANY %d (%s)", min(n, len(names)), strings.Join(names, ", "))
}

// tunePgBouncer sizes the pooler for the workload and the database machine.
//
// doc: https://www.pgbouncer.org/config.html
func (c *compilation) tunePgBouncer(role, schemaID string, overlay map[string]any) {
	if c.tuning() != tuningAuto {
		return
	}
	t := c.tune(role, schemaID, overlay)
	effective := c.effective(role, schemaID)
	// Every virtual user is a client connection; fewer slots refuse them.
	t.set("max_client_conn", max(1000, 4*c.poolSize()))
	cpu := 0
	for _, m := range c.machinesOf(topology.RoleDB) {
		cpu = max(cpu, c.hostOf(m).CPU)
	}
	if cpu > 0 {
		// 2–4 active server connections per database core; the pool is
		// opened up front so the measurement window sees no connect storm.
		t.set("default_pool_size", clamp(4*cpu, 20, 100))
		pool := int(numberOf(t.overlayOr(effective, "default_pool_size")))
		t.set("min_pool_size", pool)
		t.set("reserve_pool_size", 5)
		t.set("max_db_connections", pool+int(numberOf(t.overlayOr(effective, "reserve_pool_size"))))
	}
	// The workload prepares statements over the protocol (pgx); set it so
	// the run does not depend on the build's default (0 before 1.24).
	t.set("max_prepared_statements", 200)
}

// schemaMajor is the major of a versioned schema id (cfg.x@17 → 17).
func schemaMajor(id string) int {
	_, v, _ := strings.Cut(id, "@")
	n, _ := strconv.Atoi(v) //nolint:errcheck // 0 for an unversioned id
	return n
}

// schemaPrefix is a versioned schema id without its version.
func schemaPrefix(id string) string {
	p, _, _ := strings.Cut(id, "@")
	return p + "@"
}

package compile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// The auto tuning of MySQL 8.0/8.4 and MariaDB 10.11/11.4/11.8 (InnoDB):
// memory, redo, I/O budget, background threads and connection limits from
// the machine, the disk, the topology and the workload. The formulas and
// their sources are in docs/tuning/mysql.md and docs/tuning/mariadb.md;
// docs/tuning/README.md §2.3/§2.4 is the agreed profile.
//
// innodb_dedicated_server is not used: 8.0 and 8.4 size the redo log by
// different rules, and the values it picks never reach the run's configs.
// doc: https://dev.mysql.com/doc/refman/8.4/en/innodb-dedicated-server.html

// autoBufferPoolShare is the share of a database machine's RAM the InnoDB
// buffer pool takes in the "auto" mode: MySQL suggests up to ~80% on a
// dedicated server, the rest leaves room for connections and the OS.
// doc: https://dev.mysql.com/doc/refman/8.4/en/innodb-buffer-pool-resize.html
const autoBufferPoolShare = 0.7

const (
	// innodbChunkMB is innodb_buffer_pool_chunk_size (left at its default):
	// MySQL rounds the pool UP to a multiple of chunk × instances, so auto
	// rounds it down first to stay at the intended share.
	// doc: https://dev.mysql.com/doc/refman/8.4/en/innodb-buffer-pool-resize.html
	innodbChunkMB = 128
	// The redo log takes a quarter of the pool within these bounds: 100 MB
	// (MySQL) / 96 MB (MariaDB) forces a checkpoint flush every few seconds
	// under TPC-C; the upper bound limits crash recovery time.
	// doc: https://dev.mysql.com/doc/refman/8.4/en/innodb-redo-log.html
	// doc: https://mariadb.com/docs/server/server-usage/storage-engines/innodb/innodb-redo-log
	minRedoMB, maxMySQLRedoMB, maxMariaRedoMB = 2048, 32768, 16384
	// autoLogBufferMB is the 8.4 default, applied to 8.0 and MariaDB too.
	autoLogBufferMB = 64
	// autoBinlogRetention keeps a day of binary logs: a series of runs on
	// one datadir must not fill the disk with the 30-day default.
	autoBinlogRetention = 86400
	// galeraApplierCap bounds wsrep_slave_threads (vCPU).
	galeraApplierCap = 64
)

// innodbServer is one MySQL/MariaDB server of the stand.
type innodbServer struct {
	role, schemaID, machine string
}

// innodbAuto reports the auto tuning of a MySQL/MariaDB stand. The
// buffer_pool field predates tuning; its stock still means stock.
func (c *compilation) innodbAuto() bool {
	return c.tuning() == tuningAuto && strParam(c.params(), "buffer_pool", "auto") != tuningStock
}

// tuneInnoDB puts the auto values of one server into its config overlay;
// a key the database sets in the role's config is kept.
func (c *compilation) tuneInnoDB(s innodbServer, overlay map[string]any) {
	if !c.innodbAuto() {
		return
	}
	maria := c.in.Database.Kind == catalog.MariaDB
	mode := strParam(c.params(), "replication", "async")
	t := c.tune(s.role, s.schemaID, overlay)
	eff := c.effective(s.role, s.schemaID)
	h := c.hostOf(s.machine)

	pool := c.innodbPool(t, h, eff, maria)
	if pool > 0 {
		redo := clamp(pool/4, minRedoMB, maxMySQLRedoMB)
		if maria {
			redo = clamp(pool/4, minRedoMB, maxMariaRedoMB)
		}
		// The redo log lives on the data disk next to the data and binlog.
		if h.DiskGB > 0 {
			redo = max(8, min(redo, h.DiskGB*1024/10))
		}
		if maria {
			t.set("innodb_log_file_size", redo)
		} else {
			t.set("innodb_redo_log_capacity", redo)
		}
	}
	c.tuneIOCapacity(t, h, eff)
	t.set("innodb_log_buffer_size", autoLogBufferMB)

	conns := innodbMaxConnections(c.peakConnections(), maria)
	t.set("max_connections", conns)
	if t.has("max_connections") {
		conns = int(numberOf(eff["max_connections"]))
	}
	// Every account is granted by % (root, stroppy, exporter, proxy
	// monitors) and clients connect by IP: a reverse lookup per connection
	// only adds latency.
	// doc: https://dev.mysql.com/doc/refman/8.4/en/host-cache.html
	t.set("skip_name_resolve", "ON")
	t.set("binlog_expire_logs_seconds", autoBinlogRetention)

	if maria {
		// MariaDB's sync_binlog is 0: with log_bin on, a commit is fsynced
		// in InnoDB but not in the binlog, and crash recovery rolls back a
		// prepared transaction missing from it. 1 is MySQL's durability.
		// doc: https://mariadb.com/docs/server/ha-and-performance/standard-replication/replication-and-binary-log-system-variables#sync_binlog
		t.set("sync_binlog", 1)
		switch mode {
		case topology.ModeSemiSync:
			// AFTER_SYNC: the replica acks before the storage commit, so no
			// client sees a transaction the replica may lack (lossless).
			// doc: https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication#rpl_semi_sync_master_wait_point
			t.set("rpl_semi_sync_master_wait_point", "AFTER_SYNC")
		case topology.ModeGalera:
			// One applier per vCPU instead of 1: one writer streams write
			// sets, the other nodes apply them single-threaded otherwise.
			// doc: https://mariadb.com/docs/galera-cluster/reference/galera-cluster-system-variables#wsrep_slave_threads
			t.set("wsrep_slave_threads", clamp(h.CPU, 1, galeraApplierCap))
			t.set("wsrep_provider_options", galeraProviderOptions(h))
		}
		return
	}

	// MySQL: values 8.0 and 8.4 default differently are fixed to one
	// profile, so a version comparison measures the version.
	// doc: https://dev.mysql.com/doc/refman/8.4/en/mysql-nutshell.html
	instances := int(numberOf(overlay["innodb_buffer_pool_instances"]))
	if t.has("innodb_buffer_pool_instances") {
		instances = int(numberOf(eff["innodb_buffer_pool_instances"]))
	}
	if instances > 0 {
		// 8.4 default: one page cleaner per buffer pool instance.
		t.set("innodb_page_cleaners", instances)
	}
	if h.CPU > 0 {
		// TPC-C writes a lot of undo; 8.4 dropped the default to 1 on up
		// to 16 CPUs, and a lagging purge grows the history list.
		t.set("innodb_purge_threads", purgeThreads(h.CPU))
	}
	// AHI is a contention point under concurrent writes (OFF since 8.4);
	// change buffering is pointless with the data set in the pool (none
	// since 8.4); fdatasync skips needless metadata flushes (ON since 8.4).
	// doc: https://dev.mysql.com/doc/refman/8.4/en/innodb-parameters.html
	t.set("innodb_adaptive_hash_index", "OFF")
	t.set("innodb_change_buffering", "none")
	t.set("innodb_use_fdatasync", "ON")
	t.set("innodb_flush_neighbors", 0)
	// Descriptors for every connection and two per cached table, with
	// room to spare; the container's nofile allows it.
	// doc: https://dev.mysql.com/doc/refman/8.4/en/server-system-variables.html#sysvar_open_files_limit
	tableCache := int(numberOf(eff["table_open_cache"]))
	t.set("open_files_limit", min(dbNofile, max(65535, 10+conns+2*tableCache)))
	switch mode {
	case topology.ModeSemiSync:
		// doc: https://dev.mysql.com/doc/refman/8.4/en/replication-semisync.html
		t.set("rpl_semi_sync_source_wait_point", "AFTER_SYNC")
	case topology.ModeGroup:
		// 8.0 and 8.4 default differently (EVENTUAL / BEFORE_ON_PRIMARY_FAILOVER,
		// READ_ONLY / OFFLINE_MODE): render one choice explicitly. A
		// consistency the stand already carries (the cluster preset's
		// BEFORE) is kept.
		// doc: https://dev.mysql.com/doc/refman/8.4/en/group-replication-system-variables.html
		if s, _ := eff["group_replication_consistency"].(string); s == "" { //nolint:errcheck // absent = server default
			t.set("group_replication_consistency", "EVENTUAL")
		}
		t.set("group_replication_exit_state_action", "READ_ONLY")
	}
}

// innodbPool sets the buffer pool by the buffer_pool mode and returns the
// pool the server will have (MB, 0 = unknown); MySQL also gets the pool
// instances here, since the pool is rounded to them.
func (c *compilation) innodbPool(t tuner, h host, eff map[string]any, maria bool) int {
	p := c.params()
	pool, auto := 0, false
	switch {
	case t.has("innodb_buffer_pool_size"):
		pool = int(numberOf(eff["innodb_buffer_pool_size"]))
	case strParam(p, "buffer_pool", "auto") == "fixed":
		pool = intParam(p, "buffer_pool_mb", 4096)
	case h.MemoryMB > 0:
		pool, auto = share(h.MemoryMB, autoBufferPoolShare, 1), true
	}
	if pool == 0 {
		return 0
	}
	if !maria && h.CPU > 0 {
		instances := poolInstances(pool, h.CPU)
		if t.has("innodb_buffer_pool_instances") {
			instances = max(1, int(numberOf(eff["innodb_buffer_pool_instances"])))
		}
		t.set("innodb_buffer_pool_instances", instances)
		if auto {
			// Round down: MySQL would round the pool up past the share.
			pool -= pool % (innodbChunkMB * instances)
		}
	}
	if !t.has("innodb_buffer_pool_size") {
		t.set("innodb_buffer_pool_size", pool)
	}
	return pool
}

// poolInstances splits the pool by CPU — half the vCPUs, 2..16 — keeping
// at least 1 GB per instance (MySQL's guidance); a pool up to 1 GB is one.
// doc: https://dev.mysql.com/doc/refman/8.4/en/innodb-parameters.html#sysvar_innodb_buffer_pool_instances
func poolInstances(poolMB, cpu int) int {
	if poolMB <= 1024 {
		return 1
	}
	return max(1, min(clamp((cpu+1)/2, 2, 16), poolMB/1024))
}

// purgeThreads is 4 from 8 vCPUs (the 8.0 default), half the vCPUs below.
func purgeThreads(cpu int) int {
	if cpu >= 8 {
		return 4
	}
	return max(1, cpu/2)
}

// innodbMaxConnections leaves room above the workload's VUs for retries,
// the exporter and proxy monitors: MySQL max(500, 1.5·VU+100), MariaDB
// max(300, 1.5·VU+64). The 151 default is exhausted by 128 VUs plus
// tooling.
func innodbMaxConnections(vus int, maria bool) int {
	if maria {
		return max(300, ceilTo(vus*3, 2)/2+64)
	}
	return max(500, ceilTo(vus*3, 2)/2+100)
}

// tuneIOCapacity sizes the background flush budget from the data disk:
// innodb_io_capacity = min(½ write IOPS, 40% of the write bandwidth in 16 KB
// pages) and _max = min(write IOPS, 70% of it), both in hundreds. The rest
// of the disk is left to redo, binlog and doublewrite, which share it. An
// unknown disk keeps the schema's values.
// doc: https://dev.mysql.com/doc/refman/8.4/en/innodb-configuring-io-capacity.html
// doc: https://mariadb.com/docs/server/server-usage/storage-engines/innodb/innodb-system-variables#innodb_io_capacity
func (c *compilation) tuneIOCapacity(t tuner, h host, eff map[string]any) {
	d := h.Disk
	if d.WriteIOPS <= 0 || d.WriteMBps <= 0 {
		return
	}
	capacity, top := ioCapacity(d)
	// Keep io_capacity <= io_capacity_max next to an explicit one.
	if t.has("innodb_io_capacity") {
		top = max(top, int(numberOf(eff["innodb_io_capacity"])))
	}
	if t.has("innodb_io_capacity_max") {
		capacity = min(capacity, int(numberOf(eff["innodb_io_capacity_max"])))
	}
	t.set("innodb_io_capacity", capacity)
	t.set("innodb_io_capacity_max", top)
}

// ioCapacity is the pair of flush budgets a disk sustains.
func ioCapacity(d catalog.DiskLimits) (capacity, top int) {
	pages := float64(d.WriteMBps) * 1024 / 16
	hundreds := func(v float64) int { return max(100, int(v)/100*100) }
	capacity = hundreds(min(0.5*float64(d.WriteIOPS), 0.4*pages))
	top = max(capacity, hundreds(min(float64(d.WriteIOPS), 0.7*pages)))
	return capacity, top
}

// galeraProviderOptions is wsrep_provider_options of a Galera node (one
// string: provider options are not server variables). Every name below is
// a Galera 4 option of the 26.4 provider MariaDB 10.11/11.4/11.8 ship.
//
//   - gcache.size 2G (128M default, preallocated): a node seconds behind
//     under load gets IST instead of a full SST; at most 5% of the disk;
//   - gcs.fc_limit 128 / fc_factor 0.8 (16 / 1.0): one writer streams
//     write sets, a short queue pauses the whole cluster on every burst;
//   - gcs.fc_single_primary yes: one node takes the writes (the proxy's
//     single writer), so the limit is not rescaled by the cluster size
//     (it replaced gcs.fc_master_slave in Galera 4.10);
//   - evs.suspect/inactive/install_timeout 10s/30s/15s (5s/15s/7.5s): a
//     node stalled on a saturated network disk is not expelled; suspect <
//     inactive and install < inactive stay ordered.
//
// doc: https://mariadb.com/docs/galera-cluster/reference/wsrep-variable-details/wsrep_provider_options
// doc: https://galeracluster.com/library/documentation/galera-parameters.html
func galeraProviderOptions(h host) string {
	gcache := "2G"
	if h.DiskGB > 0 && h.DiskGB*1024/20 < 2048 {
		gcache = fmt.Sprintf("%dM", max(128, h.DiskGB*1024/20))
	}
	return strings.Join([]string{
		"gcache.size=" + gcache, "gcs.fc_limit=128", "gcs.fc_factor=0.8", "gcs.fc_single_primary=yes",
		"evs.suspect_timeout=PT10S", "evs.inactive_timeout=PT30S", "evs.install_timeout=PT15S",
	}, "; ")
}

// mariadbIOUring lets a MariaDB container use io_uring under auto. InnoDB
// on 10.6+ is built for io_uring only; when io_uring_queue_init() fails it
// silently runs with innodb_use_native_aio=OFF (simulated AIO). Docker's
// default seccomp profile blocks io_uring_setup/enter/register (Engine 25+)
// and older kernels charge the rings to RLIMIT_MEMLOCK. A custom seccomp
// profile adding only those three calls is the whole default profile
// inlined into the option (tens of KB, over the RunSpec's 256-character
// security_opt), so the database container runs unconfined; it is the
// stand's own VM and the container already has the host network.
// doc: https://github.com/MariaDB/mariadb-docker/issues/454 (memlock)
// doc: https://jira.mariadb.org/browse/MDEV-36234 (seccomp)
// doc: https://docs.docker.com/engine/security/seccomp/
func (c *compilation) mariadbIOUring(ct *spec.Container) {
	if !c.innodbAuto() || c.in.Database.Kind != catalog.MariaDB {
		return
	}
	if ct.Ulimits == nil {
		ct.Ulimits = map[string]int64{}
	}
	ct.Ulimits["memlock"] = -1
	ct.SecurityOpt = append(ct.SecurityOpt, "seccomp=unconfined")
}

// configValue is a key of a role's config as the run renders it: the
// auto value when tuning set one, the effective config otherwise.
func (c *compilation) configValue(role, schemaID, key string) any {
	if v, ok := c.out.Tuned[role][schemaID][key]; ok {
		return v
	}
	return c.effective(role, schemaID)[key]
}

// explicitConfig is the keys the database sets in a role's config (its
// deviations from the schema defaults).
func (c *compilation) explicitConfig(role, schemaID string) map[string]bool {
	out := map[string]bool{}
	if rc, ok := c.in.Database.Configs[role]; ok {
		values := map[string]json.RawMessage{}
		if json.Unmarshal(rc[schemaID], &values) == nil {
			for k := range values {
				out[k] = true
			}
		}
	}
	return out
}

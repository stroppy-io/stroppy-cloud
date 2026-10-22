package compile

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// --- CockroachDB auto tuning ------------------------------------------------
//
// Memory: the production checklist asks for --cache of 25% or more, allows
// raising it up to 35%, and bounds the pair by (2 × --max-sql-memory) +
// --cache <= 80% of RAM; cockroach-start bounds cache + SQL + TSDB by 75%.
// TPC-C transactions are short, so the SQL budget goes down to 20% and the
// block cache takes the rest: 2×20 + 35 = 75% and 35 + 20 + ~1 = ~56%.
//
// doc: docs.cockroachlabs.com/docs/stable/recommended-production-settings#cache-and-sql-memory-size
// doc: docs.cockroachlabs.com/docs/stable/cockroach-start#flags — --cache, --max-sql-memory

const (
	// crdbCacheLarge is --cache on a machine with crdbCacheLargeMB or more.
	crdbCacheLarge   = "35%"
	crdbCacheSmall   = "30%"
	crdbCacheLargeMB = 32 * 1024
	crdbSQLMemory    = "20%"
)

// Disk: --store size is a soft limit (default 100%), and temporary storage
// "is never counted towards any store usage", so store + temp must fit the
// device. The upstream temp budget is 32GiB; on a 200 GB disk 90% + 32GiB
// does not fit, 80% + min(32GiB, 15%) always does (95% at most).
//
// doc: docs.cockroachlabs.com/docs/stable/cockroach-start — --store size, --max-disk-temp-storage
const (
	crdbStoreSize   = "80%"
	crdbTempPercent = 15
	crdbTempMaxGiB  = 32
)

// crdbTempGiB is --max-disk-temp-storage for a data disk: min(32GiB, 15%).
func crdbTempGiB(diskGB int) int {
	return clamp(diskGB*crdbTempPercent/100, 1, crdbTempMaxGiB)
}

// tuneCockroachNode sizes the start flags of one node from its machine.
func (c *compilation) tuneCockroachNode(machine, schemaID string, overlay map[string]any) {
	t := c.tune(topology.RoleDB, schemaID, overlay)
	h := c.hostOf(machine)
	if h.MemoryMB > 0 {
		cache := crdbCacheSmall
		if h.MemoryMB >= crdbCacheLargeMB {
			cache = crdbCacheLarge
		}
		t.set("cache", cache)
		t.set("max_sql_memory", crdbSQLMemory)
	}
	if h.DiskGB > 0 {
		t.set("store_size", crdbStoreSize)
		t.set("max_disk_temp_storage", fmt.Sprintf("%dGiB", crdbTempGiB(h.DiskGB)))
	}
}

// Cluster settings the official TPC-C guide sets that apply at a hundred
// warehouses on one zone, plus telemetry off for a throwaway stand and the
// isolation stroppy asks for. Values are SQL literals.
//
// doc: docs.cockroachlabs.com/docs/v24.3/performance-benchmarking-with-tpcc-large — "Configure the cluster"
// doc: docs.cockroachlabs.com/docs/stable/cluster-settings
var crdbClusterSettings = map[string]string{
	// Rebalance and up-replication snapshots: 32 MiB/s upstream, the guide
	// uses 256 MiB on local SSDs; network disks of the stand take half.
	"kv.snapshot_rebalance.max_rate": "'128MiB'",
	// No range merges inside the measured window (the guide's setting;
	// queue_enabled is the older name of the same setting).
	"kv.range_merge.queue.enabled": "false",
	// No phone-home from a stand that lives hours.
	"diagnostics.reporting.enabled": "false",
}

// crdbRepeatableRead lets REPEATABLE READ — what stroppy's pgx driver asks
// for — run as REPEATABLE READ instead of being upgraded to SERIALIZABLE,
// so the engine is compared at the isolation the others run at. Named
// sql.txn.snapshot_isolation.enabled before 24.3 (still an alias); 24.1
// has only the developer-only SNAPSHOT setting and gates it on an
// enterprise license, so it is left alone there.
//
// doc: docs.cockroachlabs.com/docs/v24.3/cluster-settings — sql.txn.repeatable_read_isolation.enabled
const crdbRepeatableRead = "sql.txn.repeatable_read_isolation.enabled"

// crdbGCTTL is gc.ttlseconds of the default range: less MVCC garbage to
// scan in a short measurement (the guide's 600; upstream 14400). 600 is
// the smallest value the docs say is regularly tested.
//
// doc: docs.cockroachlabs.com/docs/stable/configure-replication-zones — gc.ttlseconds
const crdbGCTTL = 600

// tuneCockroachCluster fills the cluster-wide part of the schema: the SET
// CLUSTER SETTING block and the default zone. A setting the database names
// in its own cluster_settings wins over the auto one of the same name.
func (c *compilation) tuneCockroachCluster(schemaID string, overlay map[string]any) {
	role := topology.RoleDB
	t := c.tune(role, schemaID, overlay)
	settings := map[string]any{}
	for k, v := range crdbClusterSettings {
		settings[k] = v
	}
	if !cockroach241(c.in.Database.Version) {
		settings[crdbRepeatableRead] = "true"
	}
	if own, ok := c.effective(role, schemaID)["cluster_settings"].(map[string]any); ok {
		for k, v := range own {
			settings[k] = v
		}
	}
	overlay["cluster_settings"] = settings
	c.recordTuned(role, schemaID, "cluster_settings", settings)
	t.set("gc_ttl_seconds", crdbGCTTL)
}

// cockroach241 reports the 24.1 line.
func cockroach241(version string) bool {
	return version == "24.1" || strings.HasPrefix(version, "24.1.")
}

// crdbUnderReplicated counts ranges with fewer live replicas than their
// target across the stores (what `cockroach node status --ranges` sums);
// NULL until every store has published its metrics.
//
// doc: docs.cockroachlabs.com/docs/stable/essential-metrics-self-hosted — ranges.underreplicated
const crdbUnderReplicated = `SELECT sum((metrics->>'ranges.underreplicated')::DECIMAL)::INT FROM crdb_internal.kv_store_status`

// crdbReplicationWait bounds the wait for full replication: 100 × 3 s.
const crdbReplicationWait = 100

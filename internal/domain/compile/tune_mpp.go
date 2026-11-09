package compile

import (
	"math"
	"strconv"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// The auto profile of the Greenplum family (docs/tuning/mpp.md): the
// "benchmark" profile, which follows the one vendor report that publishes its
// GUCs (SynxDB against Greenplum 6.27.1 and 7.1.0, 16 vCPU / 128 GB hosts),
// not the production-safe formulas of the Greenplum guides. The vendor
// formula leaves half of the host's memory unused; the benchmark profile hands
// 90% of it to the segments, and the kernel (host tuning below) is told the
// same.
//
// doc: https://techdocs.broadcom.com/us/en/vmware-tanzu/data-solutions/tanzu-greenplum/7/greenplum-database/admin_guide-wlmgmt_intro.html
// doc: https://docs.synxdata.com/synxdb-4/product-overview/tpc-ds-benchmark-report.html

const (
	// mppVmemShare is the share of a host's RAM all its segment instances
	// together may use (gp_vmem_protect_limit × instances): the report ran
	// 8 × 15000 MB of 128 GB, 91%.
	mppVmemShare = 0.9
	// mppStatementShare and mppMaxStatementShare place statement_mem and
	// max_statement_mem inside gp_vmem_protect_limit: the report used 12000
	// and 14000 of 15000 (80% and 93%). statement_mem is divided by the
	// queries that run at once, so a concurrent workload does not promise the
	// same memory several times.
	mppStatementShare    = 0.75
	mppMaxStatementShare = 0.9
	// mppDefaultStatementMB and mppDefaultWorkMemMB are the engine's own
	// values, the floors of the two.
	mppDefaultStatementMB = 125
	mppDefaultWorkMemMB   = 32
	// mppWorkMemShare and mppWorkMemMaxMB: work_mem is 10% of the host's RAM
	// shared by its instances, at most 1 GB (the avamingli TPC-DS kits).
	mppWorkMemShare = 0.1
	mppWorkMemMaxMB = 1024
	// mppCacheShare is the planner's guess of the OS cache one instance may
	// count on: half of the host's RAM shared by its instances.
	mppCacheShare = 0.5
	// mppRunawayPercent: the runaway detector cancels queries only at the
	// limit itself, as in the Cloudberry TPC-H toolkit; the limit is already
	// 90% of the host.
	mppRunawayPercent = 100
	// mppPacketSize fits a 1500-byte MTU without IP fragmentation (Greenplum's
	// cloud guide); the engine's 8192 assumes jumbo frames.
	mppPacketSize = 1420
	// mppQueueDepth is the interconnect queue depth both ways, as in two
	// independent TPC-DS/TPC-H kits (the engine's 4 and 2, SynxDB 10 and 8).
	mppQueueDepth = 16
	// The fault prober and the segment connect timeout of the SynxDB run: a
	// segment that is slow under load must not be marked down, because
	// without mirrors a segment down stops the cluster.
	mppFTSProbeSeconds       = 600
	mppSegmentConnectSeconds = 1800
	// mppCoordinatorConnections is gpinitsystem's coordinator max_connections;
	// the segments take three times the coordinator's, which the engine
	// requires, plus headroom for the workload and the exporter.
	mppCoordinatorConnections  = 250
	mppSegmentConnectionFactor = 3
	mppConnectionHeadroom      = 50
	// mppMaxWALMB spaces the checkpoints of a bulk load (our choice; the
	// vendors give no load advice): 4 GB against the engine's 1 GB.
	mppMaxWALMB = 4096
	// mppOvercommitMemory is strict overcommit, which the vendor guides ask
	// of every host; mppOvercommitSlack is the formula's share of the
	// segment memory the kernel keeps back for the OS.
	mppOvercommitMemory = 2
	mppOvercommitSlack  = 0.026
	// mppMinFreeShare is vm.min_free_kbytes, 3% of RAM.
	mppMinFreeShare = 0.03
	// mppDirtyBytesRAMGB is where the vendor dirty-page advice changes from
	// ratios to byte counts; the schema has ratios only, so above it the
	// ratios are the nearest to 1.5 GiB and 4 GiB.
	mppDirtyBytesRAMGB = 64
)

// mppMemory is the memory GUCs of one machine, in MB.
type mppMemory struct {
	VmemMB, StatementMB, MaxStatementMB, WorkMB, CacheMB int
}

// mppMemoryOf sizes the instances of a machine with ramMB of RAM: the
// segments of a segment host, or the single coordinator. concurrency is the
// queries that run at once.
func mppMemoryOf(ramMB, instances, concurrency int) mppMemory {
	vmem := int(mppVmemShare * float64(ramMB) / float64(instances))
	maxStatement := int(mppMaxStatementShare * float64(vmem))
	statement := min(maxStatement-1, max(mppDefaultStatementMB, int(mppStatementShare*float64(vmem)/float64(max(1, concurrency)))))
	return mppMemory{
		VmemMB:         vmem,
		StatementMB:    statement,
		MaxStatementMB: maxStatement,
		WorkMB:         clamp(int(mppWorkMemShare*float64(ramMB)/float64(instances)), mppDefaultWorkMemMB, mppWorkMemMaxMB),
		CacheMB:        int(mppCacheShare * float64(ramMB) / float64(instances)),
	}
}

// mppOvercommitRatio is the vendor formula for vm.overcommit_ratio,
// 100 × (RAM − 0.026 × gp_vmem) / RAM, with the memory the instances may use
// in place of gp_vmem.
//
// doc: techdocs.broadcom.com …/best_practices-sysconfig.html — "vm.overcommit_ratio"
func mppOvercommitRatio(ramMB, totalVmemMB int) int {
	return clamp(int(math.Floor(100*(float64(ramMB)-mppOvercommitSlack*float64(totalVmemMB))/float64(ramMB))), 50, 100)
}

// mppDirty is vm.dirty_background_ratio and vm.dirty_ratio: 3 and 10 up to
// 64 GB of RAM (the vendor values); above it the vendor gives bytes
// (1.5 GiB and 4 GiB), which the ratios approximate.
//
// doc: techdocs.broadcom.com …/install_guide-prep_os.html — "vm.dirty_*"
func mppDirty(ramMB int) (background, dirty int) {
	gb := float64(ramMB) / 1024
	if gb <= mppDirtyBytesRAMGB {
		return 3, 10
	}
	background = max(1, int(math.Round(1.5/gb*100)))
	return background, max(background, int(math.Round(4/gb*100)))
}

// tuneMPP sizes the GUC list of a role from one machine of it (the smallest:
// the list is one for the whole role) running instances segment instances,
// and returns the gp_vmem_protect_limit the role ends up with.
func (c *compilation) tuneMPP(role, machine, schemaID string, instances int, overlay map[string]any) int {
	t := c.tune(role, schemaID, overlay)
	h := c.hostOf(machine)
	if h.MemoryMB == 0 {
		return 0
	}
	m := mppMemoryOf(h.MemoryMB, instances, c.peakVUs())
	t.set("gp_vmem_protect_limit", m.VmemMB)
	// The server refuses a statement_mem that is not below max_statement_mem;
	// the template writes max_statement_mem first.
	t.set("max_statement_mem", m.MaxStatementMB)
	t.set("statement_mem", m.StatementMB)
	t.set("work_mem", m.WorkMB)
	t.set("effective_cache_size", m.CacheMB)
	t.set("runaway_detector_activation_percent", mppRunawayPercent)
	t.set("gp_resource_manager", "queue")
	// The workloads run ANALYZE themselves after the load, so the automatic
	// statistics of 6.x engines (on_no_stats) would only analyze each table a
	// second time while it loads.
	t.set("gp_autostats_mode", "none")
	// The 6.x engines run no autovacuum beyond the wraparound guard and
	// gpconfig refuses to change the GUC there.
	if !c.mppLegacyBase() {
		t.set("autovacuum", "off")
	}
	t.set("gp_max_packet_size", mppPacketSize)
	t.set("gp_interconnect_queue_depth", mppQueueDepth)
	t.set("gp_interconnect_snd_queue_depth", mppQueueDepth)
	t.set("gp_fts_probe_timeout", mppFTSProbeSeconds)
	t.set("gp_fts_probe_interval", mppFTSProbeSeconds)
	t.set("gp_segment_connect_timeout", mppSegmentConnectSeconds)
	connections := max(mppCoordinatorConnections, ceilTo(c.clientPools()*c.poolSize()+mppConnectionHeadroom, 50))
	if role == topology.RoleDB {
		connections *= mppSegmentConnectionFactor
	}
	t.set("max_connections", connections)
	// The 6.x engines have checkpoint_segments instead, which gpconfig refuses
	// to change and gpinitsystem (CHECK_POINT_SEGMENTS) sets on the coordinator
	// only: they keep the engine's.
	if !c.mppLegacyBase() {
		t.set("max_wal_size", mppMaxWALMB)
	}
	if t.has("gp_vmem_protect_limit") {
		return intParam(c.effective(role, schemaID), "gp_vmem_protect_limit", m.VmemMB)
	}
	return m.VmemMB
}

// tuneMPPHost tells the kernel of a machine what its instances may use: strict
// overcommit with the vendor ratio for the memory they are given, the
// dirty-page limits and the free-memory floor (THP stays off, the schema
// default).
func (c *compilation) tuneMPPHost(machine string, instances, vmemMB int) {
	h := c.hostOf(machine)
	if h.MemoryMB == 0 || vmemMB == 0 {
		return
	}
	background, dirty := mppDirty(h.MemoryMB)
	overcommit := mppOvercommitMemory
	c.tuneHost(machine, hostTuning{
		Overcommit: &overcommit,
		Fields: map[string]any{
			"vm_overcommit_ratio":       mppOvercommitRatio(h.MemoryMB, instances*vmemMB),
			"vm_dirty_background_ratio": background,
			"vm_dirty_ratio":            dirty,
		},
		Custom: map[string]string{"vm.min_free_kbytes": strconv.Itoa(int(mppMinFreeShare * float64(h.MemoryMB) * 1024))},
	})
}

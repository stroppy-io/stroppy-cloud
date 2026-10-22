package compile

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// --- YDB auto tuning --------------------------------------------------------
//
// docs/tuning/ydb.md §2. Every value lands in one node's config.yaml
// overlay through the tuner, so a key set in the role's config wins.

const (
	// ydbPdiskMinGB is the smallest pdisk YDB uses: below 80 GB a node
	// cannot use the device (the size is only fit for a first look).
	//
	// doc: ydb.tech/docs/en/devops/concepts/system-requirements — "Disk subsystem"
	ydbPdiskMinGB = dbkind.YDBPdiskMinGB
	// ydbSlotsPerPdisk is how many VDisks the storage groups may put on one
	// pdisk. The BS controller's default slot limit is 16 per pdisk
	// (PDiskConfig.ExpectedSlotCount unset); half of it leaves the pdisk's
	// space and IOPS to fewer, busier VDisks and keeps room for self-heal.
	//
	// doc: ydb.tech/docs/en/maintenance/manual/balancing_load — ExpectedSlotCount
	ydbSlotsPerPdisk = 8

	// memory_controller_config defaults the auto values must stay
	// consistent with: shared_cache_min + mem_table_min + activities_limit
	// must stay below soft_limit, and the cache minimums below the target.
	//
	// doc: ydb.tech/docs/en/reference/configuration/memory_controller_config
	ydbMemTableMinPercent  = 1
	ydbActivitiesPercent   = 30
	ydbSoftLimitPercent    = 75
	ydbTargetPercent       = 50
	ydbSharedCacheMaxStock = 50
)

// ydbCPUCount is actor_system_config.cpu_count of a node: the actor
// system runs most of the node's work, but pdisk threads and the gRPC
// API run outside of it, so a storage node keeps one core (two from 9
// vCPU up) for them. A database node keeps one core from 8 vCPU up; below
// that the reserve would eat its only user-pool thread (the COMPUTE row
// for 4 CPUs is System 1 / User 1 / Batch 1 / IC 1).
//
// doc: ydb.tech/docs/en/reference/configuration/actor_system_config
// doc: github.com/ydb-platform/ydb/blob/main/ydb/core/driver_lib/run/auto_config_initializer.cpp
func ydbCPUCount(role string, cpu int) int {
	if role == topology.RoleDB {
		reserve := 1
		if cpu > 8 {
			reserve = 2
		}
		return max(2, cpu-reserve)
	}
	if cpu >= 8 {
		return cpu - 1
	}
	return max(1, cpu)
}

// ydbHardLimitBytes is memory_controller_config.hard_limit_bytes: the
// machine's RAM less what the OS, Docker, the agent and node_exporter
// take (2 GB up to 32 GB of RAM, 3 GB above). The container runs without
// a cgroup limit, so YDB would otherwise count the whole machine as its
// own. 0 when the machine is too small to spare the reserve.
func ydbHardLimitBytes(memoryMB int) int64 {
	reserveMB := 2048
	if memoryMB > 32*1024 {
		reserveMB = 3072
	}
	limitMB := memoryMB - reserveMB
	if limitMB < 1024 {
		return 0
	}
	return int64(limitMB) << 20
}

// tuneYDB puts the auto values of one node's config.yaml on its overlay.
func (c *compilation) tuneYDB(role, schemaID, machine string, overlay map[string]any) {
	h := c.hostOf(machine)
	t := c.tune(role, schemaID, overlay)
	if h.CPU > 0 {
		t.set("cpu_count", ydbCPUCount(role, h.CPU))
	}
	if limit := ydbHardLimitBytes(h.MemoryMB); limit > 0 {
		t.set("memory_hard_limit_bytes", limit)
	}
	if role != topology.RoleDBCompute {
		return
	}
	// A database node serves the tables from its shared cache; a bigger
	// cache spares the data disk the TPC-C reads.
	eff := c.effective(role, schemaID)
	pct := func(key string, def int) int {
		if v, ok := eff[key]; ok {
			return int(numberOf(v))
		}
		return def
	}
	soft := pct("memory_soft_limit_percent", ydbSoftLimitPercent)
	target := pct("memory_target_utilization_percent", ydbTargetPercent)
	if !t.has("memory_target_utilization_percent") && 60 < soft {
		t.set("memory_target_utilization_percent", 60)
		target = 60
	}
	cacheMax := pct("memory_shared_cache_max_percent", ydbSharedCacheMaxStock)
	if !t.has("memory_shared_cache_max_percent") {
		t.set("memory_shared_cache_max_percent", 60)
		cacheMax = 60
	}
	const cacheMin = 25
	if cacheMin+ydbMemTableMinPercent+ydbActivitiesPercent < soft && cacheMin+ydbMemTableMinPercent < target && cacheMin <= cacheMax {
		t.set("memory_shared_cache_min_percent", cacheMin)
	}
	// Online index builds (create_indexes) take up to 10 cores by default;
	// on a database node that starves the rest of the node.
	//
	// doc: ydb.tech/docs/en/reference/configuration/resource_broker_config
	if h.CPU > 0 {
		t.set("resource_broker_build_index_cpu", max(1, h.CPU/4))
	}
}

// ydbPdiskSize is the size of one pdisk file in GB: with auto sizing the
// data disk less 10 GB, 80% of it shared by the node's pdisks, never under
// the 80 GB minimum; otherwise the minimum.
func ydbPdiskSize(diskGB, pdisks int, auto bool) int {
	if !auto || diskGB <= 0 || pdisks <= 0 {
		return ydbPdiskMinGB
	}
	return max(ydbPdiskMinGB, int(0.8*float64(diskGB-10)/float64(pdisks)))
}

// ydbDiskMinGB is the data disk a storage node needs for its pdisks.
func ydbDiskMinGB(pdisks int) int { return dbkind.YDBDiskMinGB(pdisks) }

// ydbGroupWidth is how many VDisks one storage group of the erasure has.
//
// doc: ydb.tech/docs/en/concepts/topology
func ydbGroupWidth(erasure string) int {
	switch erasure {
	case "mirror-3-dc":
		return 9
	case "block-4-2":
		return 8
	default:
		return 1
	}
}

// ydbStorageGroups is how many storage groups the database gets. A set
// storage_groups wins; stock keeps one. Auto gives erasure none four
// independent log streams instead of one (one group puts the log of every
// tablet on one VDisk), and mirror-3-dc two groups per four database
// cores, 6 to 24. Either is cut to what the pdisks hold: ydbSlotsPerPdisk
// VDisks on each, less the static group's.
func ydbStorageGroups(set int, auto bool, erasure string, storageNodes, pdisks, computeNodes, computeCPU int) int {
	if set > 0 {
		return set
	}
	if !auto {
		return 1
	}
	want := 4
	if erasure == "mirror-3-dc" {
		want = clamp(2*computeNodes*computeCPU/4, 6, 24)
	}
	capacity := (storageNodes*pdisks*ydbSlotsPerPdisk - storageNodes) / ydbGroupWidth(erasure)
	return max(1, min(want, capacity))
}

// ydbMedia is the drive spelling of db.ydb.params disk_type: the
// host_configs drive type, the storage pool's pdisk type and the pool
// kind the database is created in.
//
// doc: ydb.tech/docs/en/reference/configuration/host_configs — drive.type SSD | NVME | ROT
func ydbMedia(diskType string) (drive, pdisk, kind string) {
	switch diskType {
	case "NVME":
		return "nvme", "NVME", "nvme"
	case "HDD":
		return "rot", "ROT", "rot"
	default:
		return "ssd", "SSD", "ssd"
	}
}

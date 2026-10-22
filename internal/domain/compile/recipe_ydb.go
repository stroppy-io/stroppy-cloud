package compile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// --- YDB --------------------------------------------------------------------
//
// doc: ydb.tech/docs/en/devops/deployment-options/manual/deploy-ydb-on-premises,
// ydb.tech/docs/en/reference/configuration. Storage nodes hold the pdisks
// (files on the data disk, one per pdisk); database nodes run the tenant
// and register through the node broker of the first storage node. The
// first storage node's sidecar initializes blob storage and creates the
// database. Containers share the host's network (pipelines deploy.go), so
// the interconnect and gRPC skip docker-proxy; Ports only document them.

const (
	ydbGRPCPort   = 2136
	ydbStaticPort = 2135
	ydbICPort     = 19001
	ydbMonPort    = 8765
	ydbDataInner  = "/ydb_data"
	ydbConfPath   = "/opt/ydb/cfg/config.yaml"
)

//nolint:funlen // one cluster: storage nodes, init sidecar, database nodes
func ydbRecipe(c *compilation) error {
	image, err := c.imageOf()
	if err != nil {
		return err
	}
	p := c.params()
	storage := c.machinesOf(topology.RoleDB)
	compute := c.machinesOf(topology.RoleDBCompute)
	if len(storage) == 0 {
		return errs.Newf(errs.CodeInvalid, "params.storage_nodes: at least one storage node").Because("ydb_storage_nodes_required")
	}
	pdisks := intParam(p, "pdisks_per_node", 1)
	erasure := ydbErasure(strParam(p, "fault_tolerance", "none"))
	dbPath := strParam(p, "database_path", ydbDatabasePath)
	auto := c.tuning() == tuningAuto
	driveType, pdiskType, poolKind := ydbMedia(strParam(p, "disk_type", "SSD"))

	// Cluster-wide values shared by every node's config.
	drives := make([]any, 0, pdisks)
	for i := 1; i <= pdisks; i++ {
		drives = append(drives, map[string]any{"path": fmt.Sprintf("%s/pdisk-%d.data", ydbDataInner, i), "type": driveType})
	}
	hosts := make([]any, 0, len(storage)+len(compute))
	for i, m := range storage {
		hosts = append(hosts, map[string]any{
			"host": ip(m), "node_id": i + 1, "host_config_id": 1, "port": ydbICPort, "data_center": c.machineLocation(m), "rack": fmt.Sprintf("rack-%d", i+1),
		})
	}
	stateNodes := make([]any, 0, len(storage))
	for i := range storage {
		stateNodes = append(stateNodes, i+1)
	}
	nto := 1
	switch {
	case erasure == "mirror-3-dc" && len(storage) >= 9:
		nto = 9
	case erasure == "block-4-2" && len(storage) >= 5:
		nto = 5
	case len(storage) >= 3:
		nto = 3
	}
	serviceSet := ydbServiceSet(erasure, len(storage), pdiskType)
	base := map[string]any{
		"static_erasure": erasure,
		"drives":         drives, "host_config_id": 1, "hosts": hosts, "state_storage_nodes": stateNodes, "state_storage_nto_select": nto,
		"storage_pool_types":       []any{map[string]any{"kind": poolKind, "erasure_species": erasure, "pdisk_type": pdiskType, "vdisk_kind": "Default", "box_id": 1}},
		"blob_storage_service_set": serviceSet, "grpc_port": ydbGRPCPort, "interconnect_port": ydbICPort, "monitoring_port": ydbMonPort,
		"spilling_root": ydbDataInner + "/spilling",
	}

	// One sparse file per pdisk on the data disk, sized from that disk
	// (auto_size_pdisks) or at the 80 GB minimum.
	autoSize := true
	if v, ok := p["auto_size_pdisks"].(bool); ok {
		autoSize = v
	}
	for _, m := range storage {
		size := ydbPdiskSize(c.hostOf(m).DiskGB, pdisks, autoSize)
		prep := &strings.Builder{}
		fmt.Fprintf(prep, "mkdir -p %s/ydb\n", dataMount)
		for i := 1; i <= pdisks; i++ {
			fmt.Fprintf(prep, "[ -s %s/ydb/pdisk-%d.data ] || truncate -s %dG %s/ydb/pdisk-%d.data\n", dataMount, i, size, dataMount, i)
		}
		c.out.Spec.HostPrep = append(c.out.Spec.HostPrep, spec.HostPrep{Role: topology.RoleDB, Machine: m, Kind: spec.HostPrepScript, Content: prep.String()})
	}

	schemaID := c.schemaFor(topology.RoleDB, "cfg.ydb.config.yaml@")
	initConf := ""
	for i, m := range storage {
		overlay := cloneMap(base)
		overlay["node_type"] = "STORAGE"
		if auto {
			c.tuneYDB(topology.RoleDB, schemaID, m, overlay)
		}
		conf, err := c.render(topology.RoleDB, schemaID, "conf", overlay)
		if err != nil {
			return err
		}
		if i == 0 {
			initConf = conf
		}
		c.ydbNode(m, auto)
		c.add(spec.Container{
			Scrape: "/counters/prometheus", ScrapePort: ydbMonPort,
			Name: m + "-ydb", Role: topology.RoleDB, Machine: m, Image: image,
			Kind:       spec.ContainerKindDatabase,
			Entrypoint: []string{"/ydbd"},
			Cmd: []string{
				"server", "--yaml-config", ydbConfPath, "--grpc-port", fmt.Sprint(ydbStaticPort),
				"--ic-port", fmt.Sprint(ydbICPort), "--mon-port", fmt.Sprint(ydbMonPort), "--node", fmt.Sprint(i + 1),
			},
			Mounts: []spec.Mount{{Source: dataMount + "/ydb", Target: ydbDataInner}},
			Files:  []spec.File{{Path: ydbConfPath, Content: conf}},
			Ports: []spec.Port{
				{Container: ydbStaticPort, Host: ydbStaticPort}, {Container: ydbICPort, Host: ydbICPort}, {Container: ydbMonPort, Host: ydbMonPort},
			},
			Healthcheck: healthcheck("CMD", "bash", "-ec", fmt.Sprintf("exec 3<>/dev/tcp/127.0.0.1/%d", ydbStaticPort)),
			Restart:     "always",
			Ulimits:     ydbUlimits(auto),
		})
	}
	first := storage[0]
	computeCPU := 0
	if len(compute) > 0 {
		computeCPU = c.hostOf(compute[0]).CPU
	}
	groups := ydbStorageGroups(intParam(p, "storage_groups", 0), auto, erasure, len(storage), pdisks, len(compute), computeCPU)
	init := fmt.Sprintf(`until /ydbd -s grpc://%s:%d admin blobstorage config init --yaml-file %s; do sleep 5; done
until /ydbd -s grpc://%s:%d admin database %s create %s:%d; do sleep 5; done
`, ip(first), ydbStaticPort, ydbConfPath, ip(first), ydbStaticPort, dbPath, poolKind, groups)
	c.add(c.initSidecar(topology.RoleDB, first, image, init, []spec.File{{Path: ydbConfPath, Content: initConf}}, c.dbContainers()))

	c.out.Spec.HostPrep = append(c.out.Spec.HostPrep, spec.HostPrep{Role: topology.RoleDBCompute, Kind: spec.HostPrepScript, Content: "mkdir -p " + dataMount + "/ydb/spilling\n"})
	computeSchema := c.schemaFor(topology.RoleDBCompute, "cfg.ydb.config.yaml@")
	for _, m := range compute {
		overlay := cloneMap(base)
		overlay["node_type"] = "COMPUTE"
		if auto {
			c.tuneYDB(topology.RoleDBCompute, computeSchema, m, overlay)
		}
		conf, err := c.render(topology.RoleDBCompute, computeSchema, "conf", overlay)
		if err != nil {
			return err
		}
		c.ydbNode(m, auto)
		c.add(spec.Container{
			Scrape: "/counters/prometheus", ScrapePort: ydbMonPort,
			Name: m + "-ydb", Role: topology.RoleDBCompute, Machine: m, Image: image,
			Kind:       spec.ContainerKindDatabase,
			Entrypoint: []string{"/ydbd"},
			Cmd: []string{
				"server", "--yaml-config", ydbConfPath, "--grpc-port", fmt.Sprint(ydbGRPCPort),
				"--ic-port", fmt.Sprint(ydbICPort), "--mon-port", fmt.Sprint(ydbMonPort), "--tenant", dbPath,
				"--node-broker", fmt.Sprintf("grpc://%s:%d", ip(first), ydbStaticPort),
				"--data-center", c.machineLocation(m), "--rack", m,
			},
			Files:  []spec.File{{Path: ydbConfPath, Content: conf}},
			Mounts: []spec.Mount{{Source: dataMount + "/ydb", Target: ydbDataInner}},
			Ports: []spec.Port{
				{Container: ydbGRPCPort, Host: ydbGRPCPort}, {Container: ydbICPort, Host: ydbICPort}, {Container: ydbMonPort, Host: ydbMonPort},
			},
			Healthcheck: healthcheck("CMD", "/ydb", "-e", fmt.Sprintf("grpc://127.0.0.1:%d", ydbGRPCPort), "-d", dbPath, "scheme", "ls", dbPath),
			Restart:     "always",
			Ulimits:     ydbUlimits(auto),
			DependsOn:   []string{first + "-init"},
		})
	}
	if len(c.machinesOf(topology.RoleProxy)) > 0 {
		return c.haproxy([]haproxyListener{{Name: "grpc", BindPort: ydbGRPCPort, Servers: c.servers(topology.RoleDBCompute, ydbGRPCPort)}})
	}
	return nil
}

// ydbNode asks the kernel of a YDB machine for what TCMalloc wants under
// auto: transparent huge pages on madvise and memory overcommit.
//
// doc: ydb.tech/docs/en/devops/deployment-options/manual/initial-deployment/deployment-preparation
func (c *compilation) ydbNode(machine string, auto bool) {
	if !auto {
		return
	}
	overcommit := 1
	c.tuneHost(machine, hostTuning{THP: "madvise", Overcommit: &overcommit})
}

// ydbUlimits are the ydbd limits under auto: memlock unlimited. The
// upstream systemd units cap it at 3 GiB (storage) and 30 GiB (database
// node) for a fixed machine; unlimited covers any size the run picks.
// nofile comes from the common database host preparation (1048576, above
// the units' LimitNOFILE=65536).
//
// doc: ydb.tech/docs/en/devops/deployment-options/manual/initial-deployment/deployment-configuration-v1 — systemd units
func ydbUlimits(auto bool) map[string]int64 {
	if !auto {
		return nil
	}
	return map[string]int64{"memlock": -1}
}

func ydbErasure(faultTolerance string) string {
	switch faultTolerance {
	case "mirror-3-dc", "mirror_3_dc":
		return "mirror-3-dc"
	case "block-4-2", "block_4_2":
		return "block-4-2"
	default:
		return "none"
	}
}

// ydbServiceSet is the static group on every node's first pdisk: one ring
// per DC for mirror-3-dc, one ring with a fail domain per node otherwise.
func ydbServiceSet(erasure string, nodes int, pdiskCategory string) json.RawMessage {
	loc := func(node int) map[string]any {
		return map[string]any{"vdisk_locations": []any{map[string]any{"node_id": node, "pdisk_category": pdiskCategory, "path": fmt.Sprintf("%s/pdisk-1.data", ydbDataInner)}}}
	}
	var rings []any
	if erasure == "mirror-3-dc" {
		for dc := 0; dc < 3; dc++ {
			var domains []any
			for n := dc; n < nodes; n += 3 {
				domains = append(domains, loc(n+1))
			}
			rings = append(rings, map[string]any{"fail_domains": domains})
		}
	} else {
		var domains []any
		for n := 1; n <= nodes; n++ {
			domains = append(domains, loc(n))
		}
		rings = []any{map[string]any{"fail_domains": domains}}
	}
	out, _ := json.Marshal(map[string]any{"groups": []any{map[string]any{"erasure_species": erasure, "rings": rings}}}) //nolint:errcheck // literals
	return out
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+2)
	for k, v := range m {
		out[k] = v
	}
	return out
}

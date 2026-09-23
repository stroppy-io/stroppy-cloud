package compile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// --- CockroachDB ------------------------------------------------------------
//
// doc: cockroachlabs.com/docs/stable/cockroach-start, cockroach-init,
// start-a-local-cluster-in-docker. One node = start-single-node; more =
// start --join=all + one init sidecar on the first node. Cluster settings
// from the "settings" template are applied by the same sidecar.

const (
	crdbPort     = 26257
	crdbHTTPPort = 8080
	crdbDataDir  = "/cockroach/cockroach-data"
)

func cockroachRecipe(c *compilation) error {
	image, err := c.imageOf()
	if err != nil {
		return err
	}
	role := topology.RoleDB
	schemaID := c.schemaFor(role, "cfg.cockroach.flags@")
	nodes := c.machinesOf(role)
	join := make([]any, 0, len(nodes))
	for _, m := range nodes {
		join = append(join, fmt.Sprintf("%s:%d", ip(m), crdbPort))
	}
	for _, m := range nodes {
		overlay := map[string]any{
			"store_path": crdbDataDir, "listen_addr": fmt.Sprintf("0.0.0.0:%d", crdbPort), "http_addr": fmt.Sprintf("0.0.0.0:%d", crdbHTTPPort),
			"advertise_addr": fmt.Sprintf("%s:%d", ip(m), crdbPort), "insecure": true,
		}
		if len(nodes) > 1 {
			overlay["join"] = join
		}
		flags, err := c.render(role, schemaID, "conf", overlay)
		if err != nil {
			return err
		}
		cmd := []string{"start"}
		if len(nodes) == 1 {
			cmd = []string{"start-single-node"}
		}
		// The schema places --log last. Its YAML can contain spaces and
		// newlines and must remain one argv element; other flags are scalar.
		plain, logConfig, hasLog := strings.Cut(strings.TrimSpace(flags), " --log=")
		cmd = append(cmd, strings.Fields(plain)...)
		if hasLog {
			cmd = append(cmd, "--log="+logConfig)
		}
		c.add(spec.Container{
			Name: m + "-cockroach", Role: role, Machine: m, Image: image, Cmd: cmd,
			Kind:        spec.ContainerKindDatabase,
			Entrypoint:  []string{"/cockroach/cockroach"},
			Mounts:      []spec.Mount{{Source: dataMount + "/cockroach", Target: crdbDataDir}},
			Ports:       []spec.Port{{Container: crdbPort, Host: crdbPort}, {Container: crdbHTTPPort, Host: crdbHTTPPort}},
			Scrape:      "/_status/vars",
			ScrapePort:  crdbHTTPPort,
			Healthcheck: healthcheck("CMD-SHELL", fmt.Sprintf("curl -sf http://127.0.0.1:%d/health", crdbHTTPPort)),
			Restart:     "always",
		})
	}
	settings, err := c.render(role, schemaID, "settings", nil)
	if err != nil {
		return err
	}
	init := c.first(role)
	var script strings.Builder
	if len(nodes) > 1 {
		clusterName := strParam(c.effective(role, schemaID), "cluster_name", "stroppy")
		fmt.Fprintf(&script, "until cockroach init --insecure --cluster-name=%s --host=%s:%d || cockroach sql --insecure --host=%s:%d -e 'SELECT 1'; do sleep 3; done\n", "'"+strings.ReplaceAll(clusterName, "'", "'\"'\"'")+"'", ip(init), crdbPort, ip(init), crdbPort)
	}
	fmt.Fprintf(&script, "until cockroach sql --insecure --host=%s:%d -e 'SELECT 1'; do sleep 3; done\n", ip(init), crdbPort)
	if strings.TrimSpace(strings.Join(nonComment(settings), "\n")) != "" {
		fmt.Fprintf(&script, "cockroach sql --insecure --host=%s:%d -f %s/settings.sql\n", ip(init), crdbPort, confDir)
	}
	if extra := strParam(c.params(), "init_sql", ""); extra != "" {
		fmt.Fprintf(&script, "cockroach sql --insecure --host=%s:%d -f %s/init.sql\n", ip(init), crdbPort, confDir)
	}
	c.add(c.initSidecar(role, init, image, script.String(), []spec.File{
		{Path: confDir + "/settings.sql", Content: settings},
		{Path: confDir + "/init.sql", Content: strParam(c.params(), "init_sql", "")},
	}, c.dbContainers()))
	if len(c.machinesOf(topology.RoleProxy)) > 0 {
		return c.haproxy([]haproxyListener{{Name: "sql", BindPort: crdbPort, Servers: c.servers(role, crdbPort)}})
	}
	return nil
}

func nonComment(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "--") && !strings.HasPrefix(t, "#") {
			out = append(out, l)
		}
	}
	return out
}

// initSidecar runs a one-shot script inside the database image after the
// databases are up, then idles so the deploy sees it healthy.
func (c *compilation) initSidecar(role, machine, image, script string, files []spec.File, deps []string) spec.Container {
	return spec.Container{
		Name: machine + "-init", Role: role, Machine: machine, Image: image,
		Kind:        spec.ContainerKindAddon,
		Entrypoint:  []string{"/bin/sh"},
		Cmd:         []string{"-c", "set -e\n" + script + "touch /tmp/stroppy-init-done\nwhile true; do sleep 3600; done"},
		Files:       files,
		Healthcheck: &spec.Healthcheck{Cmd: []string{"CMD-SHELL", "test -f /tmp/stroppy-init-done"}, Interval: spec.Duration(10e9), Retries: 100},
		Restart:     "no",
		DependsOn:   deps,
	}
}

// --- Picodata ---------------------------------------------------------------
//
// doc: docs.picodata.io/picodata/stable/reference/config — cluster.tier,
// instance.{name,tier,peer,iproto_listen/advertise,pg}; deployment via
// `picodata run --config`. Instances join through peer = every instance.

const (
	picoIprotoPort = 3301
	picoPgPort     = 4327
	picoHTTPPort   = 8081
	picoDataDir    = "/var/lib/picodata"
)

func picodataRecipe(c *compilation) error {
	image, err := c.imageOf()
	if err != nil {
		return err
	}
	role := topology.RoleDB
	schemaID := c.schemaFor(role, "cfg.picodata.yaml@")
	p := c.params()
	nodes := c.machinesOf(role)
	// Upstream images run as picodata (uid/gid 1000); Docker creates bind
	// mount directories as root unless the data disk is prepared first.
	c.out.Spec.HostPrep = append(c.out.Spec.HostPrep, spec.HostPrep{
		Role: role, Kind: spec.HostPrepScript,
		Content: "install -d -m 0750 -o 1000 -g 1000 " + dataMount + "/picodata\n",
	})
	peers := make([]any, 0, len(nodes))
	for _, m := range nodes {
		peers = append(peers, fmt.Sprintf("%s:%d", ip(m), picoIprotoPort))
	}
	// Tiers from params: name, instances, replication_factor, can_vote,
	// bucket_count, replication_mode; instances are dealt out in order.
	type tier struct {
		name      string
		instances int
		value     map[string]any
	}
	var tiers []tier
	if raw, ok := p["tiers"].([]any); ok {
		for _, t := range raw {
			m, _ := t.(map[string]any) //nolint:errcheck // baked upstream
			v := map[string]any{"name": strParam(m, "name", "default")}
			for _, k := range []string{"replication_factor", "can_vote", "bucket_count", "replication_mode"} {
				if x, ok := m[k]; ok {
					if k == "replication_mode" && strParam(p, "version", "") != "26.2" {
						if x != "async" {
							return fmt.Errorf("picodata %s does not support replication_mode=%v", strParam(p, "version", ""), x)
						}
						continue
					}
					v[k] = x
				}
			}
			tiers = append(tiers, tier{name: v["name"].(string), instances: intParam(m, "instances", 1), value: v}) //nolint:errcheck,forcetypeassert // set above
		}
	}
	if len(tiers) == 0 {
		tiers = []tier{{name: "default", instances: len(nodes), value: map[string]any{"name": "default"}}}
	}
	tierValues := make([]any, 0, len(tiers))
	for _, t := range tiers {
		tierValues = append(tierValues, t.value)
	}
	ti, left := 0, tiers[0].instances
	for _, m := range nodes {
		for left <= 0 && ti < len(tiers)-1 {
			ti++
			left = tiers[ti].instances
		}
		left--
		overlay := map[string]any{
			"cluster_name": "stroppy", "tiers": tierValues, "instance_name": m, "tier": tiers[ti].name, "peer": peers,
			"instance_dir":  picoDataDir,
			"admin_socket":  picoDataDir + "/admin.sock",
			"iproto_listen": fmt.Sprintf("0.0.0.0:%d", picoIprotoPort), "iproto_advertise": fmt.Sprintf("%s:%d", ip(m), picoIprotoPort),
			"http_listen": fmt.Sprintf("0.0.0.0:%d", picoHTTPPort),
			"pg_listen":   fmt.Sprintf("0.0.0.0:%d", picoPgPort), "pg_advertise": fmt.Sprintf("%s:%d", ip(m), picoPgPort),
		}
		if mb := intParam(p, "memtx_memory_mb", 0); mb > 0 {
			overlay["memtx_memory"] = fmt.Sprintf("%dM", mb)
		}
		conf, err := c.render(role, schemaID, "conf", overlay)
		if err != nil {
			return err
		}
		c.add(spec.Container{
			Name: m + "-picodata", Role: role, Machine: m, Image: image,
			Kind:   spec.ContainerKindDatabase,
			Cmd:    []string{"run", "--config", confDir + "/picodata.yaml"},
			Env:    map[string]string{"PICODATA_ADMIN_PASSWORD": picodataPassword},
			Mounts: []spec.Mount{{Source: dataMount + "/picodata", Target: picoDataDir}},
			Files:  []spec.File{{Path: confDir + "/picodata.yaml", Content: conf}},
			Ports: []spec.Port{
				{Container: picoIprotoPort, Host: picoIprotoPort}, {Container: picoPgPort, Host: picoPgPort}, {Container: picoHTTPPort, Host: picoHTTPPort},
			},
			Scrape:      "/metrics",
			ScrapePort:  picoHTTPPort,
			Healthcheck: healthcheck("CMD-SHELL", `printf '%s\n' '\lua' 'return pico.instance_info().current_state.variant' | picodata admin `+picoDataDir+`/admin.sock | grep -qx -- '- Online'`),
			Restart:     "always",
		})
	}
	// Dynamic SQL settings are applied once after every instance is Online.
	// Keeping them in params makes workload-specific limits reviewable.
	first := nodes[0]
	init := fmt.Sprintf(`printf '%%s\n' '\lua' 'pico.sql([[ALTER SYSTEM SET sql_vdbe_opcode_max = %d]]); return "STROPPY_SQL_CONFIGURED"' | picodata admin %s/admin.sock | grep -qx -- '- STROPPY_SQL_CONFIGURED'
`, intParam(p, "sql_vdbe_opcode_max", 45000), picoDataDir)
	sidecar := c.initSidecar(role, first, image, init, nil, c.dbContainers())
	sidecar.Mounts = []spec.Mount{{Source: dataMount + "/picodata", Target: picoDataDir}}
	c.add(sidecar)
	if len(c.machinesOf(topology.RoleProxy)) > 0 {
		port := picoIprotoPort
		if boolParam(p, "pgproto") {
			port = picoPgPort
		}
		return c.haproxy([]haproxyListener{{Name: "sql", BindPort: proxyRW, Servers: c.servers(role, port)}})
	}
	return nil
}

// --- YDB --------------------------------------------------------------------
//
// doc: ydb.tech/docs/en/devops/deployment-options/manual/deploy-ydb-on-premises,
// ydb.tech/docs/en/reference/configuration. Storage nodes hold the pdisks
// (files on the data disk, one per pdisk); database nodes run the tenant
// and register through the node broker of the first storage node. The
// first storage node's sidecar initializes blob storage and creates the
// database.

const (
	ydbGRPCPort   = 2136
	ydbStaticPort = 2135
	ydbICPort     = 19001
	ydbMonPort    = 8765
	ydbPdiskGB    = 40
	ydbDataInner  = "/ydb_data"
	ydbConfPath   = "/opt/ydb/cfg/config.yaml"
)

func ydbRecipe(c *compilation) error {
	image, err := c.imageOf()
	if err != nil {
		return err
	}
	p := c.params()
	storage := c.machinesOf(topology.RoleDB)
	compute := c.machinesOf(topology.RoleDBCompute)
	if len(storage) == 0 {
		return errs.Newf(errs.CodeInvalid, "params.storage_nodes: at least one storage node")
	}
	pdisks := intParam(p, "pdisks_per_node", 1)
	erasure := ydbErasure(strParam(p, "fault_tolerance", "none"))
	dbPath := strParam(p, "database_path", ydbDatabasePath)

	// Cluster-wide values shared by every node's config.
	drives := make([]any, 0, pdisks)
	for i := 1; i <= pdisks; i++ {
		drives = append(drives, map[string]any{"path": fmt.Sprintf("%s/pdisk-%d.data", ydbDataInner, i), "type": "ssd"})
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
	serviceSet := ydbServiceSet(erasure, len(storage), pdisks)
	base := map[string]any{
		"static_erasure": erasure,
		"drives":         drives, "host_config_id": 1, "hosts": hosts, "state_storage_nodes": stateNodes, "state_storage_nto_select": nto,
		"storage_pool_types":       []any{map[string]any{"kind": "ssd", "erasure_species": erasure, "pdisk_type": "SSD", "vdisk_kind": "Default", "box_id": 1}},
		"blob_storage_service_set": serviceSet, "grpc_port": ydbGRPCPort, "interconnect_port": ydbICPort, "monitoring_port": ydbMonPort,
		"spilling_root": ydbDataInner + "/spilling",
	}
	prep := &strings.Builder{}
	fmt.Fprintf(prep, "mkdir -p %s/ydb\n", dataMount)
	for i := 1; i <= pdisks; i++ {
		fmt.Fprintf(prep, "[ -s %s/ydb/pdisk-%d.data ] || truncate -s %dG %s/ydb/pdisk-%d.data\n", dataMount, i, ydbPdiskGB, dataMount, i)
	}
	c.out.Spec.HostPrep = append(c.out.Spec.HostPrep, spec.HostPrep{Role: topology.RoleDB, Kind: spec.HostPrepScript, Content: prep.String()})

	schemaID := c.schemaFor(topology.RoleDB, "cfg.ydb.config.yaml@")
	for i, m := range storage {
		overlay := cloneMap(base)
		overlay["node_type"] = "STORAGE"
		conf, err := c.render(topology.RoleDB, schemaID, "conf", overlay)
		if err != nil {
			return err
		}
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
		})
	}
	first := storage[0]
	groups := intParam(p, "storage_groups", 1)
	init := fmt.Sprintf(`until /ydbd -s grpc://%s:%d admin blobstorage config init --yaml-file %s; do sleep 5; done
until /ydbd -s grpc://%s:%d admin database %s create ssd:%d; do sleep 5; done
`, ip(first), ydbStaticPort, ydbConfPath, ip(first), ydbStaticPort, dbPath, groups)
	initConf, err := c.render(topology.RoleDB, schemaID, "conf", func() map[string]any { o := cloneMap(base); o["node_type"] = "STORAGE"; return o }())
	if err != nil {
		return err
	}
	c.add(c.initSidecar(topology.RoleDB, first, image, init, []spec.File{{Path: ydbConfPath, Content: initConf}}, c.dbContainers()))

	c.out.Spec.HostPrep = append(c.out.Spec.HostPrep, spec.HostPrep{Role: topology.RoleDBCompute, Kind: spec.HostPrepScript, Content: "mkdir -p " + dataMount + "/ydb/spilling\n"})
	computeSchema := c.schemaFor(topology.RoleDBCompute, "cfg.ydb.config.yaml@")
	for _, m := range compute {
		overlay := cloneMap(base)
		overlay["node_type"] = "COMPUTE"
		conf, err := c.render(topology.RoleDBCompute, computeSchema, "conf", overlay)
		if err != nil {
			return err
		}
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
			DependsOn:   []string{first + "-init"},
		})
	}
	if len(c.machinesOf(topology.RoleProxy)) > 0 {
		return c.haproxy([]haproxyListener{{Name: "grpc", BindPort: ydbGRPCPort, Servers: c.servers(topology.RoleDBCompute, ydbGRPCPort)}})
	}
	return nil
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

// ydbServiceSet is the static group: one ring per DC for mirror-3-dc, one
// ring with a fail domain per node otherwise.
func ydbServiceSet(erasure string, nodes, pdisks int) json.RawMessage {
	loc := func(node int) map[string]any {
		return map[string]any{"vdisk_locations": []any{map[string]any{"node_id": node, "pdisk_category": "SSD", "path": fmt.Sprintf("%s/pdisk-1.data", ydbDataInner)}}}
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
	_ = pdisks
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

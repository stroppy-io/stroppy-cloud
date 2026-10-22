package compile

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

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
`, intParam(p, "sql_vdbe_opcode_max", 0), picoDataDir)
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

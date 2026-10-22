package compile

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
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

// picoInstance is one picodata instance: slot k (0-based) of a machine
// listens on the base ports + k and keeps its own directory, so several
// instances share a machine (instances_per_machine).
//
// doc: https://docs.picodata.io/picodata/stable/tutorial/deploy/
type picoInstance struct {
	name, machine string
	slot          int
	tier          int
}

func (i picoInstance) iproto() int { return picoIprotoPort + i.slot }
func (i picoInstance) pg() int     { return picoPgPort + i.slot }
func (i picoInstance) http() int   { return picoHTTPPort + i.slot }

// container is the instance's container name: <machine>-picodata with
// one instance per machine, <machine>-picodata-<k> with more.
func (i picoInstance) container(perMachine int) string {
	if perMachine == 1 {
		return i.machine + "-picodata"
	}
	return fmt.Sprintf("%s-picodata-%d", i.machine, i.slot+1)
}

// dir is the instance's directory on the data disk.
func (i picoInstance) dir(perMachine int) string {
	if perMachine == 1 {
		return dataMount + "/picodata"
	}
	return fmt.Sprintf("%s/picodata/%d", dataMount, i.slot+1)
}

// picoTier is one tier of the params: its cluster.tier value and the
// number of instances started in it.
type picoTier struct {
	name      string
	instances int
	value     map[string]any
}

//nolint:funlen // one recipe: tiers, instances, init, proxy
func picodataRecipe(c *compilation) error {
	image, err := c.imageOf()
	if err != nil {
		return err
	}
	role := topology.RoleDB
	schemaID := c.schemaFor(role, "cfg.picodata.yaml@")
	p := c.params()
	nodes := c.machinesOf(role)
	perMachine := dbkind.PicodataInstancesPerMachine(p)
	tiers, err := c.picoTiers(p, schemaID, len(nodes))
	if err != nil {
		return err
	}
	// Instances go round the machines in tier order, so a tier spreads
	// over as many machines as it has instances.
	var instances []picoInstance
	for ti, t := range tiers {
		for range t.instances {
			i := len(instances)
			in := picoInstance{name: nodes[i%len(nodes)], machine: nodes[i%len(nodes)], slot: i / len(nodes), tier: ti}
			if in.slot >= perMachine {
				return fmt.Errorf("picodata: %d instances do not fit %d machines of %d", i+1, len(nodes), perMachine)
			}
			if perMachine > 1 {
				in.name = fmt.Sprintf("%s-%d", in.machine, in.slot+1)
			}
			instances = append(instances, in)
		}
	}
	if perMachine > 1 {
		// failure_domain keeps the replicas of a replicaset on distinct
		// machines; a replication factor above the machine count could
		// never form a replicaset.
		for _, t := range tiers {
			if rf := intParam(t.value, "replication_factor", 1); rf > len(nodes) {
				return fmt.Errorf("picodata tier %s: replication_factor %d needs %d machines, %d instances per machine give %d", t.name, rf, rf, perMachine, len(nodes))
			}
		}
	}
	// Upstream images run as picodata (uid/gid 1000); Docker creates bind
	// mount directories as root unless the data disk is prepared first.
	dirs := []string{dataMount + "/picodata"}
	for k := 1; perMachine > 1 && k <= perMachine; k++ {
		dirs = append(dirs, fmt.Sprintf("%s/picodata/%d", dataMount, k))
	}
	c.out.Spec.HostPrep = append(c.out.Spec.HostPrep, spec.HostPrep{
		Role: role, Kind: spec.HostPrepScript,
		Content: "install -d -m 0750 -o 1000 -g 1000 " + strings.Join(dirs, " ") + "\n",
	})
	peers := make([]any, 0, len(instances))
	for _, in := range instances {
		peers = append(peers, fmt.Sprintf("%s:%d", ip(in.machine), in.iproto()))
	}
	tierValues := make([]any, 0, len(tiers))
	for _, t := range tiers {
		tierValues = append(tierValues, t.value)
	}
	memtx := ""
	if mb := intParam(p, "memtx_memory_mb", 0); mb > 0 {
		memtx = fmt.Sprintf("%dM", mb)
	}
	auto := c.tuning() == tuningAuto
	containers := make([]string, 0, len(instances))
	for _, in := range instances {
		overlay := map[string]any{
			"cluster_name": "stroppy", "tiers": tierValues, "instance_name": in.name, "tier": tiers[in.tier].name, "peer": peers,
			"default_bucket_count": intParam(p, "default_bucket_count", 3000),
			"instance_dir":         picoDataDir,
			"admin_socket":         picoDataDir + "/admin.sock",
			"iproto_listen":        fmt.Sprintf("0.0.0.0:%d", in.iproto()), "iproto_advertise": fmt.Sprintf("%s:%d", ip(in.machine), in.iproto()),
			"http_listen": fmt.Sprintf("0.0.0.0:%d", in.http()),
			"pg_listen":   fmt.Sprintf("0.0.0.0:%d", in.pg()), "pg_advertise": fmt.Sprintf("%s:%d", ip(in.machine), in.pg()),
		}
		if perMachine > 1 {
			overlay["failure_domain"] = map[string]any{"host": in.machine}
		}
		switch {
		case memtx != "":
			overlay["memtx_memory"] = memtx
		case auto:
			c.tunePicoMemtx(in.machine, schemaID, perMachine, overlay)
		}
		conf, err := c.render(role, schemaID, "conf", overlay)
		if err != nil {
			return err
		}
		name := in.container(perMachine)
		containers = append(containers, name)
		c.add(spec.Container{
			Name: name, Role: role, Machine: in.machine, Image: image,
			Kind:   spec.ContainerKindDatabase,
			Cmd:    []string{"run", "--config", confDir + "/picodata.yaml"},
			Env:    map[string]string{"PICODATA_ADMIN_PASSWORD": picodataPassword},
			Mounts: []spec.Mount{{Source: in.dir(perMachine), Target: picoDataDir}},
			Files:  []spec.File{{Path: confDir + "/picodata.yaml", Content: conf}},
			Ports: []spec.Port{
				{Container: in.iproto(), Host: in.iproto()}, {Container: in.pg(), Host: in.pg()}, {Container: in.http(), Host: in.http()},
			},
			Scrape:      "/metrics",
			ScrapePort:  in.http(),
			Healthcheck: healthcheck("CMD-SHELL", `printf '%s\n' '\lua' 'return pico.instance_info().current_state.variant' | picodata admin `+picoDataDir+`/admin.sock | grep -qx -- '- Online'`),
			Restart:     "always",
		})
	}
	proxied := len(c.machinesOf(topology.RoleProxy)) > 0
	// Dynamic SQL settings are applied once after every instance is Online:
	// cluster-wide ALTER SYSTEM through the first instance. Behind HAProxy
	// the clients spread over every pgproto listener; without it they all
	// reach the first instance.
	endpoints := 1
	if proxied {
		endpoints = len(instances)
	}
	stmts, err := c.picoSettings(p, endpoints)
	if err != nil {
		return err
	}
	first := instances[0]
	sidecar := c.initSidecar(role, first.machine, image, picoInitScript(stmts, picoDataDir+"/admin.sock"), nil, containers)
	sidecar.Mounts = []spec.Mount{{Source: first.dir(perMachine), Target: picoDataDir}}
	c.add(sidecar)
	if !proxied {
		return nil
	}
	servers := make([]haproxyServer, 0, len(instances))
	for _, in := range instances {
		port := in.iproto()
		if boolParam(p, "pgproto") {
			port = in.pg()
		}
		servers = append(servers, haproxyServer{Name: in.name, Address: ip(in.machine), Port: port})
	}
	if err := c.haproxy([]haproxyListener{{Name: "sql", BindPort: proxyRW, Servers: servers}}); err != nil {
		return err
	}
	// The proxy waits for every instance, whatever their names. cfg.haproxy
	// checks one port per listener (the first server's); an instance on
	// another slot is checked on its own port instead.
	for i := range c.out.Spec.Containers {
		ct := &c.out.Spec.Containers[i]
		if ct.Kind != spec.ContainerKindProxy {
			continue
		}
		ct.DependsOn = containers
		for f := range ct.Files {
			for _, s := range servers[1:] {
				if s.Port != servers[0].Port {
					line := fmt.Sprintf("server %s %s:%d check port ", s.Name, s.Address, s.Port)
					ct.Files[f].Content = strings.Replace(ct.Files[f].Content, fmt.Sprintf("%s%d ", line, servers[0].Port), fmt.Sprintf("%s%d ", line, s.Port), 1)
				}
			}
		}
	}
	return nil
}

// pico262TierKeys are the tier keys 26.2.1 added, with the behavior of
// the series before it.
var pico262TierKeys = map[string]string{"replication_mode": "async", "wal_mode": "write"}

// picoTiers reads the params' tiers: name, instances, replication_factor,
// can_vote, bucket_count (default_bucket_count when unset: Picodata's tier
// has its own default of 3000 and does not fall back), replication_mode
// and wal_mode (26.2 only; fsync under tuning auto).
//
// doc: https://docs.picodata.io/picodata/26.2/reference/config/#cluster_tier_tier_bucket_count
// doc: https://docs.picodata.io/picodata/26.2/reference/config/#cluster_tier_tier_wal_mode
func (c *compilation) picoTiers(p map[string]any, schemaID string, machines int) ([]picoTier, error) {
	version := strParam(p, "version", "")
	is262 := version == "26.2"
	buckets := intParam(p, "default_bucket_count", 3000)
	auto := c.tuning() == tuningAuto
	var tiers []picoTier
	tunedWAL := false
	if raw, ok := p["tiers"].([]any); ok {
		for _, t := range raw {
			m, _ := t.(map[string]any) //nolint:errcheck // baked upstream
			name := strParam(m, "name", "default")
			v := map[string]any{"name": name, "bucket_count": buckets}
			for _, k := range []string{"replication_factor", "can_vote", "bucket_count", "replication_mode", "wal_mode"} {
				x, ok := m[k]
				if !ok {
					continue
				}
				// Before 26.2 a tier replicates async and writes without
				// fsync; the keys do not exist, so only that value passes.
				if only, gated := pico262TierKeys[k]; gated && !is262 {
					if x != only {
						return nil, fmt.Errorf("picodata %s does not support %s=%v", version, k, x)
					}
					continue
				}
				v[k] = x
			}
			if _, ok := v["wal_mode"]; !ok && is262 && auto {
				v["wal_mode"] = "fsync"
				tunedWAL = true
			}
			tiers = append(tiers, picoTier{name: name, instances: intParam(m, "instances", 1), value: v})
		}
	}
	if len(tiers) == 0 {
		v := map[string]any{"name": "default", "bucket_count": buckets}
		if is262 && auto {
			v["wal_mode"] = "fsync"
			tunedWAL = true
		}
		tiers = []picoTier{{name: "default", instances: machines, value: v}}
	}
	if tunedWAL {
		values := make([]any, 0, len(tiers))
		for _, t := range tiers {
			values = append(values, t.value)
		}
		c.recordTuned(topology.RoleDB, schemaID, "tiers", values)
	}
	return tiers, nil
}

// tunePicoMemtx sizes memtx.memory of an instance from its machine
// (picoAutoMemtxMB); a memtx_memory set in the role's config wins.
func (c *compilation) tunePicoMemtx(machine, schemaID string, perMachine int, overlay map[string]any) {
	h := c.hostOf(machine)
	if h.MemoryMB == 0 {
		return
	}
	system := 0
	if strings.HasPrefix(schemaID, "cfg.picodata.yaml@26") {
		system = picoSizeMB(c.effective(topology.RoleDB, schemaID)["memtx_system_memory"])
		if system == 0 {
			system = 256
		}
	}
	t := c.tune(topology.RoleDB, schemaID, overlay)
	t.set("memtx_memory", fmt.Sprintf("%dM", picoAutoMemtxMB(h.MemoryMB, perMachine, system)))
}

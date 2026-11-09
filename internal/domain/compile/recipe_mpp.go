package compile

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// --- Greenplum family -------------------------------------------------------
//
// Greenplum, Greengage, WarehousePG, OpenGPDB and Cloudberry on the
// mpp-builds images: one container per machine, host networking, one
// ed25519 key for every sshd. A segment host's container only runs the
// image's sshd. The coordinator's container runs the init script as the CMD
// of the image entrypoint: it waits for every sshd, maps the host names the
// hosts report, runs gpinitsystem over ssh, applies the GUCs, creates the
// workload's login and reports healthy only then, so the workload starts on
// a cluster that takes queries. No mirrors and no standby coordinator.
//
// doc: https://github.com/stroppy-io/mpp-builds — "Image contract", "Reproducing the deployment shape"

const (
	mppDataTarget     = "/data/mpp"
	mppCoordinatorDir = mppDataTarget + "/coordinator"
	mppPrimaryDir     = mppDataTarget + "/primary"
	mppReadyFile      = "/tmp/stroppy-mpp-ready"
	mppSSHDir         = "/etc/mpp/ssh"
	// mppInitAttempts and mppInitInterval bound the wait of the coordinator's
	// healthcheck for the init script: 100 × 20 s (the contract allows 100
	// attempts), about 33 minutes for gpinitsystem and the restart of
	// hundreds of segments.
	mppInitAttempts = 100
	mppInitInterval = 20e9
)

const (
	mppGpinitsystemSchema = "cfg.mpp.gpinitsystem@"
	mppGUCsSchema         = "cfg.mpp.gucs@"
)

func mppRecipe(c *compilation) error {
	kind := c.in.Database.Kind
	image, err := c.imageOf()
	if err != nil {
		return err
	}
	keyword, ok := dbkind.MPPCoordinatorKeyword(kind, c.in.Database.Version)
	if !ok {
		return errs.Newf(errs.CodeInvalid, "%s %s has no image", kind, c.in.Database.Version).Because("db_version_no_image", "kind", string(kind), "version", c.in.Database.Version)
	}
	coordinator, segments := c.first(topology.RoleCoordinator), c.machinesOf(topology.RoleDB)
	key, err := newMPPKey(c.in.RunID.String())
	if err != nil {
		return err
	}
	login := dbkind.MPPLogin()
	smallest := c.smallestMachine(segments)
	perHost := dbkind.MPPSegmentsPerHost(c.params(), c.hostOf(smallest).CPU)

	// The coordinator's keys and the data directories are the deployment's, not
	// the database's to move: the client URL, the exporter and the ports below
	// are built from them.
	initSchema := c.schemaFor(topology.RoleCoordinator, mppGpinitsystemSchema)
	dirs := make([]any, perHost)
	for i := range dirs {
		dirs[i] = mppPrimaryDir
	}
	initOverlay := map[string]any{
		"coordinator_keyword": keyword, "coordinator_hostname": ip(coordinator), "coordinator_directory": mppCoordinatorDir,
		"coordinator_port": dbkind.MPPCoordinatorPort, "data_directories": dirs, "database_name": login.Database,
	}
	gpinit, err := c.render(topology.RoleCoordinator, initSchema, "conf", initOverlay)
	if err != nil {
		return err
	}
	portBase := intParam(c.effective(topology.RoleCoordinator, initSchema), "port_base", dbkind.MPPPrimaryPortBase)

	coordinatorGUCs, err := c.mppGUCs(topology.RoleCoordinator, coordinator, 1, c.machinesOf(topology.RoleCoordinator))
	if err != nil {
		return err
	}
	segmentGUCs, err := c.mppGUCs(topology.RoleDB, smallest, perHost, segments)
	if err != nil {
		return err
	}

	only := "--coordinatoronly"
	if keyword == "MASTER" {
		only = "--masteronly"
	}
	initScript, clusterScript, err := mppScripts(mppScriptParts{
		Ready: mppReadyFile, Self: ip(coordinator), Hosts: strings.Join(ipsOf(append([]string{coordinator}, segments...)), " "), Segments: strings.Join(ipsOf(segments), " "),
		CoordinatorDir: mppCoordinatorDir, PrimaryDir: mppPrimaryDir, ScriptDir: confDir, CoordinatorOnly: only,
		Runner: "${ip:role:" + topology.RoleRunner + "}", Port: dbkind.MPPCoordinatorPort,
		User: login.User, Password: login.Password, Database: login.Database,
	})
	if err != nil {
		return err
	}

	sshFiles := []spec.File{
		{Path: mppSSHDir + "/id_ed25519", Content: key.Private, Mode: "0600"},
		{Path: mppSSHDir + "/id_ed25519.pub", Content: key.Public, Mode: "0644"},
		{Path: mppSSHDir + "/authorized_keys", Content: key.Public, Mode: "0644"},
	}
	segmentContainers := make([]string, 0, len(segments))
	for _, m := range segments {
		ports := make([]spec.Port, perHost)
		for i := range ports {
			ports[i] = spec.Port{Container: portBase + i, Host: portBase + i}
		}
		name := m + "-" + c.engineOf(topology.RoleDB)
		segmentContainers = append(segmentContainers, name)
		c.add(c.mppContainer(name, topology.RoleDB, m, image, sshFiles, ports, spec.Healthcheck{Cmd: []string{"CMD", "mpp-healthcheck"}, Interval: spec.Duration(5e9), Retries: 60}))
	}
	coordinatorContainer := c.mppContainer(coordinator+"-"+c.engineOf(topology.RoleCoordinator), topology.RoleCoordinator, coordinator, image,
		append(sshFiles,
			spec.File{Path: confDir + "/gpinitsystem_config", Content: gpinit},
			spec.File{Path: confDir + "/mpp-gucs-coordinator.conf", Content: coordinatorGUCs},
			spec.File{Path: confDir + "/mpp-gucs-segments.conf", Content: segmentGUCs},
			spec.File{Path: confDir + "/mpp-init.sh", Content: initScript, Mode: "0755"},
			spec.File{Path: confDir + "/mpp-cluster.sh", Content: clusterScript, Mode: "0755"},
		),
		[]spec.Port{{Container: dbkind.MPPCoordinatorPort, Host: dbkind.MPPCoordinatorPort}},
		// Healthy only when sshd answers and the init script has finished.
		spec.Healthcheck{Cmd: []string{"CMD-SHELL", "mpp-healthcheck && test -f " + mppReadyFile}, Interval: spec.Duration(mppInitInterval), Retries: mppInitAttempts})
	coordinatorContainer.Cmd = []string{"bash", confDir + "/mpp-init.sh"}
	coordinatorContainer.DependsOn = segmentContainers
	c.add(coordinatorContainer)

	c.mppExporter(coordinator)
	return nil
}

// mppContainer is a database container of the family: the image, the shared
// key and ssh port, the data volume and the healthcheck.
func (c *compilation) mppContainer(name, role, machine, image string, files []spec.File, ports []spec.Port, health spec.Healthcheck) spec.Container {
	return spec.Container{
		Name: name, Role: role, Machine: machine, Image: image,
		Kind:        spec.ContainerKindDatabase,
		Env:         map[string]string{"MPP_SSH_PORT": strconv.Itoa(dbkind.MPPSSHPort)},
		Mounts:      []spec.Mount{{Source: dataMount + "/mpp", Target: mppDataTarget}},
		Files:       append([]spec.File(nil), files...),
		Ports:       ports,
		Healthcheck: &health,
		Restart:     "always",
	}
}

// mppGUCs renders the GUC list of a role. The list is sized from the smallest
// machine of the role and applies to all of them; the optimizer is the
// database's own parameter and is written in every tuning mode.
func (c *compilation) mppGUCs(role, sizedBy string, instances int, machines []string) (string, error) {
	schemaID := c.schemaFor(role, mppGUCsSchema)
	overlay := map[string]any{}
	if c.tuning() == tuningAuto {
		vmem := c.tuneMPP(role, sizedBy, schemaID, instances, overlay)
		for _, m := range machines {
			c.tuneMPPHost(m, instances, vmem)
		}
	}
	if !c.explicitConfig(role, schemaID)["optimizer"] {
		optimizer := "on"
		if strParam(c.params(), "optimizer", "orca") == "postgres" {
			optimizer = "off"
		}
		overlay["optimizer"] = optimizer
		if c.tuning() == tuningAuto {
			c.recordTuned(role, schemaID, "optimizer", optimizer)
		}
	}
	return c.render(role, schemaID, "conf", overlay)
}

// mppLegacyBase reports a version on the PostgreSQL 9.4 base (the 6.x engines).
func (c *compilation) mppLegacyBase() bool {
	keyword, _ := dbkind.MPPCoordinatorKeyword(c.in.Database.Kind, c.in.Database.Version)
	return keyword == "MASTER"
}

// smallestMachine is the machine with the least memory, then the fewest
// vCPUs (the first one on a tie).
func (c *compilation) smallestMachine(machines []string) string {
	best := machines[0]
	for _, m := range machines[1:] {
		h, b := c.hostOf(m), c.hostOf(best)
		if h.MemoryMB < b.MemoryMB || (h.MemoryMB == b.MemoryMB && h.CPU < b.CPU) {
			best = m
		}
	}
	return best
}

// mppExporter scrapes the coordinator's postgres: the coordinator is the one
// server of the cluster that answers SQL on behalf of the others.
func (c *compilation) mppExporter(coordinator string) {
	login := dbkind.MPPLogin()
	c.add(spec.Container{
		Name: coordinator + "-postgres-exporter", Role: topology.RoleCoordinator, Machine: coordinator, Image: imagePostgresExport,
		Kind:      spec.ContainerKindExporter,
		Env:       map[string]string{"DATA_SOURCE_NAME": fmt.Sprintf("postgresql://%s:%s@127.0.0.1:%d/%s?sslmode=disable", login.User, login.Password, dbkind.MPPCoordinatorPort, login.Database)},
		Cmd:       mppExporterCmd(c.mppLegacyBase()),
		Ports:     []spec.Port{{Container: postgresExporterPort, Host: postgresExporterPort}},
		Scrape:    "/metrics",
		Restart:   "always",
		DependsOn: []string{coordinator + "-" + c.engineOf(topology.RoleCoordinator)},
	})
}

// mppExporterCmd is the postgres exporter's flags for the coordinator: the
// collectors of the postgres family minus the ones that cannot work there. The
// workload's role is no superuser (no pg_ls_waldir for the wal collector), the
// server has no pg_stat_statements, its pg_stat_activity has no row the
// process_idle collector reads, pg_stat_checkpointer is PostgreSQL 17+ and the
// long_running_transactions collector fails on Greenplum 7.1.0 (it scans a NULL
// when no transaction runs). The stat_user_tables collector fails on 8.x and
// Cloudberry ("query plan with multiple segworker groups is not supported") and
// would show the coordinator's share of a table only. On Cloudberry 2.1.0 the
// exporter's settings query also scans a NULL, so pg_exporter_last_scrape_error
// stays 1 there while every collector is served.
//
// A PostgreSQL 9.4 base (the 6.x engines, legacy) lacks the views of the
// wraparound, WAL receiver, replication, slot and vacuum-progress collectors,
// which are left off; its pg_stat_activity has no backend_type, so the
// exporter's own activity metrics do not exist there.
//
// doc: `postgres_exporter --help` of the pinned image (images.go); verified against all nine images
func mppExporterCmd(legacy bool) []string {
	skip := map[string]bool{"stat_statements": true, "process_idle": true, "stat_checkpointer": true, "long_running_transactions": true}
	cmd := []string{fmt.Sprintf("--web.listen-address=:%d", postgresExporterPort), "--no-collector.wal", "--no-collector.stat_user_tables"}
	if legacy {
		skip["database_wraparound"], skip["stat_wal_receiver"] = true, true
		cmd = append(cmd, "--no-collector.replication", "--no-collector.replication_slot", "--no-collector.stat_progress_vacuum")
	}
	for _, collector := range postgresCollectors {
		if !skip[collector] {
			cmd = append(cmd, "--collector."+collector)
		}
	}
	return cmd
}

// ipsOf lists the address placeholders of machines.
func ipsOf(machines []string) []string {
	out := make([]string, len(machines))
	for i, m := range machines {
		out[i] = ip(m)
	}
	return out
}

package topology

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
)

const (
	imagePostgres          = "postgres:17-alpine"
	imageEtcd              = "quay.io/coreos/etcd:v3.5.17"
	imagePgbouncer         = "edoburu/pgbouncer:latest"
	imageNodeExporter      = "prom/node-exporter:latest"
	imagePostgresExporter  = "prometheuscommunity/postgres-exporter:latest"
	imagePgbouncerExporter = "quay.io/prometheuscommunity/pgbouncer-exporter:latest"
	imageBackup            = "postgres:17-alpine"

	defaultPortPostgres          uint32 = 5432
	defaultPortPgbouncer         uint32 = 6432
	defaultPortEtcdClient        uint32 = 2379
	defaultPortEtcdPeer          uint32 = 2380
	defaultPortNodeExporter      uint32 = 9100
	defaultPortPostgresExporter  uint32 = 9187
	defaultPortPgbouncerExporter uint32 = 9127
	defaultPortPatroniAPI        uint32 = 8008

	defaultPostgresUser     = "postgres"
	defaultPostgresPassword = "postgres"
	defaultPostgresDatabase = "postgres"

	defaultEtcdClusterState = "new"
	defaultEtcdClusterToken = "postgres-etcd-cluster"

	containerMetadataDockerIPKey      = "docker.network.ipv4"
	containerMetadataPlacementNodeKey = "docker.placement.node"
	containerMetadataLogicalNameKey   = "docker.logical_name"
)

type postgresPlacementBuilder struct {
	items   []*provision.PlacementIntent_Item
	network *deployment.Network
}

func NewPostgresPlacementBuilder(network *deployment.Network) *postgresPlacementBuilder {
	return &postgresPlacementBuilder{
		items:   make([]*provision.PlacementIntent_Item, 0),
		network: network,
	}
}

func (p *postgresPlacementBuilder) BuildForPostgresInstance(
	t *database.Database_Template_PostgresInstance,
) (*provision.PlacementIntent, error) {
	if t == nil || t.PostgresInstance == nil {
		return nil, fmt.Errorf("database template postgres_instance is nil")
	}
	inst := t.PostgresInstance
	node := inst.GetNode()
	settings := mergeSettings(inst.GetDefaults(), node.GetPostgres().GetSettings())
	postgresqlConf := mergePostgresqlConf(inst.GetDefaults().GetPostgresqlConf(), node.GetPostgres().GetPostgresqlConf())

	item := &provision.PlacementIntent_Item{
		Name:       node.GetName(),
		Hardware:   node.GetHardware(),
		Containers: make([]*provision.Container, 0),
	}

	item.Containers = append(item.Containers, newPostgresContainer(
		node.GetName(),
		settings,
		postgresqlConf,
		provision.Container_PostgresRuntime_ROLE_MASTER,
		0,
		false,
	))

	addMonitoringContainers(item, node.GetMonitoring(), node.GetName())
	addBackupContainer(item, node.GetBackup(), 0)

	p.items = append(p.items, item)
	return p.finalize()
}

func (p *postgresPlacementBuilder) BuildForPostgresCluster(
	t *database.Database_Template_PostgresCluster,
) (*provision.PlacementIntent, error) {
	if t == nil || t.PostgresCluster == nil {
		return nil, fmt.Errorf("database template postgres_cluster is nil")
	}
	cluster := t.PostgresCluster
	defaults := cluster.GetDefaults()

	// Count etcd nodes for cluster_size
	etcdClusterSize := uint32(0)
	for _, node := range cluster.GetNodes() {
		if node.GetEtcd() != nil {
			etcdClusterSize++
		}
	}

	replicaIndex := uint32(0)
	etcdIndex := uint32(0)
	backupIndex := 0
	pgbouncerIndex := 0

	for _, node := range cluster.GetNodes() {
		item := &provision.PlacementIntent_Item{
			Name:       node.GetName(),
			Hardware:   node.GetHardware(),
			Containers: make([]*provision.Container, 0),
		}

		// Postgres service
		if pg := node.GetPostgres(); pg != nil {
			settings := mergeSettings(defaults, pg.GetSettings())
			postgresqlConf := mergePostgresqlConf(defaults.GetPostgresqlConf(), pg.GetPostgresqlConf())

			role := provision.Container_PostgresRuntime_ROLE_MASTER
			idx := uint32(0)
			monitor := false
			if pg.GetRole() == database.Postgres_PostgresService_ROLE_REPLICA {
				role = provision.Container_PostgresRuntime_ROLE_REPLICA
				idx = replicaIndex
				replicaIndex++
			}

			item.Containers = append(item.Containers, newPostgresContainer(
				node.GetName(), settings, postgresqlConf, role, idx, monitor,
			))
		}

		// Etcd service
		if etcdSvc := node.GetEtcd(); etcdSvc != nil {
			etcdIndex++
			etcdRuntime := &provision.Container_EtcdRuntime{
				ClusterSize: etcdClusterSize,
				NodeIndex:   etcdIndex,
			}
			if cfg := etcdSvc.GetConfig(); cfg != nil && cfg.BaseClientPort != nil {
				etcdRuntime.BaseClientPort = cfg.BaseClientPort
			}
			item.Containers = append(item.Containers, &provision.Container{
				Id:      "etcd-" + strconv.Itoa(int(etcdIndex)),
				Name:    "etcd-" + strconv.Itoa(int(etcdIndex)),
				Image:   imageEtcd,
				Monitor: etcdSvc.GetMonitor(),
				Runtime: &provision.Container_Etcd{
					Etcd: etcdRuntime,
				},
			})
			if etcdSvc.GetMonitor() {
				item.Containers = append(item.Containers, NewNodeExporterContainer("etcd-"+strconv.Itoa(int(etcdIndex)), true))
			}
		}

		// Pgbouncer service
		if pgbSvc := node.GetPgbouncer(); pgbSvc != nil {
			pgbouncerIndex++
			containerID := "pgbouncer-" + strconv.Itoa(pgbouncerIndex)
			item.Containers = append(item.Containers, &provision.Container{
				Id:      containerID,
				Name:    containerID,
				Image:   imagePgbouncer,
				Monitor: pgbSvc.GetMonitor(),
				Runtime: &provision.Container_Pgbouncer{
					Pgbouncer: &provision.Container_PgbouncerRuntime{
						Config: pgbSvc.GetConfig(),
					},
				},
			})
			if pgbSvc.GetMonitor() {
				item.Containers = append(item.Containers, &provision.Container{
					Id:      "pgbouncer-exporter-" + strconv.Itoa(pgbouncerIndex),
					Name:    "pgbouncer-exporter-" + strconv.Itoa(pgbouncerIndex),
					Image:   imagePgbouncerExporter,
					Monitor: true,
					Runtime: &provision.Container_PgbouncerExporter{
						PgbouncerExporter: &provision.Container_PgbouncerExporterRuntime{
							Enabled: true,
							Port:    defaultPortPgbouncerExporter,
						},
					},
				})
			}
		}

		// Backup service
		if backupSvc := node.GetBackup(); backupSvc != nil {
			backupIndex++
			addBackupContainer(item, backupSvc, backupIndex)
		}

		// Monitoring service
		addMonitoringContainers(item, node.GetMonitoring(), node.GetName())

		p.items = append(p.items, item)
	}

	return p.finalize()
}

func (p *postgresPlacementBuilder) finalize() (*provision.PlacementIntent, error) {
	items, err := p.finalizeItems()
	if err != nil {
		return nil, err
	}
	connStr, err := p.resolveRuntimeConfig(items)
	if err != nil {
		return nil, err
	}
	return &provision.PlacementIntent{
		Items:            items,
		Network:          p.network,
		ConnectionString: connStr,
	}, nil
}

func (p *postgresPlacementBuilder) finalizeItems() ([]*provision.PlacementIntent_Item, error) {
	ips := p.network.GetIps()
	if len(ips) < len(p.items) {
		return nil, fmt.Errorf("network has %d ips, but %d items are required", len(ips), len(p.items))
	}
	for i, item := range p.items {
		if ips[i] == nil || ips[i].GetValue() == "" {
			return nil, fmt.Errorf("network ip at index %d is empty", i)
		}
		item.InternalIp = ips[i]
		for _, c := range item.GetContainers() {
			ensureMetadata(c)
			c.Metadata[containerMetadataDockerIPKey] = item.GetInternalIp().GetValue()
			c.Metadata[containerMetadataPlacementNodeKey] = item.GetName()
			c.Metadata[containerMetadataLogicalNameKey] = containerLogicalName(c)
		}
	}
	return p.items, nil
}

func (p *postgresPlacementBuilder) resolveRuntimeConfig(items []*provision.PlacementIntent_Item) (string, error) {
	type etcdMember struct {
		name       string
		ip         string
		clientPort uint32
		peerPort   uint32
	}
	var members []etcdMember
	var masterIP string

	for _, item := range items {
		itemIP := item.GetInternalIp().GetValue()
		for _, c := range item.GetContainers() {
			if pg := c.GetPostgres(); pg != nil && pg.GetRole() == provision.Container_PostgresRuntime_ROLE_MASTER {
				masterIP = itemIP
			}
			if e := c.GetEtcd(); e != nil {
				clientPort := e.GetBaseClientPort()
				if clientPort == 0 {
					clientPort = defaultPortEtcdClient
				}
				peerPort := e.GetPeerPort()
				if peerPort == 0 {
					peerPort = defaultPortEtcdPeer
				}
				members = append(members, etcdMember{
					name:       c.GetName(),
					ip:         itemIP,
					clientPort: clientPort,
					peerPort:   peerPort,
				})
			}
		}
	}

	etcdInitialCluster := make([]string, 0, len(members))
	etcdHosts := make([]string, 0, len(members))
	for _, m := range members {
		etcdInitialCluster = append(etcdInitialCluster, fmt.Sprintf("%s=http://%s:%d", m.name, m.ip, m.peerPort))
		etcdHosts = append(etcdHosts, fmt.Sprintf("%s:%d", m.ip, m.clientPort))
	}
	initialClusterValue := strings.Join(etcdInitialCluster, ",")
	etcdHostsValue := strings.Join(etcdHosts, ",")

	// Track the first pgbouncer endpoint for the connection string.
	var pgbouncerConnIP string
	var pgbouncerConnPort uint32

	for _, item := range items {
		itemIP := item.GetInternalIp().GetValue()
		hasPgbouncer := false
		var pgbouncerPort = defaultPortPgbouncer

		for _, c := range item.GetContainers() {
			if pb := c.GetPgbouncer(); pb != nil {
				hasPgbouncer = true
				if port := pb.GetConfig().GetPort(); port != 0 {
					pgbouncerPort = port
				}
				if pgbouncerConnIP == "" {
					pgbouncerConnIP = itemIP
					pgbouncerConnPort = pgbouncerPort
				}
			}
		}

		for _, c := range item.GetContainers() {
			switch {
			case c.GetEtcd() != nil:
				e := c.GetEtcd()
				clientPort := e.GetBaseClientPort()
				if clientPort == 0 {
					clientPort = defaultPortEtcdClient
				}
				peerPort := e.GetPeerPort()
				if peerPort == 0 {
					peerPort = defaultPortEtcdPeer
				}
				ensureEnv(c)
				c.Env["ETCD_NAME"] = c.GetName()
				c.Env["ETCD_INITIAL_CLUSTER"] = initialClusterValue
				c.Env["ETCD_INITIAL_CLUSTER_STATE"] = defaultEtcdClusterState
				c.Env["ETCD_INITIAL_CLUSTER_TOKEN"] = defaultEtcdClusterToken
				c.Env["ETCD_LISTEN_PEER_URLS"] = fmt.Sprintf("http://0.0.0.0:%d", peerPort)
				c.Env["ETCD_INITIAL_ADVERTISE_PEER_URLS"] = fmt.Sprintf("http://%s:%d", itemIP, peerPort)
				c.Env["ETCD_LISTEN_CLIENT_URLS"] = fmt.Sprintf("http://0.0.0.0:%d", clientPort)
				c.Env["ETCD_ADVERTISE_CLIENT_URLS"] = fmt.Sprintf("http://%s:%d", itemIP, clientPort)

			case c.GetPostgres() != nil:
				ensureEnv(c)
				c.Env["POSTGRES_USER"] = defaultPostgresUser
				c.Env["POSTGRES_PASSWORD"] = defaultPostgresPassword
				c.Env["POSTGRES_DB"] = defaultPostgresDatabase
				postgresSettings := c.GetPostgres().GetSettings()
				if postgresSettings.GetPatroni().GetEnabled() {
					if len(etcdHosts) == 0 {
						return "", fmt.Errorf("patroni is enabled but etcd endpoints are empty")
					}
					c.Env["PATRONI_NAME"] = c.GetName()
					c.Env["PATRONI_ETCD3_HOSTS"] = etcdHostsValue
					c.Env["PATRONI_POSTGRESQL_CONNECT_ADDRESS"] = fmt.Sprintf("%s:%d", itemIP, defaultPortPostgres)
					c.Env["PATRONI_RESTAPI_CONNECT_ADDRESS"] = fmt.Sprintf("%s:%d", itemIP, defaultPortPatroniAPI)
				}
				if err := applyPostgresqlConf(c, postgresSettings); err != nil {
					return "", err
				}

			case c.GetPgbouncer() != nil:
				if masterIP == "" {
					return "", fmt.Errorf("pgbouncer is configured but postgres master endpoint is not found")
				}
				ensureEnv(c)
				c.Env["PGBOUNCER_UPSTREAM_HOST"] = masterIP
				c.Env["PGBOUNCER_UPSTREAM_PORT"] = fmt.Sprintf("%d", defaultPortPostgres)

			case c.GetPostgresExporter() != nil:
				ensureEnv(c)
				target := fmt.Sprintf("%s:%d", itemIP, defaultPortPostgres)
				if masterIP != "" {
					target = fmt.Sprintf("%s:%d", masterIP, defaultPortPostgres)
				}
				c.Env["PG_EXPORTER_TARGET"] = target

			case c.GetPgbouncerExporter() != nil:
				if !hasPgbouncer {
					continue
				}
				ensureEnv(c)
				c.Env["PGBOUNCER_EXPORTER_TARGET"] = fmt.Sprintf("%s:%d", itemIP, pgbouncerPort)
			}
		}
	}

	if masterIP == "" {
		return "", fmt.Errorf("postgres master not found in placement items")
	}

	connHost := masterIP
	connPort := defaultPortPostgres
	if pgbouncerConnIP != "" {
		connHost = pgbouncerConnIP
		connPort = pgbouncerConnPort
	}
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		defaultPostgresUser, defaultPostgresPassword,
		connHost, connPort,
		defaultPostgresDatabase)

	return connStr, nil
}

// mergeSettings returns per-node settings if provided, otherwise falls back to defaults.
func mergeSettings(defaults, override *database.Postgres_Settings) *database.Postgres_Settings {
	if override != nil {
		return override
	}
	return defaults
}

// mergePostgresqlConf merges cluster-level conf with per-node conf. Per-node keys win.
func mergePostgresqlConf(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	merged := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}

func newPostgresContainer(
	nodeName string,
	settings *database.Postgres_Settings,
	postgresqlConf map[string]string,
	role provision.Container_PostgresRuntime_Role,
	replicaIndex uint32,
	monitor bool,
) *provision.Container {
	// Build merged settings with postgresql_conf applied
	mergedSettings := settings
	if len(postgresqlConf) > 0 && (settings == nil || len(settings.GetPostgresqlConf()) == 0) {
		// Need to attach conf to settings for downstream consumption
		if settings != nil {
			// Clone to avoid mutating the original
			mergedSettings = &database.Postgres_Settings{
				Version:        settings.GetVersion(),
				StorageEngine:  settings.GetStorageEngine(),
				Patroni:        settings.GetPatroni(),
				PostgresqlConf: postgresqlConf,
			}
		}
	}

	return &provision.Container{
		Id:      nodeName + "-container",
		Name:    nodeName + "-container",
		Image:   imagePostgres,
		Monitor: monitor,
		Runtime: &provision.Container_Postgres{
			Postgres: &provision.Container_PostgresRuntime{
				Role:         role,
				Settings:     mergedSettings,
				ReplicaIndex: replicaIndex,
			},
		},
	}
}

func addMonitoringContainers(item *provision.PlacementIntent_Item, monitoring *database.Postgres_MonitoringService, suffix string) {
	if monitoring == nil {
		return
	}
	if ne := monitoring.GetNodeExporter(); ne != nil {
		port := ne.GetPort()
		if port == 0 {
			port = defaultPortNodeExporter
		}
		item.Containers = append(item.Containers, &provision.Container{
			Id:      "node-exporter-" + suffix,
			Name:    "node-exporter-" + suffix,
			Image:   imageNodeExporter,
			Monitor: true,
			Runtime: &provision.Container_NodeExporter{
				NodeExporter: &provision.Container_NodeExporterRuntime{
					Enabled: true,
					Port:    port,
				},
			},
		})
	}
	if pe := monitoring.GetPostgresExporter(); pe != nil {
		port := pe.GetPort()
		if port == 0 {
			port = defaultPortPostgresExporter
		}
		item.Containers = append(item.Containers, &provision.Container{
			Id:      "postgres-exporter-" + suffix,
			Name:    "postgres-exporter-" + suffix,
			Image:   imagePostgresExporter,
			Monitor: true,
			Runtime: &provision.Container_PostgresExporter{
				PostgresExporter: &provision.Container_PostgresExporterRuntime{
					Enabled:            true,
					Port:               port,
					CustomQueriesPaths: pe.GetCustomQueriesPaths(),
				},
			},
		})
	}
}

func addBackupContainer(item *provision.PlacementIntent_Item, backup *database.Postgres_BackupService, index int) {
	if backup == nil || backup.GetConfig() == nil {
		return
	}
	suffix := strconv.Itoa(index)
	if index == 0 {
		suffix = item.GetName()
	}
	item.Containers = append(item.Containers, &provision.Container{
		Id:      "backup-" + suffix,
		Name:    "backup-" + suffix,
		Image:   imageBackup,
		Monitor: false,
		Runtime: &provision.Container_Backup{
			Backup: &provision.Container_BackupRuntime{
				Config: backup.GetConfig(),
			},
		},
	})
}

func NewNodeExporterContainer(suffix string, monitor bool) *provision.Container {
	return &provision.Container{
		Id:      "node-exporter-" + suffix,
		Name:    "node-exporter-" + suffix,
		Image:   imageNodeExporter,
		Monitor: monitor,
		Runtime: &provision.Container_NodeExporter{
			NodeExporter: &provision.Container_NodeExporterRuntime{
				Enabled: true,
				Port:    defaultPortNodeExporter,
			},
		},
	}
}

func ensureEnv(c *provision.Container) {
	if c.Env == nil {
		c.Env = make(map[string]string)
	}
}

func ensureMetadata(c *provision.Container) {
	if c.Metadata == nil {
		c.Metadata = make(map[string]string)
	}
}

func containerLogicalName(c *provision.Container) string {
	if c.GetName() != "" {
		return c.GetName()
	}
	if c.GetId() != "" {
		return c.GetId()
	}
	return "container"
}

func applyPostgresqlConf(c *provision.Container, settings *database.Postgres_Settings) error {
	if c == nil || settings == nil || len(settings.GetPostgresqlConf()) == 0 {
		return nil
	}

	if settings.GetPatroni().GetEnabled() {
		ensureEnv(c)
		encoded, err := json.Marshal(settings.GetPostgresqlConf())
		if err != nil {
			return fmt.Errorf("encode postgresql_conf for %q: %w", c.GetName(), err)
		}
		c.Env["PATRONI_POSTGRESQL_PARAMETERS"] = string(encoded)
		return nil
	}

	c.Args = append(c.Args, postgresqlConfArgs(settings.GetPostgresqlConf())...)
	return nil
}

func postgresqlConfArgs(conf map[string]string) []string {
	keys := make([]string, 0, len(conf))
	for k := range conf {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	args := make([]string, 0, len(conf)*2)
	for _, k := range keys {
		args = append(args, "-c", fmt.Sprintf("%s=%s", k, conf[k]))
	}
	return args
}

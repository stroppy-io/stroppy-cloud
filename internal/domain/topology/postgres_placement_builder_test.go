package topology

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
)

func ip(v string) *deployment.Ip { return &deployment.Ip{Value: v} }

func hw(cores, mem, disk uint32) *deployment.Hardware {
	return &deployment.Hardware{Cores: cores, Memory: mem, Disk: disk}
}

func networkWithIPs(ips ...string) *deployment.Network {
	out := make([]*deployment.Ip, len(ips))
	for i, v := range ips {
		out[i] = ip(v)
	}
	return &deployment.Network{
		Identifier: &deployment.Identifier{Id: "net-1", Name: "test-net"},
		Cidr:       &deployment.Cidr{Value: "10.0.0.0/24"},
		Ips:        out,
	}
}

func pgSettings() *database.Postgres_Settings {
	return &database.Postgres_Settings{
		Version:       database.Postgres_Settings_VERSION_17,
		StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_HEAP,
	}
}

func patroniSettings() *database.Postgres_Settings {
	s := pgSettings()
	s.Patroni = &database.Postgres_Settings_Patroni{Enabled: true}
	return s
}

func findContainer(items []*provision.PlacementIntent_Item, itemName, containerID string) *provision.Container {
	for _, it := range items {
		if it.GetName() != itemName {
			continue
		}
		for _, c := range it.GetContainers() {
			if c.GetId() == containerID {
				return c
			}
		}
	}
	return nil
}

func findItem(items []*provision.PlacementIntent_Item, name string) *provision.PlacementIntent_Item {
	for _, it := range items {
		if it.GetName() == name {
			return it
		}
	}
	return nil
}

func countContainersWithRuntime[T any](items []*provision.PlacementIntent_Item, extract func(*provision.Container) T, check func(T) bool) int {
	n := 0
	for _, it := range items {
		for _, c := range it.GetContainers() {
			if check(extract(c)) {
				n++
			}
		}
	}
	return n
}

// ---- Tests ----

func TestBuildForPostgresInstance_SingleNode(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := NewPostgresPlacementBuilder(network)

	intent, err := b.BuildForPostgresInstance(&database.Database_Template_PostgresInstance{
		PostgresInstance: &database.Postgres_Instance{
			Defaults: pgSettings(),
			Node: &database.Postgres_Node{
				Name:     "postgres-master",
				Hardware: hw(4, 8, 100),
				Postgres: &database.Postgres_PostgresService{
					Role: database.Postgres_PostgresService_ROLE_MASTER,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(intent.GetItems()) != 1 {
		t.Fatalf("expected 1 item, got %d", len(intent.GetItems()))
	}

	item := intent.GetItems()[0]
	if item.GetName() != "postgres-master" {
		t.Errorf("expected name postgres-master, got %s", item.GetName())
	}
	if item.GetInternalIp().GetValue() != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %s", item.GetInternalIp().GetValue())
	}
	if item.GetHardware().GetCores() != 4 {
		t.Errorf("expected 4 cores, got %d", item.GetHardware().GetCores())
	}

	// Should have exactly 1 postgres container
	pgCount := 0
	for _, c := range item.GetContainers() {
		if c.GetPostgres() != nil {
			pgCount++
			if c.GetPostgres().GetRole() != provision.Container_PostgresRuntime_ROLE_MASTER {
				t.Errorf("expected ROLE_MASTER, got %s", c.GetPostgres().GetRole())
			}
		}
	}
	if pgCount != 1 {
		t.Errorf("expected 1 postgres container, got %d", pgCount)
	}
}

func TestBuildForPostgresInstance_NilTemplate(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := NewPostgresPlacementBuilder(network)

	_, err := b.BuildForPostgresInstance(nil)
	if err == nil {
		t.Fatal("expected error for nil template")
	}
}

func TestBuildForPostgresInstance_WithSidecars(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := NewPostgresPlacementBuilder(network)

	intent, err := b.BuildForPostgresInstance(&database.Database_Template_PostgresInstance{
		PostgresInstance: &database.Postgres_Instance{
			Defaults: pgSettings(),
			Node: &database.Postgres_Node{
				Name:     "postgres-master",
				Hardware: hw(4, 8, 100),
				Postgres: &database.Postgres_PostgresService{
					Role: database.Postgres_PostgresService_ROLE_MASTER,
				},
				Monitoring: &database.Postgres_MonitoringService{
					NodeExporter:     &database.Postgres_NodeExporterConfig{Port: uint32Ptr(9100)},
					PostgresExporter: &database.Postgres_PostgresExporterConfig{},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	item := intent.GetItems()[0]
	// 1 postgres + 1 node-exporter sidecar + 1 postgres-exporter sidecar = 3
	if len(item.GetContainers()) != 3 {
		t.Errorf("expected 3 containers, got %d", len(item.GetContainers()))
		for _, c := range item.GetContainers() {
			t.Logf("  container: id=%s runtime=%T", c.GetId(), c.GetRuntime())
		}
	}
}

func TestBuildForPostgresCluster_MasterAndReplicas(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2", "10.0.0.3")
	b := NewPostgresPlacementBuilder(network)

	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: pgSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(8, 16, 200),
					Postgres: &database.Postgres_PostgresService{
						Role: database.Postgres_PostgresService_ROLE_MASTER,
					},
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{
						Role: database.Postgres_PostgresService_ROLE_REPLICA,
					},
				},
				{
					Name:     "postgres-replica-1",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{
						Role: database.Postgres_PostgresService_ROLE_REPLICA,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(intent.GetItems()) != 3 {
		t.Fatalf("expected 3 items (1 master + 2 replicas), got %d", len(intent.GetItems()))
	}

	master := findItem(intent.GetItems(), "postgres-master")
	if master == nil {
		t.Fatal("master item not found")
	}
	if master.GetHardware().GetCores() != 8 {
		t.Errorf("master expected 8 cores, got %d", master.GetHardware().GetCores())
	}

	replica0 := findItem(intent.GetItems(), "postgres-replica-0")
	if replica0 == nil {
		t.Fatal("replica-0 not found")
	}
	if replica0.GetHardware().GetCores() != 4 {
		t.Errorf("replica-0 expected 4 cores, got %d", replica0.GetHardware().GetCores())
	}

	replica1 := findItem(intent.GetItems(), "postgres-replica-1")
	if replica1 == nil {
		t.Fatal("replica-1 not found")
	}
}

func TestBuildForPostgresCluster_ReplicaOverride(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2", "10.0.0.3")
	b := NewPostgresPlacementBuilder(network)

	overrideHw := hw(16, 32, 500)
	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: pgSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(8, 16, 200),
					Postgres: &database.Postgres_PostgresService{
						Role: database.Postgres_PostgresService_ROLE_MASTER,
					},
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{
						Role: database.Postgres_PostgresService_ROLE_REPLICA,
					},
				},
				{
					Name:     "postgres-replica-1",
					Hardware: overrideHw,
					Postgres: &database.Postgres_PostgresService{
						Role: database.Postgres_PostgresService_ROLE_REPLICA,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	replica0 := findItem(intent.GetItems(), "postgres-replica-0")
	if replica0.GetHardware().GetCores() != 4 {
		t.Errorf("replica-0 should use default hw, got cores=%d", replica0.GetHardware().GetCores())
	}

	replica1 := findItem(intent.GetItems(), "postgres-replica-1")
	if replica1.GetHardware().GetCores() != 16 {
		t.Errorf("replica-1 should use override hw, got cores=%d", replica1.GetHardware().GetCores())
	}
}

func TestBuildForPostgresCluster_WithMonitoring(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2")
	b := NewPostgresPlacementBuilder(network)

	monitoring := &database.Postgres_MonitoringService{
		NodeExporter:     &database.Postgres_NodeExporterConfig{},
		PostgresExporter: &database.Postgres_PostgresExporterConfig{},
	}

	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: pgSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:       "postgres-master",
					Hardware:   hw(4, 8, 100),
					Postgres:   &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
					Monitoring: monitoring,
				},
				{
					Name:       "postgres-replica-0",
					Hardware:   hw(4, 8, 100),
					Postgres:   &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
					Monitoring: monitoring,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	master := findItem(intent.GetItems(), "postgres-master")
	// master: 1 postgres + 1 node-exporter + 1 postgres-exporter = 3
	if len(master.GetContainers()) != 3 {
		t.Errorf("master expected 3 containers (pg + ne + pge), got %d", len(master.GetContainers()))
	}

	replica := findItem(intent.GetItems(), "postgres-replica-0")
	if len(replica.GetContainers()) != 3 {
		t.Errorf("replica expected 3 containers, got %d", len(replica.GetContainers()))
	}
}

func TestBuildForPostgresCluster_ColocatedEtcd(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2", "10.0.0.3")
	b := NewPostgresPlacementBuilder(network)

	etcdSvc := &database.Postgres_EtcdService{}

	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: pgSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
					Etcd:     etcdSvc,
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
					Etcd:     etcdSvc,
				},
				{
					Name:     "postgres-replica-1",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
					Etcd:     etcdSvc,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Still 3 items (colocated, no extra nodes)
	if len(intent.GetItems()) != 3 {
		t.Fatalf("expected 3 items, got %d", len(intent.GetItems()))
	}

	// Each node should have etcd container
	etcdCount := 0
	for _, item := range intent.GetItems() {
		for _, c := range item.GetContainers() {
			if c.GetEtcd() != nil {
				etcdCount++
			}
		}
	}
	if etcdCount != 3 {
		t.Errorf("expected 3 etcd containers, got %d", etcdCount)
	}
}

func TestBuildForPostgresCluster_DedicatedEtcd(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5", "10.0.0.6")
	b := NewPostgresPlacementBuilder(network)

	etcdSvc := &database.Postgres_EtcdService{}

	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: pgSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
				},
				{
					Name:     "postgres-replica-1",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
				},
				{
					Name:     "etcd-0",
					Hardware: hw(2, 4, 50),
					Etcd:     etcdSvc,
				},
				{
					Name:     "etcd-1",
					Hardware: hw(2, 4, 50),
					Etcd:     etcdSvc,
				},
				{
					Name:     "etcd-2",
					Hardware: hw(2, 4, 50),
					Etcd:     etcdSvc,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3 postgres nodes + 3 dedicated etcd nodes = 6
	if len(intent.GetItems()) != 6 {
		t.Fatalf("expected 6 items, got %d", len(intent.GetItems()))
	}

	// Dedicated etcd items should have only etcd containers (no postgres)
	for _, item := range intent.GetItems() {
		hasPG := false
		hasEtcd := false
		for _, c := range item.GetContainers() {
			if c.GetPostgres() != nil {
				hasPG = true
			}
			if c.GetEtcd() != nil {
				hasEtcd = true
			}
		}
		if hasPG && hasEtcd {
			t.Errorf("item %s has both postgres and etcd -- dedicated etcd should be separate", item.GetName())
		}
	}
}

func TestBuildForPostgresCluster_Pgbouncer(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2")
	b := NewPostgresPlacementBuilder(network)

	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: pgSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
					Pgbouncer: &database.Postgres_PgbouncerService{
						Config: &database.Postgres_PgbouncerConfig{
							PoolSize: 20,
							PoolMode: database.Postgres_PgbouncerConfig_TRANSACTION,
						},
					},
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	master := findItem(intent.GetItems(), "postgres-master")
	hasPgbouncer := false
	for _, c := range master.GetContainers() {
		if c.GetPgbouncer() != nil {
			hasPgbouncer = true
			if c.GetPgbouncer().GetConfig().GetPoolMode() != database.Postgres_PgbouncerConfig_TRANSACTION {
				t.Errorf("expected TRANSACTION pool mode")
			}
		}
	}
	if !hasPgbouncer {
		t.Error("expected pgbouncer container on master")
	}
}

func TestBuildForPostgresCluster_BackupOnMaster(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2")
	b := NewPostgresPlacementBuilder(network)

	backupSvc := &database.Postgres_BackupService{
		Config: &database.Postgres_BackupConfig{
			Schedule:  "0 3 * * *",
			Retention: "7d",
			Tool:      database.Postgres_BackupConfig_WAL_G,
			Storage: &database.Postgres_BackupConfig_Local{
				Local: &database.Postgres_BackupConfig_LocalStorage{Path: "/backups"},
			},
		},
	}

	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: pgSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
					Backup:   backupSvc,
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	master := findItem(intent.GetItems(), "postgres-master")
	hasBackup := false
	for _, c := range master.GetContainers() {
		if c.GetBackup() != nil {
			hasBackup = true
		}
	}
	if !hasBackup {
		t.Error("expected backup container on master")
	}

	replica := findItem(intent.GetItems(), "postgres-replica-0")
	for _, c := range replica.GetContainers() {
		if c.GetBackup() != nil {
			t.Error("backup should not be on replica (scope=MASTER)")
		}
	}
}

func TestBuildForPostgresCluster_NilNodes(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := NewPostgresPlacementBuilder(network)

	_, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{},
	})
	if err == nil {
		t.Fatal("expected error for empty cluster nodes")
	}
}

func TestBuild_NotEnoughIPs(t *testing.T) {
	network := networkWithIPs("10.0.0.1") // only 1 IP
	b := NewPostgresPlacementBuilder(network)

	_, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: pgSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
				},
				{
					Name:     "postgres-replica-1",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error when not enough IPs")
	}
}

func TestResolveRuntimeConfig_EtcdEnv(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2", "10.0.0.3")
	b := NewPostgresPlacementBuilder(network)

	etcdSvc := &database.Postgres_EtcdService{}

	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: pgSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
					Etcd:     etcdSvc,
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
					Etcd:     etcdSvc,
				},
				{
					Name:     "postgres-replica-1",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
					Etcd:     etcdSvc,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check etcd env vars are set on all etcd containers
	for _, item := range intent.GetItems() {
		for _, c := range item.GetContainers() {
			if c.GetEtcd() == nil {
				continue
			}
			if c.Env == nil {
				t.Errorf("item %s etcd container %s has no env", item.GetName(), c.GetId())
				continue
			}
			if c.Env["ETCD_NAME"] == "" {
				t.Errorf("ETCD_NAME not set on %s/%s", item.GetName(), c.GetId())
			}
			if c.Env["ETCD_INITIAL_CLUSTER"] == "" {
				t.Errorf("ETCD_INITIAL_CLUSTER not set on %s/%s", item.GetName(), c.GetId())
			}
			if c.Env["ETCD_LISTEN_CLIENT_URLS"] == "" {
				t.Errorf("ETCD_LISTEN_CLIENT_URLS not set on %s/%s", item.GetName(), c.GetId())
			}
		}
	}
}

func TestResolveRuntimeConfig_PatroniEnv(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2", "10.0.0.3")
	b := NewPostgresPlacementBuilder(network)

	etcdSvc := &database.Postgres_EtcdService{}

	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: patroniSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
					Etcd:     etcdSvc,
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
					Etcd:     etcdSvc,
				},
				{
					Name:     "postgres-replica-1",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
					Etcd:     etcdSvc,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All postgres containers should have patroni env
	for _, item := range intent.GetItems() {
		for _, c := range item.GetContainers() {
			pg := c.GetPostgres()
			if pg == nil {
				continue
			}
			if c.Env == nil || c.Env["PATRONI_NAME"] == "" {
				t.Errorf("PATRONI_NAME not set on %s/%s", item.GetName(), c.GetId())
			}
			if c.Env["PATRONI_ETCD3_HOSTS"] == "" {
				t.Errorf("PATRONI_ETCD3_HOSTS not set on %s/%s", item.GetName(), c.GetId())
			}
			if c.Env["PATRONI_POSTGRESQL_CONNECT_ADDRESS"] == "" {
				t.Errorf("PATRONI_POSTGRESQL_CONNECT_ADDRESS not set on %s/%s", item.GetName(), c.GetId())
			}
		}
	}
}

func TestBuildForPostgresInstance_PostgresqlConfToArgs(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := NewPostgresPlacementBuilder(network)

	settings := pgSettings()
	settings.PostgresqlConf = map[string]string{
		"shared_buffers":  "256MB",
		"max_connections": "1000",
	}

	intent, err := b.BuildForPostgresInstance(&database.Database_Template_PostgresInstance{
		PostgresInstance: &database.Postgres_Instance{
			Defaults: settings,
			Node: &database.Postgres_Node{
				Name:     "postgres-master",
				Hardware: hw(4, 8, 100),
				Postgres: &database.Postgres_PostgresService{
					Role: database.Postgres_PostgresService_ROLE_MASTER,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pg := findContainer(intent.GetItems(), "postgres-master", "postgres-master-container")
	if pg == nil {
		t.Fatal("postgres container not found")
	}

	expectedArgs := []string{
		"-c", "max_connections=1000",
		"-c", "shared_buffers=256MB",
	}
	if !reflect.DeepEqual(pg.GetArgs(), expectedArgs) {
		t.Fatalf("unexpected postgres args, got=%v expected=%v", pg.GetArgs(), expectedArgs)
	}
}

func TestResolveRuntimeConfig_PatroniPostgresqlConfToEnv(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2", "10.0.0.3")
	b := NewPostgresPlacementBuilder(network)

	settings := patroniSettings()
	settings.PostgresqlConf = map[string]string{
		"max_connections": "1000",
		"wal_level":       "logical",
	}

	etcdSvc := &database.Postgres_EtcdService{}

	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: settings,
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
					Etcd:     etcdSvc,
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
					Etcd:     etcdSvc,
				},
				{
					Name:     "postgres-replica-1",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
					Etcd:     etcdSvc,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, item := range intent.GetItems() {
		for _, c := range item.GetContainers() {
			pg := c.GetPostgres()
			if pg == nil {
				continue
			}
			raw := c.GetEnv()["PATRONI_POSTGRESQL_PARAMETERS"]
			if raw == "" {
				t.Fatalf("PATRONI_POSTGRESQL_PARAMETERS is not set on %s/%s", item.GetName(), c.GetId())
			}

			got := map[string]string{}
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatalf("invalid PATRONI_POSTGRESQL_PARAMETERS json on %s/%s: %v", item.GetName(), c.GetId(), err)
			}
			if !reflect.DeepEqual(got, settings.GetPostgresqlConf()) {
				t.Fatalf("unexpected PATRONI_POSTGRESQL_PARAMETERS on %s/%s: got=%v expected=%v", item.GetName(), c.GetId(), got, settings.GetPostgresqlConf())
			}
		}
	}
}

func TestResolveRuntimeConfig_PatroniWithoutEtcd(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2")
	b := NewPostgresPlacementBuilder(network)

	_, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: patroniSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error: patroni enabled without etcd")
	}
}

func TestResolveRuntimeConfig_PgbouncerEnv(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2")
	b := NewPostgresPlacementBuilder(network)

	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: pgSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
					Pgbouncer: &database.Postgres_PgbouncerService{
						Config: &database.Postgres_PgbouncerConfig{},
					},
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	master := findItem(intent.GetItems(), "postgres-master")
	for _, c := range master.GetContainers() {
		if c.GetPgbouncer() == nil {
			continue
		}
		if c.Env == nil {
			t.Fatal("pgbouncer env is nil")
		}
		if c.Env["PGBOUNCER_UPSTREAM_HOST"] != "10.0.0.1" {
			t.Errorf("expected PGBOUNCER_UPSTREAM_HOST=10.0.0.1, got %s", c.Env["PGBOUNCER_UPSTREAM_HOST"])
		}
		if c.Env["PGBOUNCER_UPSTREAM_PORT"] != "5432" {
			t.Errorf("expected PGBOUNCER_UPSTREAM_PORT=5432, got %s", c.Env["PGBOUNCER_UPSTREAM_PORT"])
		}
	}
}

func TestItemOrder_MatchesCreation(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2", "10.0.0.3")
	b := NewPostgresPlacementBuilder(network)

	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: pgSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
				},
				{
					Name:     "postgres-replica-1",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Order should be: master, replica-0, replica-1
	expected := []string{"postgres-master", "postgres-replica-0", "postgres-replica-1"}
	for i, item := range intent.GetItems() {
		if item.GetName() != expected[i] {
			t.Errorf("item[%d] expected %s, got %s", i, expected[i], item.GetName())
		}
	}
}

func TestIPAssignment_MatchesOrder(t *testing.T) {
	network := networkWithIPs("10.0.0.10", "10.0.0.20", "10.0.0.30")
	b := NewPostgresPlacementBuilder(network)

	intent, err := b.BuildForPostgresCluster(&database.Database_Template_PostgresCluster{
		PostgresCluster: &database.Postgres_Cluster{
			Defaults: pgSettings(),
			Nodes: []*database.Postgres_Node{
				{
					Name:     "postgres-master",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_MASTER},
				},
				{
					Name:     "postgres-replica-0",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
				},
				{
					Name:     "postgres-replica-1",
					Hardware: hw(4, 8, 100),
					Postgres: &database.Postgres_PostgresService{Role: database.Postgres_PostgresService_ROLE_REPLICA},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedIPs := []string{"10.0.0.10", "10.0.0.20", "10.0.0.30"}
	for i, item := range intent.GetItems() {
		if item.GetInternalIp().GetValue() != expectedIPs[i] {
			t.Errorf("item[%d] expected IP %s, got %s", i, expectedIPs[i], item.GetInternalIp().GetValue())
		}
	}
}

func uint32Ptr(v uint32) *uint32 { return &v }

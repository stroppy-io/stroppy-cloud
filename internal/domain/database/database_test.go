package database

import (
	"context"
	"testing"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
)

func pgSettings(version database.Postgres_Settings_Version, storageEngine database.Postgres_Settings_StorageEngine) *database.Postgres_Settings {
	return &database.Postgres_Settings{
		Version:       version,
		StorageEngine: storageEngine,
	}
}

func hw(cores, mem, disk uint32) *deployment.Hardware {
	return &deployment.Hardware{Cores: cores, Memory: mem, Disk: disk}
}

func pgNode(name string, role database.Postgres_PostgresService_Role, hardware *deployment.Hardware) *database.Postgres_Node {
	return &database.Postgres_Node{
		Name:     name,
		Hardware: hardware,
		Postgres: &database.Postgres_PostgresService{Role: role},
	}
}

func TestValidateDatabaseTemplate_Nil(t *testing.T) {
	ctx := context.Background()
	err := ValidateDatabaseTemplate(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil template")
	}
	if err.Error() != "database template is nil" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateDatabaseTemplate_NilContent(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for nil template content")
	}
}

func TestValidatePostgresInstance_Valid(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresInstance{
			PostgresInstance: &database.Postgres_Instance{
				Defaults: pgSettings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
				Node:     pgNode("pg-0", database.Postgres_PostgresService_ROLE_MASTER, hw(4, 8, 100)),
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePostgresInstance_NilInstance(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresInstance{},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for nil postgres instance")
	}
}

func TestValidatePostgresInstance_PatroniEnabled(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresInstance{
			PostgresInstance: &database.Postgres_Instance{
				Defaults: &database.Postgres_Settings{
					Version:       database.Postgres_Settings_VERSION_17,
					StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_HEAP,
					Patroni:       &database.Postgres_Settings_Patroni{Enabled: true},
				},
				Node: pgNode("pg-0", database.Postgres_PostgresService_ROLE_MASTER, hw(4, 8, 100)),
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for patroni on instance")
	}
}

func TestValidatePostgresCluster_Valid(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster{
				Defaults: pgSettings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
				Nodes: []*database.Postgres_Node{
					pgNode("master", database.Postgres_PostgresService_ROLE_MASTER, hw(4, 8, 100)),
					pgNode("replica-1", database.Postgres_PostgresService_ROLE_REPLICA, hw(2, 4, 50)),
					pgNode("replica-2", database.Postgres_PostgresService_ROLE_REPLICA, hw(2, 4, 50)),
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePostgresCluster_NilCluster(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for nil cluster")
	}
}

func TestValidatePostgresCluster_TooFewNodes(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster{
				Defaults: pgSettings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
				Nodes: []*database.Postgres_Node{
					pgNode("master", database.Postgres_PostgresService_ROLE_MASTER, hw(4, 8, 100)),
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for cluster with less than 2 nodes")
	}
}

func TestValidatePostgresCluster_NoMaster(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster{
				Defaults: pgSettings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
				Nodes: []*database.Postgres_Node{
					pgNode("replica-1", database.Postgres_PostgresService_ROLE_REPLICA, hw(2, 4, 50)),
					pgNode("replica-2", database.Postgres_PostgresService_ROLE_REPLICA, hw(2, 4, 50)),
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for cluster with no master")
	}
}

func TestValidatePostgresCluster_DuplicateNodeName(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster{
				Defaults: pgSettings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
				Nodes: []*database.Postgres_Node{
					pgNode("pg-0", database.Postgres_PostgresService_ROLE_MASTER, hw(4, 8, 100)),
					pgNode("pg-0", database.Postgres_PostgresService_ROLE_REPLICA, hw(2, 4, 50)),
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for duplicate node name")
	}
}

func TestValidatePostgresCluster_EtcdQuorum(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster{
				Defaults: pgSettings(database.Postgres_Settings_VERSION_17, database.Postgres_Settings_STORAGE_ENGINE_HEAP),
				Nodes: []*database.Postgres_Node{
					pgNode("master", database.Postgres_PostgresService_ROLE_MASTER, hw(4, 8, 100)),
					pgNode("replica-1", database.Postgres_PostgresService_ROLE_REPLICA, hw(2, 4, 50)),
					{Name: "etcd-1", Hardware: hw(2, 4, 10), Etcd: &database.Postgres_EtcdService{}},
					{Name: "etcd-2", Hardware: hw(2, 4, 10), Etcd: &database.Postgres_EtcdService{}},
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for invalid etcd quorum size (2)")
	}
}

func TestValidatePostgresCluster_PatroniWithEtcd(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster{
				Defaults: &database.Postgres_Settings{
					Version:       database.Postgres_Settings_VERSION_17,
					StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_HEAP,
					Patroni:       &database.Postgres_Settings_Patroni{Enabled: true},
				},
				Nodes: []*database.Postgres_Node{
					pgNode("master", database.Postgres_PostgresService_ROLE_MASTER, hw(4, 8, 100)),
					pgNode("replica-1", database.Postgres_PostgresService_ROLE_REPLICA, hw(2, 4, 50)),
					{Name: "etcd-1", Hardware: hw(2, 4, 10), Etcd: &database.Postgres_EtcdService{}},
					{Name: "etcd-2", Hardware: hw(2, 4, 10), Etcd: &database.Postgres_EtcdService{}},
					{Name: "etcd-3", Hardware: hw(2, 4, 10), Etcd: &database.Postgres_EtcdService{}},
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePostgresCluster_PatroniWithoutEtcd(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresCluster{
			PostgresCluster: &database.Postgres_Cluster{
				Defaults: &database.Postgres_Settings{
					Version:       database.Postgres_Settings_VERSION_17,
					StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_HEAP,
					Patroni:       &database.Postgres_Settings_Patroni{Enabled: true},
				},
				Nodes: []*database.Postgres_Node{
					pgNode("master", database.Postgres_PostgresService_ROLE_MASTER, hw(4, 8, 100)),
					pgNode("replica-1", database.Postgres_PostgresService_ROLE_REPLICA, hw(2, 4, 50)),
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for patroni without etcd")
	}
}

func TestValidatePostgresCluster_PatroniSynchronousMode(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name        string
		syncNodes   uint32
		replicas    int
		expectError bool
	}{
		{"sync_nodes_equals_replicas", 2, 2, false},
		{"sync_nodes_less_than_replicas", 1, 2, false},
		{"sync_nodes_more_than_replicas", 3, 2, true},
		{"sync_nodes_zero_uses_default", 0, 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := []*database.Postgres_Node{
				pgNode("master", database.Postgres_PostgresService_ROLE_MASTER, hw(4, 8, 100)),
			}
			for i := 0; i < tt.replicas; i++ {
				nodes = append(nodes, pgNode("replica-"+string(rune('1'+i)), database.Postgres_PostgresService_ROLE_REPLICA, hw(2, 4, 50)))
			}
			nodes = append(nodes,
				&database.Postgres_Node{Name: "etcd-1", Hardware: hw(2, 4, 10), Etcd: &database.Postgres_EtcdService{}},
				&database.Postgres_Node{Name: "etcd-2", Hardware: hw(2, 4, 10), Etcd: &database.Postgres_EtcdService{}},
				&database.Postgres_Node{Name: "etcd-3", Hardware: hw(2, 4, 10), Etcd: &database.Postgres_EtcdService{}},
			)
			tmpl := &database.Database_Template{
				Template: &database.Database_Template_PostgresCluster{
					PostgresCluster: &database.Postgres_Cluster{
						Defaults: &database.Postgres_Settings{
							Version:       database.Postgres_Settings_VERSION_17,
							StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_HEAP,
							Patroni: &database.Postgres_Settings_Patroni{
								Enabled:              true,
								SynchronousMode:      true,
								SynchronousNodeCount: tt.syncNodes,
							},
						},
						Nodes: nodes,
					},
				},
			}
			err := ValidateDatabaseTemplate(ctx, tmpl)
			if tt.expectError && err == nil {
				t.Fatal("expected error for patroni synchronous mode")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateSettings_OrioledbValidVersion(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		version database.Postgres_Settings_Version
	}{
		{"orioledb_v16", database.Postgres_Settings_VERSION_16},
		{"orioledb_v17", database.Postgres_Settings_VERSION_17},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &database.Database_Template{
				Template: &database.Database_Template_PostgresInstance{
					PostgresInstance: &database.Postgres_Instance{
						Defaults: &database.Postgres_Settings{
							Version:       tt.version,
							StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_ORIOLEDB,
						},
						Node: pgNode("pg-0", database.Postgres_PostgresService_ROLE_MASTER, hw(4, 8, 100)),
					},
				},
			}
			err := ValidateDatabaseTemplate(ctx, tmpl)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateSettings_OrioledbInvalidVersion(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PostgresInstance{
			PostgresInstance: &database.Postgres_Instance{
				Defaults: &database.Postgres_Settings{
					Version:       database.Postgres_Settings_VERSION_18,
					StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_ORIOLEDB,
				},
				Node: pgNode("pg-0", database.Postgres_PostgresService_ROLE_MASTER, hw(4, 8, 100)),
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for orioledb with invalid version")
	}
}

func TestValidatePicodataInstance_Valid(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PicodataInstance{
			PicodataInstance: &database.Picodata_Instance{
				Template: &database.Picodata_Instance_Template{
					Settings: &database.Picodata_Settings{Version: "24.11"},
					Hardware: hw(4, 8, 100),
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePicodataCluster_Valid(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PicodataCluster{
			PicodataCluster: &database.Picodata_Cluster{
				Template: &database.Picodata_Cluster_Template{
					Topology: &database.Picodata_Cluster_Template_Topology{
						Settings:     &database.Picodata_Settings{Version: "24.11"},
						NodeHardware: hw(4, 8, 100),
						NodesCount:   2,
					},
				},
				Nodes: []*database.Picodata_Instance{
					{Template: &database.Picodata_Instance_Template{Settings: &database.Picodata_Settings{Version: "24.11"}, Hardware: hw(4, 8, 100)}},
					{Template: &database.Picodata_Instance_Template{Settings: &database.Picodata_Settings{Version: "24.11"}, Hardware: hw(4, 8, 100)}},
				},
			},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePicodataCluster_Empty(t *testing.T) {
	ctx := context.Background()
	tmpl := &database.Database_Template{
		Template: &database.Database_Template_PicodataCluster{
			PicodataCluster: &database.Picodata_Cluster{},
		},
	}
	err := ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		t.Fatal("expected error for empty picodata cluster")
	}
}

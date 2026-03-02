package provision

import (
	"testing"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
)

func TestRequiredIPCount(t *testing.T) {
	hw := func(cores, mem, disk uint32) *deployment.Hardware {
		return &deployment.Hardware{Cores: cores, Memory: mem, Disk: disk}
	}
	pgSettings := &database.Postgres_Settings{
		Version:       database.Postgres_Settings_VERSION_17,
		StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_HEAP,
	}
	pgNode := func(name string, role database.Postgres_PostgresService_Role) *database.Postgres_Node {
		return &database.Postgres_Node{
			Name:     name,
			Hardware: hw(2, 4, 50),
			Postgres: &database.Postgres_PostgresService{Role: role},
		}
	}

	tests := []struct {
		name string
		tmpl *database.Database_Template
		want int
	}{
		{
			name: "nil template",
			tmpl: nil,
			want: 0,
		},
		{
			name: "postgres instance",
			tmpl: &database.Database_Template{
				Template: &database.Database_Template_PostgresInstance{
					PostgresInstance: &database.Postgres_Instance{
						Defaults: pgSettings,
						Node: &database.Postgres_Node{
							Name:     "pg-standalone",
							Hardware: hw(4, 8, 100),
							Postgres: &database.Postgres_PostgresService{
								Role: database.Postgres_PostgresService_ROLE_MASTER,
							},
						},
					},
				},
			},
			want: 1,
		},
		{
			name: "postgres cluster with 3 nodes",
			tmpl: &database.Database_Template{
				Template: &database.Database_Template_PostgresCluster{
					PostgresCluster: &database.Postgres_Cluster{
						Defaults: pgSettings,
						Nodes: []*database.Postgres_Node{
							pgNode("master", database.Postgres_PostgresService_ROLE_MASTER),
							pgNode("replica-1", database.Postgres_PostgresService_ROLE_REPLICA),
							pgNode("replica-2", database.Postgres_PostgresService_ROLE_REPLICA),
						},
					},
				},
			},
			want: 3,
		},
		{
			name: "postgres cluster with 5 nodes (master + replicas + etcd)",
			tmpl: &database.Database_Template{
				Template: &database.Database_Template_PostgresCluster{
					PostgresCluster: &database.Postgres_Cluster{
						Defaults: pgSettings,
						Nodes: []*database.Postgres_Node{
							pgNode("master", database.Postgres_PostgresService_ROLE_MASTER),
							pgNode("replica-1", database.Postgres_PostgresService_ROLE_REPLICA),
							pgNode("replica-2", database.Postgres_PostgresService_ROLE_REPLICA),
							{Name: "etcd-1", Hardware: hw(2, 4, 10), Etcd: &database.Postgres_EtcdService{}},
							{Name: "etcd-2", Hardware: hw(2, 4, 10), Etcd: &database.Postgres_EtcdService{}},
						},
					},
				},
			},
			want: 5,
		},
		{
			name: "picodata instance",
			tmpl: &database.Database_Template{
				Template: &database.Database_Template_PicodataInstance{
					PicodataInstance: &database.Picodata_Instance{
						Template: &database.Picodata_Instance_Template{
							Settings: &database.Picodata_Settings{Version: "24.11"},
							Hardware: hw(4, 8, 100),
						},
					},
				},
			},
			want: 1,
		},
		{
			name: "picodata cluster with 3 nodes",
			tmpl: &database.Database_Template{
				Template: &database.Database_Template_PicodataCluster{
					PicodataCluster: &database.Picodata_Cluster{
						Nodes: []*database.Picodata_Instance{
							{Template: &database.Picodata_Instance_Template{Settings: &database.Picodata_Settings{Version: "24.11"}, Hardware: hw(4, 8, 100)}},
							{Template: &database.Picodata_Instance_Template{Settings: &database.Picodata_Settings{Version: "24.11"}, Hardware: hw(4, 8, 100)}},
							{Template: &database.Picodata_Instance_Template{Settings: &database.Picodata_Settings{Version: "24.11"}, Hardware: hw(4, 8, 100)}},
						},
					},
				},
			},
			want: 3,
		},
		{
			name: "picodata cluster nil nodes",
			tmpl: &database.Database_Template{
				Template: &database.Database_Template_PicodataCluster{
					PicodataCluster: &database.Picodata_Cluster{},
				},
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RequiredIPCount(tt.tmpl); got != tt.want {
				t.Errorf("RequiredIPCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

package store

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	stroppyProto "github.com/stroppy-io/stroppy/pkg/common/proto/stroppy"
)

// SeedBuiltins creates built-in workloads and topology templates if they don't exist.
// Idempotent — skips rows that already exist (matched by name).
func SeedBuiltins(ctx context.Context, pool *pgxpool.Pool) error {
	if err := seedWorkloads(ctx, pool); err != nil {
		return fmt.Errorf("seed workloads: %w", err)
	}
	if err := seedTopologyTemplates(ctx, pool); err != nil {
		return fmt.Errorf("seed topology templates: %w", err)
	}
	return nil
}

func seedWorkloads(ctx context.Context, pool *pgxpool.Pool) error {
	workloads := []struct {
		name        string
		description string
		probe       *pb.ProbeResult
	}{
		{
			name:        "TPC-C",
			description: "TPC-C benchmark — OLTP workload simulating a wholesale supplier (warehouses, orders, payments)",
			probe: &pb.ProbeResult{
				DriverConfig: &stroppyProto.DriverConfig{
					DriverType: stroppyProto.DriverConfig_DRIVER_TYPE_POSTGRES,
				},
				Steps: []string{"init", "run", "cleanup"},
			},
		},
		{
			name:        "TPC-B",
			description: "TPC-B benchmark — simple OLTP workload (account balance updates)",
			probe: &pb.ProbeResult{
				DriverConfig: &stroppyProto.DriverConfig{
					DriverType: stroppyProto.DriverConfig_DRIVER_TYPE_POSTGRES,
				},
				Steps: []string{"init", "run", "cleanup"},
			},
		},
	}

	for _, w := range workloads {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM workloads WHERE name = $1)`, w.name,
		).Scan(&exists); err != nil {
			return fmt.Errorf("check workload %q: %w", w.name, err)
		}
		if exists {
			continue
		}

		probeJSON, err := protojson.Marshal(w.probe)
		if err != nil {
			return fmt.Errorf("marshal probe for %q: %w", w.name, err)
		}

		if _, err := pool.Exec(ctx,
			`INSERT INTO workloads (id, name, description, builtin, script, probe)
			 VALUES ($1, $2, $3, true, $4, $5)`,
			ulid.Make().String(), w.name, w.description, []byte("// builtin"), probeJSON,
		); err != nil {
			return fmt.Errorf("insert workload %q: %w", w.name, err)
		}
		log.Printf("Seeded builtin workload: %s", w.name)
	}

	return nil
}

func seedTopologyTemplates(ctx context.Context, pool *pgxpool.Pool) error {
	templates := []struct {
		name        string
		description string
		template    *database.Database_Template
	}{
		{
			name:        "PostgreSQL Standalone",
			description: "Single PostgreSQL 17 instance — for dev/test, no replication",
			template: &database.Database_Template{
				Template: &database.Database_Template_PostgresInstance{
					PostgresInstance: &database.Postgres_Instance{
						Defaults: pgDefaults(),
						Node: &database.Postgres_Node{
							Name:     "pg-standalone",
							Hardware: hw(2, 4, 50),
							Postgres: &database.Postgres_PostgresService{
								Role: database.Postgres_PostgresService_ROLE_MASTER,
							},
							Monitoring: &database.Postgres_MonitoringService{},
						},
					},
				},
			},
		},
		{
			name:        "PostgreSQL HA Cluster (3 nodes)",
			description: "1 master + 2 replicas with Patroni, etcd, PgBouncer and monitoring",
			template: &database.Database_Template{
				Template: &database.Database_Template_PostgresCluster{
					PostgresCluster: &database.Postgres_Cluster{
						Defaults: pgDefaultsWithPatroni(),
						Nodes: []*database.Postgres_Node{
							{
								Name:     "pg-master",
								Hardware: hw(4, 8, 100),
								Postgres: &database.Postgres_PostgresService{
									Role: database.Postgres_PostgresService_ROLE_MASTER,
								},
								Etcd:       &database.Postgres_EtcdService{},
								Pgbouncer:  pgbouncer(),
								Monitoring: &database.Postgres_MonitoringService{},
							},
							{
								Name:     "pg-replica-1",
								Hardware: hw(4, 8, 100),
								Postgres: &database.Postgres_PostgresService{
									Role: database.Postgres_PostgresService_ROLE_REPLICA,
								},
								Etcd:       &database.Postgres_EtcdService{},
								Pgbouncer:  pgbouncer(),
								Monitoring: &database.Postgres_MonitoringService{},
							},
							{
								Name:     "pg-replica-2",
								Hardware: hw(4, 8, 100),
								Postgres: &database.Postgres_PostgresService{
									Role: database.Postgres_PostgresService_ROLE_REPLICA,
								},
								Etcd:       &database.Postgres_EtcdService{},
								Pgbouncer:  pgbouncer(),
								Monitoring: &database.Postgres_MonitoringService{},
							},
						},
					},
				},
			},
		},
		{
			name:        "PostgreSQL Large Cluster (7 nodes)",
			description: "1 master + 4 replicas + 2 dedicated etcd — production-grade HA with sync replication, Patroni, PgBouncer, monitoring",
			template: &database.Database_Template{
				Template: &database.Database_Template_PostgresCluster{
					PostgresCluster: &database.Postgres_Cluster{
						Defaults: pgDefaultsLargeCluster(),
						Nodes: []*database.Postgres_Node{
							{
								Name:     "pg-master",
								Hardware: hw(16, 64, 500),
								Postgres: &database.Postgres_PostgresService{
									Role: database.Postgres_PostgresService_ROLE_MASTER,
								},
								Pgbouncer:  pgbouncerLarge(),
								Monitoring: &database.Postgres_MonitoringService{},
							},
							pgReplicaNode("pg-replica-1", 16, 64, 500),
							pgReplicaNode("pg-replica-2", 16, 64, 500),
							pgReplicaNode("pg-replica-3", 8, 32, 250),
							pgReplicaNode("pg-replica-4", 8, 32, 250),
							{
								Name:     "etcd-1",
								Hardware: hw(2, 4, 20),
								Etcd:     &database.Postgres_EtcdService{Monitor: true},
							},
							{
								Name:     "etcd-2",
								Hardware: hw(2, 4, 20),
								Etcd:     &database.Postgres_EtcdService{Monitor: true},
							},
						},
					},
				},
			},
		},
	}

	for _, t := range templates {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM topology_templates WHERE name = $1)`, t.name,
		).Scan(&exists); err != nil {
			return fmt.Errorf("check template %q: %w", t.name, err)
		}
		if exists {
			continue
		}

		templateJSON, err := protojson.Marshal(t.template)
		if err != nil {
			return fmt.Errorf("marshal template %q: %w", t.name, err)
		}

		dbType := int32(pb.DatabaseType_DATABASE_TYPE_POSTGRES)

		if _, err := pool.Exec(ctx,
			`INSERT INTO topology_templates (id, name, description, database_type, builtin, template_data, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, true, $5, now(), now())`,
			ulid.Make().String(), t.name, t.description, dbType, templateJSON,
		); err != nil {
			return fmt.Errorf("insert template %q: %w", t.name, err)
		}
		log.Printf("Seeded builtin topology template: %s", t.name)
	}

	return nil
}

// --- helpers ---

func hw(cores, memory, disk uint32) *deployment.Hardware {
	return &deployment.Hardware{Cores: cores, Memory: memory, Disk: disk}
}

func pgDefaults() *database.Postgres_Settings {
	return &database.Postgres_Settings{
		Version:       database.Postgres_Settings_VERSION_17,
		StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_HEAP,
	}
}

func pgDefaultsWithPatroni() *database.Postgres_Settings {
	s := pgDefaults()
	s.Patroni = &database.Postgres_Settings_Patroni{
		Enabled:      true,
		Ttl:          30,
		LoopWait:     10,
		RetryTimeout: 10,
	}
	return s
}

func pgDefaultsLargeCluster() *database.Postgres_Settings {
	s := pgDefaults()
	s.Patroni = &database.Postgres_Settings_Patroni{
		Enabled:              true,
		Ttl:                  30,
		LoopWait:             10,
		RetryTimeout:         10,
		SynchronousMode:      true,
		SynchronousNodeCount: 1,
	}
	return s
}

func pgbouncer() *database.Postgres_PgbouncerService {
	return &database.Postgres_PgbouncerService{
		Config: &database.Postgres_PgbouncerConfig{
			PoolSize:      100,
			PoolMode:      database.Postgres_PgbouncerConfig_TRANSACTION,
			MaxClientConn: 1000,
		},
	}
}

func pgbouncerLarge() *database.Postgres_PgbouncerService {
	return &database.Postgres_PgbouncerService{
		Config: &database.Postgres_PgbouncerConfig{
			PoolSize:      200,
			PoolMode:      database.Postgres_PgbouncerConfig_TRANSACTION,
			MaxClientConn: 5000,
		},
	}
}

func pgReplicaNode(name string, cores, memory, disk uint32) *database.Postgres_Node {
	return &database.Postgres_Node{
		Name:     name,
		Hardware: hw(cores, memory, disk),
		Postgres: &database.Postgres_PostgresService{
			Role: database.Postgres_PostgresService_ROLE_REPLICA,
		},
		Pgbouncer:  pgbouncer(),
		Monitoring: &database.Postgres_MonitoringService{},
	}
}

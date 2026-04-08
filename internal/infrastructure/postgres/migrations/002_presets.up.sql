CREATE TABLE presets (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    db_kind     TEXT NOT NULL,
    topology    TEXT NOT NULL,
    is_builtin  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

CREATE INDEX idx_presets_tenant ON presets(tenant_id);
CREATE INDEX idx_presets_kind ON presets(tenant_id, db_kind);

-- Seed built-in presets for all existing tenants.

-- PostgreSQL single
INSERT INTO presets (id, tenant_id, name, description, db_kind, topology, is_builtin)
SELECT
    'builtin-pg-single-' || t.id, t.id,
    'PostgreSQL Single', 'Single PostgreSQL instance',
    'postgres',
    '{"master":{"role":"database","count":1,"cpus":2,"memory_mb":4096,"disk_gb":50}}',
    TRUE
FROM tenants t;

-- PostgreSQL HA
INSERT INTO presets (id, tenant_id, name, description, db_kind, topology, is_builtin)
SELECT
    'builtin-pg-ha-' || t.id, t.id,
    'PostgreSQL HA', 'PostgreSQL with Patroni, HAProxy, PgBouncer, synchronous replication',
    'postgres',
    '{"master":{"role":"database","count":1,"cpus":4,"memory_mb":8192,"disk_gb":100},"replicas":[{"role":"database","count":2,"cpus":4,"memory_mb":8192,"disk_gb":100}],"haproxy":{"role":"proxy","count":1,"cpus":2,"memory_mb":2048,"disk_gb":20},"pgbouncer":true,"patroni":true,"etcd":true,"sync_replicas":1}',
    TRUE
FROM tenants t;

-- PostgreSQL Scale
INSERT INTO presets (id, tenant_id, name, description, db_kind, topology, is_builtin)
SELECT
    'builtin-pg-scale-' || t.id, t.id,
    'PostgreSQL Scale', 'PostgreSQL with 4 replicas, 2 HAProxy nodes, full HA stack',
    'postgres',
    '{"master":{"role":"database","count":1,"cpus":8,"memory_mb":16384,"disk_gb":200},"replicas":[{"role":"database","count":4,"cpus":8,"memory_mb":16384,"disk_gb":200}],"haproxy":{"role":"proxy","count":2,"cpus":2,"memory_mb":2048,"disk_gb":20},"pgbouncer":true,"patroni":true,"etcd":true,"sync_replicas":2}',
    TRUE
FROM tenants t;

-- MySQL single
INSERT INTO presets (id, tenant_id, name, description, db_kind, topology, is_builtin)
SELECT
    'builtin-my-single-' || t.id, t.id,
    'MySQL Single', 'Single MySQL instance',
    'mysql',
    '{"primary":{"role":"database","count":1,"cpus":2,"memory_mb":4096,"disk_gb":50}}',
    TRUE
FROM tenants t;

-- MySQL Replica
INSERT INTO presets (id, tenant_id, name, description, db_kind, topology, is_builtin)
SELECT
    'builtin-my-replica-' || t.id, t.id,
    'MySQL Replica', 'MySQL with semi-synchronous replication and ProxySQL',
    'mysql',
    '{"primary":{"role":"database","count":1,"cpus":4,"memory_mb":8192,"disk_gb":100},"replicas":[{"role":"database","count":2,"cpus":4,"memory_mb":8192,"disk_gb":100}],"proxysql":{"role":"proxy","count":1,"cpus":2,"memory_mb":2048,"disk_gb":20},"semi_sync":true}',
    TRUE
FROM tenants t;

-- MySQL Group
INSERT INTO presets (id, tenant_id, name, description, db_kind, topology, is_builtin)
SELECT
    'builtin-my-group-' || t.id, t.id,
    'MySQL Group', 'MySQL with Group Replication and ProxySQL',
    'mysql',
    '{"primary":{"role":"database","count":1,"cpus":8,"memory_mb":16384,"disk_gb":200},"replicas":[{"role":"database","count":2,"cpus":8,"memory_mb":16384,"disk_gb":200}],"proxysql":{"role":"proxy","count":2,"cpus":2,"memory_mb":2048,"disk_gb":20},"group_replication":true}',
    TRUE
FROM tenants t;

-- Picodata single
INSERT INTO presets (id, tenant_id, name, description, db_kind, topology, is_builtin)
SELECT
    'builtin-pico-single-' || t.id, t.id,
    'Picodata Single', 'Single Picodata instance',
    'picodata',
    '{"instances":[{"role":"database","count":1,"cpus":2,"memory_mb":4096,"disk_gb":50}],"replication_factor":1,"shards":1}',
    TRUE
FROM tenants t;

-- Picodata Cluster
INSERT INTO presets (id, tenant_id, name, description, db_kind, topology, is_builtin)
SELECT
    'builtin-pico-cluster-' || t.id, t.id,
    'Picodata Cluster', 'Picodata with 3 instances, 3 shards, HAProxy',
    'picodata',
    '{"instances":[{"role":"database","count":3,"cpus":4,"memory_mb":8192,"disk_gb":100}],"haproxy":{"role":"proxy","count":1,"cpus":2,"memory_mb":2048,"disk_gb":20},"replication_factor":2,"shards":3}',
    TRUE
FROM tenants t;

-- Picodata Scale
INSERT INTO presets (id, tenant_id, name, description, db_kind, topology, is_builtin)
SELECT
    'builtin-pico-scale-' || t.id, t.id,
    'Picodata Scale', 'Picodata with 6 instances, multi-tier deployment',
    'picodata',
    '{"instances":[{"role":"database","count":6,"cpus":8,"memory_mb":16384,"disk_gb":200}],"haproxy":{"role":"proxy","count":2,"cpus":2,"memory_mb":2048,"disk_gb":20},"replication_factor":3,"shards":6,"tiers":[{"name":"compute","replication_factor":1,"can_vote":true,"count":3},{"name":"storage","replication_factor":2,"can_vote":false,"count":3}]}',
    TRUE
FROM tenants t;

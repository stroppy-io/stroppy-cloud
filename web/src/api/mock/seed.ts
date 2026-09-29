import type {
  ApiToken,
  AuditEntry,
  CatalogDatabase,
  CatalogProvider,
  DatabaseKind,
  Example,
  Invite,
  Me,
  Member,
  MetricDef,
  ProviderProfile,
  QuotaReport,
  Run,
  RunPhase,
  RunStatus,
  Schedule,
  Schemas,
  Share,
  StroppyCatalog,
  SuiteRun,
  Tenant,
  UserRef,
  Webhook,
  WebhookDelivery,
} from '@api/types'
import { expandCapacityDemo } from './capacity'
import { capacitySizes } from './capacity-sizes'
import { progressSegments, segmentBudgets, syncLiveSummary } from './live-summary'
import type { Database, MockStore, RunLive, Suite, TenantData, Test, Workload } from './store'
import { daysAgo, durationStr, hoursAgo, iso, minutesAgo, pick, rng, uuid } from './util'

// ---------- users ----------
const ADMIN_ID = 'u-admin'
export const users: { ref: UserRef; email: string; admin: boolean }[] = [
  {
    ref: { id: ADMIN_ID, display_name: 'Alex Morgan' },
    email: 'alex@stroppy.example',
    admin: true,
  },
  { ref: { id: 'u-olga', display_name: 'Olga Petrova' }, email: 'olga@example.com', admin: false },
  { ref: { id: 'u-max', display_name: 'Max Ivanov' }, email: 'max@example.com', admin: false },
  { ref: { id: 'u-lena', display_name: 'Lena Sidorova' }, email: 'lena@example.com', admin: false },
]
const [admin, olga, max, lena] = users.map((u) => u.ref)

// ---------- catalog ----------
const pgVersions = ['18', '17', '16', '15']
export function catalogDatabases(): CatalogDatabase[] {
  const mk = (
    kind: DatabaseKind,
    title: string,
    versions: string[],
    roles: CatalogDatabase['roles'],
    topologies: CatalogDatabase['topologies'],
    protocols: CatalogDatabase['protocols'],
    deployable = true
  ): CatalogDatabase => ({
    kind,
    title,
    versions: versions.map((v, i) => ({
      version: v,
      default: i === 1 || versions.length === 1,
      image: `${kind}:${v}`,
    })),
    roles,
    topologies,
    params_schema: `db.${kind}.params`,
    protocols,
    deployable,
  })
  return [
    mk(
      'postgres',
      'PostgreSQL',
      pgVersions,
      [
        {
          role: 'master',
          title: 'Primary',
          engine: 'postgres',
          config_schemas: ['cfg.postgresql.conf', 'cfg.pg_hba.conf'],
        },
        {
          role: 'replica',
          title: 'Replica',
          engine: 'postgres',
          config_schemas: ['cfg.postgresql.conf'],
        },
        { role: 'etcd', title: 'etcd', engine: 'etcd', config_schemas: ['cfg.etcd'] },
        {
          role: 'haproxy',
          title: 'HAProxy',
          engine: 'haproxy',
          config_schemas: ['cfg.haproxy.cfg'],
        },
        {
          role: 'pgbouncer',
          title: 'PgBouncer',
          engine: 'pgbouncer',
          config_schemas: ['cfg.pgbouncer.ini'],
        },
      ],
      [
        {
          id: 'single',
          title: 'Single node',
          description: 'One primary, no replicas.',
          params: { replicas: 0, ha: 'none' },
        },
        {
          id: 'primary-replica',
          title: 'Primary + replicas',
          description: 'Streaming replication, manual failover.',
          params: { replicas: 2, ha: 'none' },
        },
        {
          id: 'patroni-ha',
          title: 'Patroni HA',
          description: 'Patroni + etcd + HAProxy.',
          params: { replicas: 2, ha: 'patroni', etcd_nodes: 3, haproxy: 2 },
        },
      ],
      ['pg']
    ),
    mk(
      'mysql',
      'MySQL',
      ['8.4', '8.0'],
      [
        { role: 'master', title: 'Primary', engine: 'mysql', config_schemas: ['cfg.my.cnf'] },
        { role: 'replica', title: 'Replica', engine: 'mysql', config_schemas: ['cfg.my.cnf'] },
        {
          role: 'proxysql',
          title: 'ProxySQL',
          engine: 'proxysql',
          config_schemas: ['cfg.proxysql.cnf'],
        },
      ],
      [
        { id: 'single', title: 'Single node', params: { replicas: 0 } },
        { id: 'source-replica', title: 'Source + replicas', params: { replicas: 2 } },
      ],
      ['mysql']
    ),
    mk(
      'mariadb',
      'MariaDB',
      ['11.8', '11.4', '10.11'],
      [
        {
          role: 'master',
          title: 'Primary',
          engine: 'mariadb',
          config_schemas: ['cfg.mariadb.cnf'],
        },
        {
          role: 'replica',
          title: 'Replica',
          engine: 'mariadb',
          config_schemas: ['cfg.mariadb.cnf'],
        },
        {
          role: 'maxscale',
          title: 'MaxScale',
          engine: 'maxscale',
          config_schemas: ['cfg.maxscale.cnf'],
        },
      ],
      [
        { id: 'single', title: 'Single node', params: { replicas: 0 } },
        { id: 'galera', title: 'Galera cluster', params: { replicas: 2, galera: true } },
      ],
      ['mysql']
    ),
    mk(
      'picodata',
      'Picodata',
      ['26.1', '26', '25'],
      [
        {
          role: 'instance',
          title: 'Instance',
          engine: 'picodata',
          config_schemas: ['cfg.picodata.yaml'],
        },
      ],
      [
        { id: 'single', title: 'Single instance', params: { instances: 1 } },
        { id: 'cluster-3', title: '3 instances', params: { instances: 3, replication_factor: 2 } },
      ],
      ['picodata']
    ),
    mk(
      'ydb',
      'YDB',
      ['26.3', '26.2', '26.1', '25.4'],
      [
        {
          role: 'storage',
          title: 'Storage',
          engine: 'ydb',
          config_schemas: ['cfg.ydb.config.yaml'],
        },
        {
          role: 'database',
          title: 'Database',
          engine: 'ydb',
          config_schemas: ['cfg.ydb.config.yaml'],
        },
      ],
      [
        { id: 'single', title: 'Single node', params: { nodes: 1 } },
        { id: 'mirror-3-dc', title: 'mirror-3-dc', params: { nodes: 9, erasure: 'mirror-3-dc' } },
      ],
      ['ydb_grpc', 'ydb_grpcs']
    ),
    mk(
      'ydb_managed',
      'YDB (managed)',
      ['latest'],
      [{ role: 'managed', title: 'Managed cluster' }],
      [{ id: 'managed', title: 'Managed', params: {} }],
      ['ydb_grpcs']
    ),
    mk(
      'cockroach',
      'CockroachDB',
      ['26', '25', '24'],
      [
        {
          role: 'node',
          title: 'Node',
          engine: 'cockroach',
          config_schemas: ['cfg.cockroach.flags'],
        },
      ],
      [
        { id: 'single', title: 'Single node', params: { nodes: 1 } },
        { id: 'cluster-3', title: '3 nodes', params: { nodes: 3 } },
      ],
      ['cockroach']
    ),
    mk(
      'orioledb',
      'OrioleDB',
      ['18', '17', '16'],
      [
        {
          role: 'master',
          title: 'Primary',
          engine: 'orioledb',
          config_schemas: ['cfg.orioledb.postgresql.conf'],
        },
      ],
      [{ id: 'single', title: 'Single node', params: { replicas: 0 } }],
      ['pg']
    ),
    mk(
      'external',
      'External (DSN)',
      ['any'],
      [{ role: 'external', title: 'External endpoint' }],
      [{ id: 'external', title: 'External', params: {} }],
      ['pg', 'mysql', 'ydb_grpc', 'cockroach'],
      false
    ),
    mk(
      'noop',
      'No-op',
      ['1'],
      [{ role: 'noop', title: 'No-op' }],
      [{ id: 'noop', title: 'No-op', params: {} }],
      ['noop'],
      false
    ),
    mk(
      'pg_noop',
      'PG no-op',
      ['1'],
      [{ role: 'noop', title: 'PG no-op' }],
      [{ id: 'noop', title: 'No-op', params: {} }],
      ['pg'],
      false
    ),
  ]
}

export const SIZES: Schemas['SizeSpec'][] = [
  { size: 'XS', cpu: 2, memory_gb: 4, instance_type: 'standard-v3-2-4', default_disk_gb: 40 },
  { size: 'S', cpu: 4, memory_gb: 16, instance_type: 'standard-v3-4-16', default_disk_gb: 80 },
  { size: 'M', cpu: 8, memory_gb: 32, instance_type: 'standard-v3-8-32', default_disk_gb: 160 },
  { size: 'L', cpu: 16, memory_gb: 64, instance_type: 'standard-v3-16-64', default_disk_gb: 320 },
  {
    size: 'XL',
    cpu: 32,
    memory_gb: 128,
    instance_type: 'standard-v3-32-128',
    default_disk_gb: 640,
  },
]

export function catalogProviders(): CatalogProvider[] {
  const sizes = capacitySizes.yandex
  return [
    {
      kind: 'yandex',
      title: 'Yandex Cloud',
      settings_schema: 'provider.yandex.settings',
      credentials_schema: 'provider.yandex.credentials',
      locations: [
        { id: 'ru-central1-a', title: 'ru-central1-a' },
        { id: 'ru-central1-b', title: 'ru-central1-b' },
        { id: 'ru-central1-d', title: 'ru-central1-d' },
      ],
      platforms: [
        { id: 'standard-v3', title: 'Intel Ice Lake' },
        { id: 'standard-v2', title: 'Intel Cascade Lake' },
      ],
      disk_types: [
        { id: 'network-ssd', title: 'Network SSD', min_gb: 10, step_gb: 1 },
        { id: 'network-ssd-nonreplicated', title: 'Non-replicated SSD', min_gb: 93, step_gb: 93 },
        { id: 'network-hdd', title: 'Network HDD', min_gb: 10, step_gb: 1 },
      ],
      sizes,
      images: [{ id: 'ubuntu-2404-lts', os: 'Ubuntu 24.04' }],
    },
    {
      kind: 'aws',
      title: 'AWS',
      settings_schema: 'provider.aws.settings',
      credentials_schema: 'provider.aws.credentials',
      locations: [
        { id: 'eu-central-1', title: 'Frankfurt' },
        { id: 'eu-north-1', title: 'Stockholm' },
        { id: 'us-east-1', title: 'N. Virginia' },
      ],
      disk_types: [
        { id: 'gp3', title: 'gp3', min_gb: 8, step_gb: 1 },
        { id: 'io2', title: 'io2', min_gb: 8, step_gb: 1 },
      ],
      sizes: capacitySizes.aws,
      images: [{ id: 'ami-ubuntu-2404', os: 'Ubuntu 24.04' }],
    },
  ]
}

export function stroppyCatalog(): StroppyCatalog {
  const scripts: Schemas['StroppyScript'][] = [
    {
      id: 'tpcc/tx',
      title: 'TPC-C (transactions)',
      protocols: ['pg', 'mysql', 'cockroach', 'picodata'],
      steps: [
        { id: 'load_data', title: 'Load data', phase: 'bootstrap' },
        { id: 'run', title: 'Run', phase: 'workload' },
      ],
      params: [
        { name: 'warehouses', config: 'warehouses', type: 'int', scope: 'workload' },
        { name: 'load-workers', config: 'loadWorkers', type: 'int', scope: 'workload' },
      ],
    },
    {
      id: 'tpcc/procs',
      title: 'TPC-C (stored procedures)',
      protocols: ['pg'],
      steps: [
        { id: 'load_data', title: 'Load data', phase: 'bootstrap' },
        { id: 'run', title: 'Run', phase: 'workload' },
      ],
    },
    {
      id: 'tpcb/tx',
      title: 'TPC-B',
      protocols: ['pg', 'mysql'],
      steps: [
        { id: 'load_data', title: 'Load data', phase: 'bootstrap' },
        { id: 'run', title: 'Run', phase: 'workload' },
      ],
      params: [{ name: 'scale-factor', config: 'scaleFactor', type: 'int', scope: 'workload' }],
    },
    {
      id: 'tpch/tx',
      title: 'TPC-H',
      protocols: ['pg', 'ydb_grpc', 'cockroach'],
      steps: [
        { id: 'load_data', title: 'Load data', phase: 'bootstrap' },
        { id: 'run', title: 'Queries', phase: 'workload' },
      ],
      params: [{ name: 'scale-factor', config: 'scaleFactor', type: 'int', scope: 'workload' }],
    },
    {
      id: 'tpcds',
      title: 'TPC-DS',
      protocols: ['pg'],
      steps: [
        { id: 'load_data', phase: 'bootstrap' },
        { id: 'run', phase: 'workload' },
      ],
    },
    {
      id: 'simple',
      title: 'Simple KV',
      protocols: ['pg', 'mysql', 'ydb_grpc', 'picodata', 'cockroach'],
      steps: [{ id: 'run', phase: 'workload' }],
      params: [{ name: 'rows', config: 'rows', type: 'int64', scope: 'workload' }],
    },
    {
      id: 'execute_sql',
      title: 'Custom SQL',
      protocols: ['pg', 'mysql', 'cockroach'],
      steps: [{ id: 'run', phase: 'workload' }],
    },
  ]
  return {
    source: 'static',
    versions: [
      {
        version: '6.1.0',
        image: 'ghcr.io/stroppy-io/stroppy:6.1.0',
        default: true,
        baseline: true,
        protocols: ['pg', 'mysql', 'ydb_grpc', 'ydb_grpcs', 'picodata', 'cockroach', 'noop'],
        scripts,
      },
      {
        version: '6.0.0',
        image: 'ghcr.io/stroppy-io/stroppy:6.0.0',
        protocols: ['pg', 'mysql', 'ydb_grpc', 'picodata', 'cockroach', 'noop'],
        scripts,
      },
      {
        version: 'nightly-3f1c2a',
        image: 'ghcr.io/stroppy-io/stroppy:nightly-3f1c2a',
        protocols: ['pg', 'mysql', 'ydb_grpc', 'ydb_grpcs', 'picodata', 'cockroach', 'noop'],
        scripts,
      },
    ],
  }
}

export function metricDefs(): MetricDef[] {
  return [
    {
      key: 'queries_per_second',
      title: 'Queries / s',
      unit: 'qps',
      higher_is_better: true,
      group: 'Throughput',
    },
    {
      key: 'tps',
      title: 'Transactions / s',
      unit: 'tps',
      higher_is_better: true,
      group: 'Throughput',
    },
    {
      key: 'latency_p50_ms',
      title: 'Latency p50',
      unit: 'ms',
      higher_is_better: false,
      group: 'Latency',
    },
    {
      key: 'latency_p95_ms',
      title: 'Latency p95',
      unit: 'ms',
      higher_is_better: false,
      group: 'Latency',
    },
    {
      key: 'latency_p99_ms',
      title: 'Latency p99',
      unit: 'ms',
      higher_is_better: false,
      group: 'Latency',
    },
    { key: 'errors', title: 'Errors', unit: 'count', higher_is_better: false, group: 'Errors' },
    {
      key: 'error_rate',
      title: 'Error rate',
      unit: 'percent',
      higher_is_better: false,
      group: 'Errors',
    },
    { key: 'db_cpu', title: 'DB CPU', unit: 'percent', higher_is_better: false, group: 'Database' },
    {
      key: 'db_cache_hit',
      title: 'Cache hit ratio',
      unit: 'percent',
      higher_is_better: true,
      group: 'Database',
    },
    {
      key: 'db_io_read',
      title: 'Disk read',
      unit: 'Bps',
      higher_is_better: false,
      group: 'Database',
    },
    {
      key: 'db_io_write',
      title: 'Disk write',
      unit: 'Bps',
      higher_is_better: false,
      group: 'Database',
    },
    {
      key: 'runner_cpu',
      title: 'Runner CPU',
      unit: 'percent',
      higher_is_better: false,
      group: 'Runner',
    },
  ] as MetricDef[]
}

export function examples(): Example[] {
  return [
    {
      id: 'ex-pg-selfcheck',
      kind: 'test',
      title: 'PostgreSQL self-check',
      description: 'Single node, TPC-B, 2 minutes. Verifies the whole pipeline end to end.',
      db_kind: 'postgres',
      tags: { purpose: 'self-check' },
    },
    {
      id: 'ex-pg-patroni',
      kind: 'database',
      title: 'PostgreSQL Patroni HA',
      description: 'Primary + 2 replicas, etcd×3, HAProxy×2.',
      db_kind: 'postgres',
      tags: { topology: 'ha' },
    },
    {
      id: 'ex-tpcc-medium',
      kind: 'workload',
      title: 'TPC-C 100 warehouses',
      description: '10 minutes constant-vus, 64 VUs.',
      tags: { script: 'tpcc' },
    },
    {
      id: 'ex-tpch-split',
      kind: 'test',
      title: 'TPC-C / TPC-H split',
      description: 'OLTP then OLAP on the same stand, two segments.',
      db_kind: 'postgres',
    },
    {
      id: 'ex-ydb-mirror',
      kind: 'database',
      title: 'YDB mirror-3-dc',
      description: '9 storage nodes, 3 fail domains.',
      db_kind: 'ydb',
    },
    {
      id: 'ex-mysql-suite',
      kind: 'suite',
      title: 'MySQL versions matrix',
      description: '8.0 vs 8.4 across S/M/L sizes.',
      db_kind: 'mysql',
    },
    { id: 'ex-cockroach-3', kind: 'test', title: 'CockroachDB 3-node TPC-C', db_kind: 'cockroach' },
    { id: 'ex-picodata', kind: 'test', title: 'Picodata simple KV', db_kind: 'picodata' },
  ]
}

// ---------- tenant fixtures ----------
const now = Date.now()

function entity(
  id: string,
  name: string,
  author: UserRef,
  ageDays: number,
  extra: Partial<Schemas['Entity']> = {}
): Schemas['Entity'] {
  return {
    id,
    name,
    author,
    created_at: daysAgo(ageDays),
    updated_at: daysAgo(Math.max(0, ageDays - 1)),
    ...extra,
  }
}

function databases(): Database[] {
  const mk = (
    id: string,
    name: string,
    kind: DatabaseKind,
    version: string,
    params: Record<string, unknown>,
    topo: Schemas['TopologyPreview'],
    req: Schemas['Requirements'],
    author: UserRef,
    age: number,
    extra: Partial<Schemas['Entity']> = {}
  ): Database => ({
    ...entity(id, name, author, age, extra),
    kind,
    version,
    params: { version, ...params },
    schema: { id: `db.${kind}.params`, version: '1' },
    topology_preview: topo,
    requirements: req,
    configs: {},
  })
  return [
    mk(
      'db-pg-single',
      'pg-17-single',
      'postgres',
      '17',
      { replicas: 0, ha: 'none', extensions: ['pg_stat_statements'] },
      { label: 'single', node_count: 1, nodes: [{ role: 'master', engine: 'postgres', count: 1 }] },
      { db: { cpu: 2, memory_gb: 4, disk_gb: 40, reason: 'shared_buffers 1GB' } },
      olga,
      40,
      {
        description: 'Baseline single-node PostgreSQL 17.',
        tags: { env: 'baseline' },
        is_favorite: true,
      }
    ),
    mk(
      'db-pg-ha',
      'pg-17-patroni-ha',
      'postgres',
      '17',
      { replicas: 2, sync_replicas: 1, ha: 'patroni', etcd_nodes: 3, haproxy: 2, pgbouncer: true },
      {
        label: 'patroni-ha',
        node_count: 8,
        nodes: [
          { role: 'master', engine: 'postgres', count: 1 },
          { role: 'replica', engine: 'postgres', count: 2 },
          { role: 'etcd', engine: 'etcd', count: 3 },
          { role: 'haproxy', engine: 'haproxy', count: 2 },
          { role: 'pgbouncer', engine: 'pgbouncer', count: 3, colocated_with: 'master' },
        ],
        flows: [
          { from: 'haproxy', to: 'master', protocol: 'pg', port: 5432 },
          { from: 'master', to: 'replica', protocol: 'pg', port: 5432 },
          { from: 'master', to: 'etcd', protocol: 'http', port: 2379 },
        ],
      },
      {
        db: { cpu: 8, memory_gb: 32, disk_gb: 200, reason: 'shared_buffers 8GB' },
        proxy: { cpu: 2, memory_gb: 4 },
        etcd: { cpu: 2, memory_gb: 4 },
      },
      max,
      25,
      {
        description: 'Patroni + etcd + HAProxy, synchronous replica.',
        tags: { env: 'ha', team: 'core' },
      }
    ),
    mk(
      'db-pg-18',
      'pg-18-replica',
      'postgres',
      '18',
      { replicas: 1, ha: 'none' },
      {
        label: 'primary-replica',
        node_count: 2,
        nodes: [
          { role: 'master', engine: 'postgres', count: 1 },
          { role: 'replica', engine: 'postgres', count: 1 },
        ],
      },
      { db: { cpu: 4, memory_gb: 16, disk_gb: 80 } },
      olga,
      5,
      { tags: { env: 'preview' } }
    ),
    mk(
      'db-mysql-84',
      'mysql-8.4-source-replica',
      'mysql',
      '8.4',
      { replicas: 2 },
      {
        label: 'source-replica',
        node_count: 3,
        nodes: [
          { role: 'master', engine: 'mysql', count: 1 },
          { role: 'replica', engine: 'mysql', count: 2 },
        ],
      },
      { db: { cpu: 4, memory_gb: 16, disk_gb: 80, reason: 'innodb_buffer_pool 8GB' } },
      lena,
      18
    ),
    mk(
      'db-ydb-3dc',
      'ydb-26-mirror-3-dc',
      'ydb',
      '26',
      { nodes: 9, erasure: 'mirror-3-dc', pdisks_per_node: 3 },
      {
        label: 'mirror-3-dc',
        node_count: 9,
        nodes: [
          { role: 'storage', engine: 'ydb', count: 9 },
          { role: 'database', engine: 'ydb', count: 3, colocated_with: 'storage' },
        ],
      },
      { db: { cpu: 8, memory_gb: 32, disk_gb: 300, reason: '3 pdisks × 93GB' } },
      max,
      12,
      { description: '9 storage nodes, 3 fail domains.', tags: { env: 'ha' } }
    ),
    mk(
      'db-crdb-3',
      'cockroach-25-3n',
      'cockroach',
      '25',
      { nodes: 3 },
      {
        label: 'cluster-3',
        node_count: 3,
        nodes: [{ role: 'node', engine: 'cockroach', count: 3 }],
      },
      { db: { cpu: 4, memory_gb: 16, disk_gb: 100 } },
      lena,
      9
    ),
    mk(
      'db-picodata',
      'picodata-26-3i',
      'picodata',
      '26',
      { instances: 3, replication_factor: 2 },
      {
        label: 'cluster-3',
        node_count: 3,
        nodes: [{ role: 'instance', engine: 'picodata', count: 3 }],
      },
      { db: { cpu: 4, memory_gb: 16, disk_gb: 60, reason: 'memtx 8GB' } },
      olga,
      3
    ),
    mk(
      'db-oriole',
      'orioledb-17',
      'orioledb',
      '17',
      { replicas: 0 },
      { label: 'single', node_count: 1, nodes: [{ role: 'master', engine: 'orioledb', count: 1 }] },
      { db: { cpu: 4, memory_gb: 16, disk_gb: 80 } },
      max,
      2
    ),
  ]
}

function workloads(): Workload[] {
  const seg = (
    name: string,
    script: string,
    params: Record<string, unknown>,
    run: Record<string, unknown>
  ) => ({
    name,
    workload: { script, ...params },
    run: { executor: 'constant-vus', ...run },
    thresholds: { p99_ms: 200, error_rate: 0.01 },
  })
  const mk = (
    id: string,
    name: string,
    protocol: Schemas['Protocol'],
    segments: Record<string, unknown>[],
    runner: Schemas['RoleRequirement'],
    author: UserRef,
    age: number,
    extra: Partial<Schemas['Entity']> = {}
  ): Workload => ({
    ...entity(id, name, author, age, extra),
    stroppy_version: '6.1.0',
    protocol,
    segments,
    options: {
      driver: { default_insert_method: 'copy', bulk_size: 1000 },
      connection: { kind: protocol },
      baseline: { enabled: true, tiers: ['noop', 'wire'], quick: true },
    },
    schema: { id: 'workload.stroppy', version: '1' },
    requirements: { runner },
  })
  return [
    mk(
      'wl-tpcc-64',
      'tpcc-100wh-64vu-10m',
      'pg',
      [seg('tpcc', 'tpcc/tx', { warehouses: 100, load_workers: 32 }, { vus: 64, duration: '10m' })],
      { cpu: 8, memory_gb: 16, reason: '64 VUs' },
      olga,
      30,
      {
        description: 'TPC-C, 100 warehouses, 64 virtual users for 10 minutes.',
        tags: { script: 'tpcc' },
        is_favorite: true,
      }
    ),
    mk(
      'wl-tpcb-quick',
      'tpcb-sf10-2m',
      'pg',
      [seg('tpcb', 'tpcb/tx', { scale_factor: 10 }, { vus: 16, duration: '2m' })],
      { cpu: 2, memory_gb: 4 },
      olga,
      30,
      {
        description: 'Quick smoke: TPC-B scale 10, 2 minutes.',
        tags: { script: 'tpcb', purpose: 'smoke' },
      }
    ),
    mk(
      'wl-split',
      'tpcc-then-tpch',
      'pg',
      [
        seg('oltp', 'tpcc/tx', { warehouses: 50 }, { vus: 32, duration: '5m' }),
        seg(
          'olap',
          'tpch/tx',
          { scale_factor: 5 },
          { executor: 'shared-iterations', vus: 4, iterations: 22 }
        ),
      ],
      { cpu: 8, memory_gb: 32 },
      max,
      15,
      { tags: { script: 'mixed' } }
    ),
    mk(
      'wl-mysql-tpcc',
      'mysql-tpcc-50wh',
      'mysql',
      [seg('tpcc', 'tpcc/tx', { warehouses: 50 }, { vus: 32, duration: '5m' })],
      { cpu: 4, memory_gb: 8 },
      lena,
      14,
      { tags: { script: 'tpcc' } }
    ),
    mk(
      'wl-simple-ydb',
      'ydb-simple-kv',
      'ydb_grpc',
      [seg('kv', 'simple', { rows: '10000000' }, { vus: 128, duration: '10m' })],
      { cpu: 16, memory_gb: 32, reason: '128 VUs' },
      max,
      10
    ),
    mk(
      'wl-crdb',
      'cockroach-tpcc-20wh',
      'cockroach',
      [seg('tpcc', 'tpcc/tx', { warehouses: 20 }, { vus: 16, duration: '3m' })],
      { cpu: 4, memory_gb: 8 },
      lena,
      8
    ),
  ]
}

const sizesFor = (
  db: Schemas['Size'],
  runner: Schemas['Size'],
  extra: Record<string, Schemas['Size']> = {}
): Schemas['RoleSizes'] => {
  const out: Schemas['RoleSizes'] = {
    db: { size: db, disk: { type: 'network-ssd', gb: 100 } },
    runner: { size: runner },
  }
  for (const [k, v] of Object.entries(extra)) out[k] = { size: v }
  return out
}

function tests(provOk: string, provAws: string): Test[] {
  const mk = (
    id: string,
    name: string,
    dbId: string,
    wlId: string,
    sizes: Schemas['RoleSizes'],
    provider: string | null,
    status: Schemas['TestStatus'],
    author: UserRef,
    age: number,
    extra: Partial<Schemas['Entity']> = {},
    fit: Schemas['Fit'] = { fits: true }
  ): Test => ({
    ...entity(id, name, author, age, extra),
    status,
    database: { ref: { id: dbId } },
    workload: { ref: { id: wlId } },
    sizes,
    provider_profile_id: provider,
    keep: '0s',
    rating: { tenant: true, global: false },
    validation: fit,
  })
  return [
    mk(
      't-pg-smoke',
      'pg-17 smoke',
      'db-pg-single',
      'wl-tpcb-quick',
      sizesFor('S', 'XS'),
      provOk,
      'ready',
      olga,
      28,
      {
        description: 'Daily smoke of the single-node stand.',
        tags: { purpose: 'smoke' },
        is_favorite: true,
      }
    ),
    mk(
      't-pg-ha-tpcc',
      'pg-17 Patroni TPC-C 64vu',
      'db-pg-ha',
      'wl-tpcc-64',
      sizesFor('L', 'M', { proxy: 'S', etcd: 'XS' }),
      provOk,
      'ready',
      max,
      20,
      { tags: { env: 'ha', team: 'core' }, is_favorite: true }
    ),
    mk(
      't-pg-18',
      'pg-18 replica TPC-C',
      'db-pg-18',
      'wl-tpcc-64',
      sizesFor('M', 'M'),
      provOk,
      'needs_attention',
      olga,
      4,
      { tags: { env: 'preview' } },
      {
        fits: true,
        stale: { database: true },
        issues: [
          {
            path: 'database',
            code: 'STALE_REF',
            severity: 'WARNING',
            message: 'Database pg-18-replica changed after this test was validated.',
          },
        ],
      }
    ),
    mk(
      't-split',
      'OLTP→OLAP split',
      'db-pg-ha',
      'wl-split',
      sizesFor('XL', 'L', { proxy: 'S', etcd: 'XS' }),
      provAws,
      'ready',
      max,
      12
    ),
    mk(
      't-mysql',
      'mysql 8.4 TPC-C',
      'db-mysql-84',
      'wl-mysql-tpcc',
      sizesFor('M', 'S'),
      provOk,
      'ready',
      lena,
      13
    ),
    mk(
      't-ydb',
      'ydb mirror-3-dc KV',
      'db-ydb-3dc',
      'wl-simple-ydb',
      sizesFor('M', 'L'),
      provOk,
      'ready',
      max,
      9
    ),
    mk(
      't-crdb',
      'cockroach 3n TPC-C',
      'db-crdb-3',
      'wl-crdb',
      sizesFor('S', 'S'),
      provOk,
      'draft',
      lena,
      1,
      {},
      {
        fits: false,
        issues: [
          {
            path: 'sizes.db.size',
            code: 'SIZE_TOO_SMALL',
            severity: 'ERROR',
            message:
              'cockroach needs at least 4 CPU per node; S has 4 but memory 16GB < 24GB required',
            suggested: 'M',
          },
        ],
      }
    ),
    mk(
      't-picodata',
      'picodata KV quick',
      'db-picodata',
      'wl-simple-ydb',
      sizesFor('S', 'M'),
      null,
      'draft',
      olga,
      0,
      {},
      {
        fits: false,
        issues: [
          {
            path: 'workload',
            code: 'PROTOCOL_MISMATCH',
            severity: 'ERROR',
            message: 'workload protocol ydb_grpc does not match picodata',
          },
          {
            path: 'provider_profile_id',
            code: 'REQUIRED',
            severity: 'ERROR',
            message: 'Pick a provider profile before launching',
          },
        ],
      }
    ),
  ]
}

const PHASES: RunPhase[] = ['provisioning', 'deploying', 'workload', 'collecting', 'teardown']
const PHASE_TITLES: Record<RunPhase, string> = {
  queued: 'Queued',
  provisioning: 'Provisioning',
  deploying: 'Deploying',
  workload: 'Workload',
  collecting: 'Collecting',
  teardown: 'Teardown',
  done: 'Done',
}

export interface RunSeed {
  id: string
  name: string
  test: Test
  db: Database
  wl: Workload
  status: RunStatus
  phase: RunPhase
  startedMinAgo: number
  durationMin: number
  author: UserRef
  provider: ProviderProfile
  trigger?: Run['trigger']
  trigger_ref?: Run['trigger_ref']
  tps?: number
  p99?: number
  errors?: number
  keep?: boolean
  labels?: Record<string, string>
  notes?: string
  reason?: string
  favorite?: boolean
}

function machinesFor(
  db: Database,
  sizes: Schemas['RoleSizes'],
  provider?: ProviderProfile
): NonNullable<Schemas['RunSnapshot']['machines']> {
  const out: NonNullable<Schemas['RunSnapshot']['machines']> = []
  for (const n of db.topology_preview?.nodes ?? []) {
    if (n.colocated_with) continue
    const family = ['etcd', 'coordinator'].includes(n.role)
      ? 'coordinator'
      : ['haproxy', 'proxy', 'proxysql'].includes(n.role)
        ? 'proxy'
        : 'db'
    const roleSize = sizes[n.role] ?? sizes[family] ?? sizes.db
    const table = capacitySizes[provider?.kind ?? 'yandex'][family]
    const spec = table.find((s) => s.size === roleSize.size) ?? table[1]
    for (let i = 1; i <= n.count; i++)
      out.push({
        name: `${n.role}-${i}`,
        role: n.role,
        size: spec.size,
        cpu: spec.cpu,
        memory_gb: spec.memory_gb,
        disk_gb: roleSize.disk?.gb ?? spec.default_disk_gb,
        disk_type: roleSize.disk?.type ?? 'network-ssd',
        instance_type: spec.instance_type,
        location: provider?.kind === 'aws' ? 'eu-central-1' : 'ru-central1-a',
      })
  }
  const runnerSizes = capacitySizes[provider?.kind ?? 'yandex'].runner
  const r = runnerSizes.find((s) => s.size === sizes.runner?.size) ?? runnerSizes[1]
  out.push({
    name: 'runner-1',
    role: 'runner',
    size: r.size,
    cpu: r.cpu,
    memory_gb: r.memory_gb,
    disk_gb: r.default_disk_gb,
    disk_type: 'network-ssd',
    instance_type: r.instance_type,
    location: provider?.kind === 'aws' ? 'eu-central-1' : 'ru-central1-a',
  })
  return out
}

export function buildRun(s: RunSeed): Run {
  const started = now - s.startedMinAgo * 60_000
  const finished = started + s.durationMin * 60_000
  const terminal = s.status === 'completed' || s.status === 'failed' || s.status === 'cancelled'
  const sizes = s.test.sizes ?? sizesFor('M', 'M')
  const segNames = (s.wl.segments as { name: string }[]).map((x) => x.name)
  const result: Schemas['RunResult'] | undefined =
    s.status === 'completed'
      ? {
          metrics: {
            queries_per_second: { value: (s.tps ?? 0) * 4, unit: 'qps' },
            tps: { value: s.tps ?? 0, unit: 'tps' },
            latency_p50_ms: { value: (s.p99 ?? 0) * 0.3, unit: 'ms' },
            latency_p95_ms: { value: (s.p99 ?? 0) * 0.7, unit: 'ms' },
            latency_p99_ms: { value: s.p99 ?? 0, unit: 'ms' },
            errors: { value: s.errors ?? 0, unit: 'count' },
            error_rate: {
              value: ((s.errors ?? 0) / Math.max(1, (s.tps ?? 1) * s.durationMin * 60)) * 100,
              unit: 'percent',
            },
          },
          segments: segNames.map((name) => ({
            name,
            status: 'completed' as const,
            exit_code: 0,
            metrics: { tps: { value: s.tps ?? 0, unit: 'tps' } },
            errors: {
              terminal_errors: 0,
              failed_iterations: s.errors ?? 0,
              failed_queries: s.errors ?? 0,
              retry_attempts: 0,
            },
          })),
          baseline: {
            ok: true,
            verdicts: [
              { check: 'noop-tier', status: 'ok', detail: 'runner overhead 0.4%' },
              { check: 'wire-tier', status: 'ok' },
            ],
          },
          summary: {
            tps: s.tps,
            latency_p50_ms: (s.p99 ?? 0) * 0.3,
            latency_p95_ms: (s.p99 ?? 0) * 0.7,
            latency_p99_ms: s.p99,
            errors: s.errors ?? 0,
            duration: durationStr(s.durationMin * 60_000),
          },
          artifacts: ['art-raw', 'art-report'],
        }
      : s.status === 'failed'
        ? {
            segments: segNames.map((name, i) => ({
              name,
              status: i === 0 ? ('failed' as const) : ('skipped' as const),
              exit_code: i === 0 ? 1 : undefined,
              error: i === 0 ? s.reason : undefined,
            })),
          }
        : undefined
  return {
    id: s.id,
    name: s.name,
    status: s.status,
    phase: s.phase,
    status_reason: s.reason,
    stand_kept: !!s.keep,
    keep_until: s.keep ? iso(now + 2 * 3600_000) : null,
    trigger: s.trigger ?? 'manual',
    trigger_ref: s.trigger_ref,
    test_ref: { id: s.test.id, name: s.test.name },
    snapshot: {
      database: {
        kind: s.db.kind,
        version: s.db.version,
        params: s.db.params,
        schema: s.db.schema,
        configs: s.db.configs,
      },
      database_name: s.db.name,
      workload: {
        stroppy_version: s.wl.stroppy_version,
        protocol: s.wl.protocol,
        segments: s.wl.segments,
        options: s.wl.options,
        schema: s.wl.schema,
      },
      workload_name: s.wl.name,
      sizes,
      provider_profile: { id: s.provider.id, name: s.provider.name },
      keep: s.keep ? '2h' : '0s',
      machines: machinesFor(s.db, sizes, s.provider),
    },
    run_spec: {
      schema: { id: 'spec.run', version: '1' },
      values: {
        name: s.name,
        database: { kind: s.db.kind, version: s.db.version },
        machines: machinesFor(s.db, sizes, s.provider).map((m) => ({
          name: m.name,
          role: m.role,
          size: m.size,
        })),
        workload: {
          stroppy_image: `ghcr.io/stroppy-io/stroppy:${s.wl.stroppy_version}`,
          segments: s.wl.segments,
        },
      },
    },
    rating: { tenant: true, global: s.status === 'completed' && s.favorite === true },
    notes: s.notes,
    labels: s.labels,
    author: s.author,
    is_favorite: s.favorite,
    created_at: iso(started - 20_000),
    started_at: iso(started),
    finished_at: terminal ? iso(finished) : null,
    duration: terminal ? durationStr(finished - started) : null,
    summary: {
      db_kind: s.db.kind,
      db_version: s.db.version,
      workload_name: s.wl.name,
      protocol: s.wl.protocol,
      stroppy_version: s.wl.stroppy_version,
      topology_label: s.db.topology_preview?.label,
      node_count: s.db.topology_preview?.node_count,
      provider_kind: s.provider.kind,
      provider_profile: { id: s.provider.id, name: s.provider.name },
      sizes,
      league: `${sizes.db?.size ?? 'M'}/${sizes.runner?.size ?? 'M'}`,
      progress_pct: terminal ? 100 : s.status === 'pending' ? 0 : 40,
      segment: s.status === 'running' && s.phase === 'workload' ? segNames[0] : undefined,
      headline:
        s.status === 'completed'
          ? {
              tps: s.tps ?? 0,
              qps: (s.tps ?? 0) * 4,
              latency_p50_ms: (s.p99 ?? 0) * 0.3,
              latency_p95_ms: (s.p99 ?? 0) * 0.7,
              latency_p99_ms: s.p99 ?? 0,
              errors: s.errors ?? 0,
            }
          : undefined,
    },
    result,
    graphene: { run_ref: `run/${s.id}`, namespace: 't-main', pipeline_revision: 'a1b2c3d' },
  }
}

// Build a complete "live" record for a terminal run (persisted overview + full event log).
export function buildLive(run: Run, seedNum: number): RunLive {
  const rand = rng(seedNum)
  const started = new Date(run.started_at ?? run.created_at).getTime()
  const finished = run.finished_at ? new Date(run.finished_at).getTime() : now
  const total = Math.max(60_000, finished - started)
  const terminal = ['completed', 'failed', 'cancelled'].includes(run.status)
  const failedPhaseIdx = run.status === 'failed' ? PHASES.indexOf(run.phase) : -1
  const weights = [0.12, 0.18, 0.55, 0.07, 0.08]
  let cursor = started
  const phases: Schemas['RunOverview']['phases'] = PHASES.map((id, i) => {
    const len = total * weights[i]
    const st = cursor
    cursor += len
    let status: (typeof phases)[number]['status'] = 'completed'
    if (!terminal) {
      const cur = PHASES.indexOf(run.phase)
      status = i < cur ? 'completed' : i === cur ? 'running' : 'pending'
    } else if (failedPhaseIdx >= 0) {
      status = i < failedPhaseIdx ? 'completed' : i === failedPhaseIdx ? 'failed' : 'skipped'
    } else if (run.status === 'cancelled') {
      const cur = PHASES.indexOf(run.phase)
      status = i < cur ? 'completed' : i === cur ? 'cancelled' : 'skipped'
    }
    return {
      id,
      title: PHASE_TITLES[id],
      status,
      started_at: status === 'pending' ? null : iso(st),
      finished_at:
        status === 'completed' ||
        status === 'failed' ||
        status === 'skipped' ||
        status === 'cancelled'
          ? iso(st + len)
          : null,
      steps: [],
    }
  })
  const machines = (run.snapshot.machines ?? []).map((m) => ({
    name: m.name,
    role: m.role,
    status: terminal && run.status !== 'failed' ? ('deleted' as const) : ('ready' as const),
    presence: terminal ? ('terminated' as const) : ('online' as const),
    agent_id: `agent-${m.name}`,
    address: `10.0.${Math.floor(rand() * 20)}.${Math.floor(rand() * 250)}`,
    public_ip: `84.201.${Math.floor(rand() * 250)}.${Math.floor(rand() * 250)}`,
    size: m.size,
    provider_resource_id: `fhm${Math.floor(rand() * 1e8).toString(36)}`,
    last_heartbeat_at: iso(terminal ? finished : now - 3000),
  }))
  const components: Schemas['RunOverview']['components'] = machines
    .filter((m) => m.role !== 'runner')
    .map((m) => ({
      id: `${m.role}/${m.name}`,
      role: m.role,
      engine: run.snapshot.database.kind,
      image: `${run.snapshot.database.kind}:${run.snapshot.database.version}`,
      machine: m.name,
      status: terminal ? ('deleted' as const) : ('ready' as const),
      endpoints: [
        {
          name: 'sql',
          address: m.address,
          port: run.snapshot.database.kind === 'postgres' ? 5432 : 3306,
        },
      ],
      scrape: `${m.address}:9187`,
    }))
  const segs = (run.snapshot.workload.segments as { name: string }[]).map((s) => s.name)
  const events: Schemas['RunEvent'][] = []
  let seq = 0
  const ev = (
    at: number,
    kind: string,
    title: string,
    extra: Partial<Schemas['RunEvent']> = {}
  ) => {
    events.push({ id: `${run.id}-e${++seq}`, at: iso(at), kind, title, ...extra })
  }
  ev(started, 'run.started', 'Run started', { subject: run.name })
  for (const p of phases) {
    if (p.started_at === null) break
    ev(new Date(p.started_at ?? started).getTime(), 'phase.started', `${p.title} started`, {
      subject: p.id,
      status: 'running',
    })
    if (p.id === 'provisioning' && p.status !== 'running')
      for (const m of machines)
        ev(
          new Date(p.started_at ?? started).getTime() + rand() * total * 0.1,
          'machine.ready',
          `Machine ${m.name} ready`,
          { subject: m.name, status: 'ready', payload: { address: m.address } }
        )
    if (p.id === 'deploying' && p.status !== 'running')
      for (const c of components)
        ev(
          new Date(p.started_at ?? started).getTime() + rand() * total * 0.15,
          'container.ready',
          `${c.id} healthy`,
          { subject: c.id, status: 'ready' }
        )
    if (p.id === 'workload' && p.status !== 'pending')
      segs.forEach((sname, i) => {
        ev(
          new Date(p.started_at ?? started).getTime() + i * 1000,
          'segment.started',
          `Segment ${sname} started`,
          { subject: sname }
        )
      })
    if (p.finished_at)
      ev(
        new Date(p.finished_at).getTime(),
        p.status === 'failed' ? 'phase.failed' : 'phase.finished',
        `${p.title} ${p.status}`,
        {
          subject: p.id,
          status: p.status,
          error: p.status === 'failed' ? run.status_reason : undefined,
        }
      )
  }
  if (terminal)
    ev(finished, `run.${run.status}`, `Run ${run.status}`, {
      status: run.status,
      error: run.status_reason,
    })
  const logs: Schemas['LogLine'][] = []
  const lines = [
    'checkpoint complete: wrote 1832 buffers (1.4%)',
    'connection received: host=10.0.3.11 port=51322',
    'automatic vacuum of table "tpcc.stock": index scans: 1',
    'LOG:  database system is ready to accept connections',
    'stroppy: segment tpcc iteration 12000 ok p99=41ms',
    'stroppy: warmup finished, starting measurement',
    'haproxy: backend primary status UP',
    'patroni: leader lock acquired by master-1',
    'agent: heartbeat ok load=2.31',
    'ERROR: deadlock detected (retrying)',
  ]
  for (let i = 0; i < 400; i++) {
    const t = started + (total * i) / 400
    const m = machines[Math.floor(rand() * machines.length)]
    const msg = lines[Math.floor(rand() * lines.length)]
    logs.push({
      time: iso(t),
      seq: i + 1,
      message: msg,
      level: msg.startsWith('ERROR') ? 'error' : msg.includes('warmup') ? 'warn' : 'info',
      stream: 'stdout',
      role: m.role,
      machine: m.name,
      container: m.role === 'runner' ? 'stroppy' : `${m.role}`,
      phase:
        phases.find(
          (p) =>
            p.started_at &&
            t >= new Date(p.started_at).getTime() &&
            (!p.finished_at || t <= new Date(p.finished_at).getTime())
        )?.id ?? 'workload',
      segment: segs[0],
    })
  }
  const metrics: Record<string, number[][]> = {}
  const wl = phases[2]
  const wStart = new Date(wl.started_at ?? started).getTime()
  const wEnd = wl.finished_at ? new Date(wl.finished_at).getTime() : now
  const base = run.result?.summary?.tps ?? run.summary?.headline?.tps ?? 900
  const p99 = run.result?.summary?.latency_p99_ms ?? run.summary?.headline?.latency_p99_ms ?? 60
  const pts = (fn: (i: number, x: number) => number) => {
    const out: number[][] = []
    const n = 120
    for (let i = 0; i <= n; i++) out.push([wStart + ((wEnd - wStart) * i) / n, fn(i, i / n)])
    return out
  }
  metrics.tps = pts((i, x) =>
    Math.max(
      0,
      base *
        Math.min(1, x / 0.08) *
        (1 +
          0.06 * Math.sin(i / 6) +
          0.025 * Math.sin(i / 2) -
          0.16 * Math.exp(-((x - 0.64) ** 2) / 0.001)) +
        (rand() - 0.5) * base * 0.025
    )
  )
  metrics.latency_p50_ms = pts((i) => p99 * 0.3 * (1 + 0.1 * Math.sin(i / 9)))
  metrics.latency_p95_ms = pts((i) => p99 * 0.7 * (1 + 0.15 * Math.sin(i / 7)))
  metrics.latency_p99_ms = pts((i) => p99 * (1 + 0.25 * Math.abs(Math.sin(i / 5))) + rand() * 5)
  metrics.errors = pts(() => (rand() < 0.03 ? Math.floor(rand() * 5) : 0))
  metrics.db_cpu = pts((i) => 62 + 25 * Math.abs(Math.sin(i / 11)) + rand() * 5)
  metrics.db_cache_hit = pts(() => 97 + rand() * 2.5)
  metrics.db_io_read = pts((i) => 12e6 + 8e6 * Math.abs(Math.sin(i / 13)))
  metrics.db_io_write = pts((i) => 30e6 + 15e6 * Math.abs(Math.cos(i / 10)))
  metrics.runner_cpu = pts(() => 58 + rand() * 24)
  const cur = PHASES.indexOf(run.phase)
  return {
    overview: {
      run_id: run.id,
      status: run.status,
      phase: run.phase,
      progress_pct: run.summary?.progress_pct,
      source: terminal ? 'persisted' : 'live',
      observed_at: iso(),
      degraded_reasons:
        run.status === 'failed' ? ['agent heartbeat lost on master-1 before teardown'] : undefined,
      phases,
      components,
      machines,
      flows:
        run.snapshot.database.kind === 'postgres'
          ? [
              { from: 'runner-1', to: 'haproxy-1', protocol: 'pg', port: 5432 },
              { from: 'haproxy-1', to: 'master-1', protocol: 'pg', port: 5432 },
            ]
          : [
              {
                from: 'runner-1',
                to: machines[0]?.name,
                protocol: run.snapshot.workload.protocol,
                port: 5432,
              },
            ],
      workload_segments: segs.map((name, i) => ({
        name,
        status: terminal
          ? run.status === 'completed'
            ? 'completed'
            : i === 0
              ? run.status
              : 'skipped'
          : cur >= 2
            ? i === 0
              ? 'running'
              : 'pending'
            : 'pending',
        started_at: cur >= 2 || terminal ? iso(wStart + i * 1000) : null,
        finished_at: terminal ? iso(wEnd) : null,
      })),
      pending_activity:
        run.status === 'running' && run.phase === 'provisioning'
          ? { activity: 'crossplane.wait_ready', attempt: 1, since: minutesAgo(1) }
          : undefined,
    },
    events,
    logs,
    metrics,
    phaseIndex: terminal ? PHASES.length : Math.max(0, cur),
    // The simulation measures the current phase from its recorded start.
    phaseStartedAt: Date.parse(phases[Math.max(0, cur)]?.started_at ?? iso(now)),
    segmentIndex: 0,
    seq: logs.length,
  }
}

function providersFor(): ProviderProfile[] {
  return [
    {
      id: 'prov-yc-main',
      name: 'yc-benchmarks',
      kind: 'yandex',
      status: 'ready',
      verified_at: daysAgo(30),
      settings: {
        cloud_id: 'b1gxxxxxxxxxxxxxxxxx',
        folder_id: 'b1gyyyyyyyyyyyyyyyyy',
        zone: 'ru-central1-a',
        platform_id: 'standard-v3',
      },
      quotas_observed_at: minutesAgo(4),
      created_at: daysAgo(45),
      created_by: max,
    },
    {
      id: 'prov-aws-eu',
      name: 'aws-eu-central',
      kind: 'aws',
      status: 'ready',
      verified_at: daysAgo(10),
      settings: { region: 'eu-central-1', vpc: 'vpc-0a1b2c3d' },
      quotas_observed_at: hoursAgo(1),
      created_at: daysAgo(12),
      created_by: olga,
    },
    {
      id: 'prov-yc-broken',
      name: 'yc-old-key',
      kind: 'yandex',
      status: 'failed',
      status_reason: 'service account key is revoked (iam: 401)',
      verified_at: null,
      settings: {
        cloud_id: 'b1gzzzzzzzzzzzzzzzzz',
        folder_id: 'b1gwwwwwwwwwwwwwwwww',
        zone: 'ru-central1-b',
      },
      created_at: daysAgo(60),
      created_by: lena,
    },
  ]
}

function quotasFor(): Record<string, QuotaReport> {
  return {
    'prov-yc-main': {
      observed_at: minutesAgo(4),
      stale: false,
      scope: 'folder b1gy…',
      quotas: [
        { name: 'compute.instances', title: 'Instances', unit: 'count', limit: 100, used: 23 },
        { name: 'compute.cores', title: 'vCPU', unit: 'count', limit: 512, used: 196 },
        { name: 'compute.memory', title: 'RAM', unit: 'GB', limit: 2048, used: 704 },
        { name: 'compute.ssd', title: 'Network SSD', unit: 'GB', limit: 8192, used: 2100 },
        { name: 'vpc.addresses', title: 'Public IPs', unit: 'count', limit: 32, used: 11 },
      ],
    },
    'prov-aws-eu': {
      observed_at: hoursAgo(1),
      stale: true,
      scope: 'eu-central-1',
      quotas: [
        {
          name: 'ec2.vcpus.standard',
          title: 'On-demand vCPU (standard)',
          unit: 'count',
          limit: 256,
          used: 64,
        },
        { name: 'ebs.gp3', title: 'gp3 storage', unit: 'TB', limit: 50, used: 3.2 },
        { name: 'vpc.eip', title: 'Elastic IPs', unit: 'count', limit: 5, used: 5 },
      ],
    },
    'prov-yc-broken': {
      observed_at: null,
      stale: true,
      unavailable_reason: 'profile failed verification',
      quotas: [],
    },
  }
}

function webhooksFor(): { webhooks: Webhook[]; deliveries: Record<string, WebhookDelivery[]> } {
  const webhooks: Webhook[] = [
    {
      id: 'wh-ci',
      url: 'https://ci.example.com/hooks/stroppy',
      events: ['run.finished', 'run.failed', 'suite.finished'],
      enabled: true,
      description: 'GitLab CI status relay',
      last_delivery: { at: minutesAgo(35), status: 'success' } as Webhook['last_delivery'],
      created_at: daysAgo(20),
    },
    {
      id: 'wh-slack',
      url: 'https://hooks.slack.com/services/T000/B000/XXXX',
      events: ['run.started', 'run.finished', 'run.failed', 'run.cancelled'],
      enabled: false,
      description: 'Slack #benchmarks',
      last_delivery: { at: daysAgo(2), status: 'failed' } as Webhook['last_delivery'],
      created_at: daysAgo(15),
    },
  ]
  const deliveries: Record<string, WebhookDelivery[]> = {
    'wh-ci': Array.from({ length: 12 }, (_, i) => ({
      id: `d-ci-${i}`,
      event: pick(['run.finished', 'run.failed', 'suite.finished'] as const, i),
      status: i === 3 ? 'failed' : 'success',
      attempts: i === 3 ? 5 : 1,
      last_attempt_at: hoursAgo(i * 3),
      response_status: i === 3 ? 502 : 200,
      error: i === 3 ? 'upstream returned 502 Bad Gateway' : undefined,
      payload: { run_id: `run-${i}`, status: 'completed' },
      created_at: hoursAgo(i * 3 + 0.1),
    })),
    'wh-slack': Array.from({ length: 4 }, (_, i) => ({
      id: `d-sl-${i}`,
      event: 'run.finished' as const,
      status: 'failed',
      attempts: 5,
      last_attempt_at: daysAgo(2 + i),
      response_status: 404,
      error: 'channel_not_found',
      created_at: daysAgo(2 + i),
    })),
  }
  return { webhooks, deliveries }
}

function tenantMain(): TenantData {
  const tenant: Tenant = {
    id: 'ten-main',
    slug: 'main',
    name: 'Stroppy Labs',
    description: 'Core team database benchmarks',
    status: 'active',
    owner: admin,
    public_name: 'stroppy-core',
    member_count: 4,
    created_at: daysAgo(90),
  }
  const providers = providersFor()
  const dbs = databases()
  const wls = workloads()
  const ts = tests('prov-yc-main', 'prov-aws-eu')
  const byId = <T extends { id: string }>(arr: T[], id: string) => arr.find((x) => x.id === id) as T
  const prov = (id: string) => byId(providers, id)
  const seeds: RunSeed[] = [
    {
      id: 'run-live-1',
      name: 'pg-17 Patroni TPC-C 64vu #42',
      test: byId(ts, 't-pg-ha-tpcc'),
      db: byId(dbs, 'db-pg-ha'),
      wl: byId(wls, 'wl-tpcc-64'),
      status: 'running',
      phase: 'workload',
      startedMinAgo: 9,
      durationMin: 18,
      author: max,
      provider: prov('prov-yc-main'),
      labels: { branch: 'main', ci: 'nightly' },
    },
    {
      id: 'run-live-2',
      name: 'mysql 8.4 TPC-C #7',
      test: byId(ts, 't-mysql'),
      db: byId(dbs, 'db-mysql-84'),
      wl: byId(wls, 'wl-mysql-tpcc'),
      status: 'running',
      phase: 'provisioning',
      startedMinAgo: 1,
      durationMin: 12,
      author: lena,
      provider: prov('prov-yc-main'),
    },
    {
      id: 'run-live-3',
      name: 'ydb mirror-3-dc KV #3',
      test: byId(ts, 't-ydb'),
      db: byId(dbs, 'db-ydb-3dc'),
      wl: byId(wls, 'wl-simple-ydb'),
      status: 'running',
      phase: 'deploying',
      startedMinAgo: 4,
      durationMin: 25,
      author: max,
      provider: prov('prov-yc-main'),
      trigger: 'suite',
      trigger_ref: { suite_run_id: 'sr-2', cell_id: 'c-ydb-m' },
    },
    {
      id: 'run-pending-1',
      name: 'pg-17 smoke #131',
      test: byId(ts, 't-pg-smoke'),
      db: byId(dbs, 'db-pg-single'),
      wl: byId(wls, 'wl-tpcb-quick'),
      status: 'pending',
      phase: 'queued',
      startedMinAgo: 0,
      durationMin: 5,
      author: admin,
      provider: prov('prov-yc-main'),
      trigger: 'schedule',
      trigger_ref: { schedule_id: 'sch-smoke' },
    },
    {
      id: 'run-cancelling',
      name: 'OLTP→OLAP split #5',
      test: byId(ts, 't-split'),
      db: byId(dbs, 'db-pg-ha'),
      wl: byId(wls, 'wl-split'),
      status: 'cancelling',
      phase: 'teardown',
      startedMinAgo: 30,
      durationMin: 32,
      author: max,
      provider: prov('prov-aws-eu'),
    },
    {
      id: 'run-kept',
      name: 'pg-17 Patroni TPC-C 64vu #41',
      test: byId(ts, 't-pg-ha-tpcc'),
      db: byId(dbs, 'db-pg-ha'),
      wl: byId(wls, 'wl-tpcc-64'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 95,
      durationMin: 19,
      author: max,
      provider: prov('prov-yc-main'),
      tps: 3120,
      p99: 48,
      errors: 2,
      keep: true,
      favorite: true,
      labels: { branch: 'main' },
      notes: '## Baseline for 17.2\n\nsync replica on, `shared_buffers=8GB`. Compare against #38.',
    },
    {
      id: 'run-c1',
      name: 'pg-17 smoke #130',
      test: byId(ts, 't-pg-smoke'),
      db: byId(dbs, 'db-pg-single'),
      wl: byId(wls, 'wl-tpcb-quick'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 6,
      durationMin: 5,
      author: admin,
      provider: prov('prov-yc-main'),
      trigger: 'schedule',
      trigger_ref: { schedule_id: 'sch-smoke' },
      tps: 1450,
      p99: 22,
      errors: 0,
    },
    {
      id: 'run-f1',
      name: 'pg-18 replica TPC-C #2',
      test: byId(ts, 't-pg-18'),
      db: byId(dbs, 'db-pg-18'),
      wl: byId(wls, 'wl-tpcc-64'),
      status: 'failed',
      phase: 'deploying',
      startedMinAgo: 60 * 8,
      durationMin: 7,
      author: olga,
      provider: prov('prov-yc-main'),
      reason:
        'container master-1/postgres failed healthcheck: FATAL: could not load library "vector.so"',
      labels: { branch: 'feature/pg18' },
    },
    {
      id: 'run-c2',
      name: 'mysql 8.4 TPC-C #6',
      test: byId(ts, 't-mysql'),
      db: byId(dbs, 'db-mysql-84'),
      wl: byId(wls, 'wl-mysql-tpcc'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 20,
      durationMin: 11,
      author: lena,
      provider: prov('prov-yc-main'),
      tps: 2210,
      p99: 61,
      errors: 14,
    },
    {
      id: 'run-x1',
      name: 'ydb mirror-3-dc KV #2',
      test: byId(ts, 't-ydb'),
      db: byId(dbs, 'db-ydb-3dc'),
      wl: byId(wls, 'wl-simple-ydb'),
      status: 'cancelled',
      phase: 'workload',
      startedMinAgo: 60 * 26,
      durationMin: 14,
      author: max,
      provider: prov('prov-yc-main'),
      reason: 'cancelled by Max Ivanov',
    },
    {
      id: 'run-c3',
      name: 'OLTP→OLAP split #4',
      test: byId(ts, 't-split'),
      db: byId(dbs, 'db-pg-ha'),
      wl: byId(wls, 'wl-split'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 30,
      durationMin: 34,
      author: max,
      provider: prov('prov-aws-eu'),
      tps: 2890,
      p99: 55,
      errors: 0,
      favorite: true,
    },
    {
      id: 'run-c4',
      name: 'pg-17 Patroni TPC-C 64vu #40',
      test: byId(ts, 't-pg-ha-tpcc'),
      db: byId(dbs, 'db-pg-ha'),
      wl: byId(wls, 'wl-tpcc-64'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 50,
      durationMin: 19,
      author: max,
      provider: prov('prov-yc-main'),
      tps: 3080,
      p99: 51,
      errors: 1,
      trigger: 'api',
    },
    {
      id: 'run-f2',
      name: 'cockroach 3n TPC-C #1',
      test: byId(ts, 't-crdb'),
      db: byId(dbs, 'db-crdb-3'),
      wl: byId(wls, 'wl-crdb'),
      status: 'failed',
      phase: 'provisioning',
      startedMinAgo: 60 * 52,
      durationMin: 4,
      author: lena,
      provider: prov('prov-yc-main'),
      reason: 'quota exceeded: compute.cores (512/512) in folder b1gy…',
    },
    {
      id: 'run-c5',
      name: 'pg-17 smoke #129',
      test: byId(ts, 't-pg-smoke'),
      db: byId(dbs, 'db-pg-single'),
      wl: byId(wls, 'wl-tpcb-quick'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 30,
      durationMin: 5,
      author: admin,
      provider: prov('prov-yc-main'),
      trigger: 'schedule',
      trigger_ref: { schedule_id: 'sch-smoke' },
      tps: 1432,
      p99: 23,
      errors: 0,
    },
    {
      id: 'run-c6',
      name: 'pg-17 smoke #128',
      test: byId(ts, 't-pg-smoke'),
      db: byId(dbs, 'db-pg-single'),
      wl: byId(wls, 'wl-tpcb-quick'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 54,
      durationMin: 5,
      author: admin,
      provider: prov('prov-yc-main'),
      trigger: 'schedule',
      trigger_ref: { schedule_id: 'sch-smoke' },
      tps: 1398,
      p99: 25,
      errors: 0,
    },
    {
      id: 'run-f3',
      name: 'pg-17 smoke #127',
      test: byId(ts, 't-pg-smoke'),
      db: byId(dbs, 'db-pg-single'),
      wl: byId(wls, 'wl-tpcb-quick'),
      status: 'failed',
      phase: 'workload',
      startedMinAgo: 60 * 78,
      durationMin: 3,
      author: admin,
      provider: prov('prov-yc-main'),
      trigger: 'schedule',
      trigger_ref: { schedule_id: 'sch-smoke' },
      reason: 'threshold violated: p99 312ms > 200ms',
    },
    {
      id: 'run-c7',
      name: 'mysql 8.4 TPC-C #5',
      test: byId(ts, 't-mysql'),
      db: byId(dbs, 'db-mysql-84'),
      wl: byId(wls, 'wl-mysql-tpcc'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 70,
      durationMin: 11,
      author: lena,
      provider: prov('prov-yc-main'),
      tps: 2150,
      p99: 64,
      errors: 20,
      trigger: 'suite',
      trigger_ref: { suite_run_id: 'sr-1', cell_id: 'c-mysql-m' },
    },
    {
      id: 'run-c8',
      name: 'mysql 8.4 TPC-C L #5',
      test: byId(ts, 't-mysql'),
      db: byId(dbs, 'db-mysql-84'),
      wl: byId(wls, 'wl-mysql-tpcc'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 70,
      durationMin: 11,
      author: lena,
      provider: prov('prov-yc-main'),
      tps: 3940,
      p99: 39,
      errors: 3,
      trigger: 'suite',
      trigger_ref: { suite_run_id: 'sr-1', cell_id: 'c-mysql-l' },
    },
    {
      id: 'run-c9',
      name: 'pg-17 Patroni TPC-C 64vu #39',
      test: byId(ts, 't-pg-ha-tpcc'),
      db: byId(dbs, 'db-pg-ha'),
      wl: byId(wls, 'wl-tpcc-64'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 100,
      durationMin: 20,
      author: olga,
      provider: prov('prov-yc-main'),
      tps: 2990,
      p99: 57,
      errors: 0,
    },
    {
      id: 'run-c10',
      name: 'pg-17 Patroni TPC-C 64vu #38',
      test: byId(ts, 't-pg-ha-tpcc'),
      db: byId(dbs, 'db-pg-ha'),
      wl: byId(wls, 'wl-tpcc-64'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 150,
      durationMin: 20,
      author: olga,
      provider: prov('prov-yc-main'),
      tps: 2750,
      p99: 66,
      errors: 4,
      notes: 'shared_buffers=4GB — slower.',
    },
    {
      id: 'run-c11',
      name: 'orioledb-17 TPC-C #1',
      test: byId(ts, 't-pg-smoke'),
      db: byId(dbs, 'db-oriole'),
      wl: byId(wls, 'wl-tpcc-64'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 12,
      durationMin: 12,
      author: max,
      provider: prov('prov-yc-main'),
      tps: 4410,
      p99: 33,
      errors: 0,
      favorite: true,
      labels: { experiment: 'orioledb' },
    },
    {
      id: 'run-c12',
      name: 'picodata KV #1',
      test: byId(ts, 't-picodata'),
      db: byId(dbs, 'db-picodata'),
      wl: byId(wls, 'wl-simple-ydb'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 200,
      durationMin: 12,
      author: olga,
      provider: prov('prov-yc-main'),
      tps: 18200,
      p99: 9,
      errors: 0,
    },
    {
      id: 'run-x2',
      name: 'OLTP→OLAP split #3',
      test: byId(ts, 't-split'),
      db: byId(dbs, 'db-pg-ha'),
      wl: byId(wls, 'wl-split'),
      status: 'cancelled',
      phase: 'provisioning',
      startedMinAgo: 60 * 210,
      durationMin: 2,
      author: max,
      provider: prov('prov-aws-eu'),
      reason: 'cancelled by Max Ivanov',
    },
    {
      id: 'run-c13',
      name: 'ydb mirror-3-dc KV #1',
      test: byId(ts, 't-ydb'),
      db: byId(dbs, 'db-ydb-3dc'),
      wl: byId(wls, 'wl-simple-ydb'),
      status: 'completed',
      phase: 'done',
      startedMinAgo: 60 * 240,
      durationMin: 26,
      author: max,
      provider: prov('prov-yc-main'),
      tps: 21400,
      p99: 12,
      errors: 0,
      favorite: true,
    },
  ]
  // A month of repeatable benchmark history fills the dashboard, comparisons and test trends.
  const historyRand = rng(20260929)
  const baselines = seeds.filter((r) => r.status === 'completed')
  for (let i = 0; i < 48; i++) {
    const baseline = baselines[i % baselines.length]
    const factor = 0.88 + historyRand() * 0.2
    seeds.push({
      ...baseline,
      id: `run-history-${i + 1}`,
      name: `${baseline.test.name} #${200 + i}`,
      startedMinAgo: (i + 1) * 11 * 60,
      tps: Math.round((baseline.tps ?? 1500) * factor),
      p99: Math.round((baseline.p99 ?? 40) / factor),
      errors: i % 13 === 0 ? 2 : 0,
      keep: false,
      favorite: i % 11 === 0,
      trigger: 'api',
      trigger_ref: undefined,
      author: users[i % users.length].ref,
      labels: {
        branch: 'main',
        environment: 'benchmark',
        revision: `b${(0x8ae120 + i).toString(16)}`,
      },
      notes: 'Repeatable baseline · dedicated runners · warm cache · 64 virtual users.',
    })
  }
  const runs = seeds.map(buildRun)
  const runLive: Record<string, RunLive> = {}
  runs.forEach((r, i) => {
    const live = buildLive(r, 1000 + i)
    runLive[r.id] = live
    live.metrics.queries_per_second = (live.metrics.tps ?? []).map(([t, v]) => [t, v * 4])
    if (r.summary && r.status === 'completed') {
      r.summary.qps_series = live.metrics.queries_per_second
        .filter((_, n) => n % 4 === 0)
        .map(([t, v]) => ({ t, v }))
    }
  })

  for (const [index, test] of ts.entries()) {
    const history = runs
      .filter((r) => r.test_ref?.id === test.id)
      .sort((a, b) => b.created_at.localeCompare(a.created_at))
    const latest = history[0]
    ts[index] = {
      ...test,
      summary: {
        ...test.summary,
        run_count: history.length,
        last_run: latest
          ? {
              id: latest.id,
              name: latest.name,
              status: latest.status,
              started_at: latest.started_at,
            }
          : undefined,
      },
    }
  }

  const suites: Suite[] = [
    {
      ...entity('suite-mysql', 'MySQL sizes matrix', lena, 22, {
        description: 'mysql 8.4 TPC-C across S/M/L.',
        tags: { team: 'mysql' },
      }),
      tests: [{ ref: { id: 't-mysql', name: 'mysql 8.4 TPC-C' } }],
      axes: {
        sizes: [sizesFor('S', 'S'), sizesFor('M', 'S'), sizesFor('L', 'M')],
        provider_profiles: ['prov-yc-main'],
      },
      cells: [
        {
          id: 'c-mysql-s',
          name: 'mysql / S',
          test: { id: 't-mysql' },
          enabled: true,
          generated: true,
          axis: { sizes: sizesFor('S', 'S'), provider_profile_id: 'prov-yc-main' },
        },
        {
          id: 'c-mysql-m',
          name: 'mysql / M',
          test: { id: 't-mysql' },
          enabled: true,
          generated: true,
          axis: { sizes: sizesFor('M', 'S'), provider_profile_id: 'prov-yc-main' },
        },
        {
          id: 'c-mysql-l',
          name: 'mysql / L',
          test: { id: 't-mysql' },
          enabled: true,
          generated: true,
          axis: { sizes: sizesFor('L', 'M'), provider_profile_id: 'prov-yc-main' },
        },
      ],
      concurrency: 2,
      defaults: { rating: { tenant: true }, keep: '0s' },
      summary: {
        cell_count: 3,
        enabled_cell_count: 3,
        run_count: 1,
        last_run: { id: 'sr-1', name: 'MySQL sizes matrix #1' },
      },
    },
    {
      ...entity('suite-nightly', 'Nightly regression', max, 40, {
        description: 'All ready tests, every night.',
        tags: { ci: 'nightly' },
        is_favorite: true,
      }),
      tests: [
        { ref: { id: 't-pg-ha-tpcc' } },
        { ref: { id: 't-ydb' } },
        { ref: { id: 't-mysql' } },
        { ref: { id: 't-pg-smoke' } },
      ],
      axes: { provider_profiles: ['prov-yc-main'] },
      cells: [
        {
          id: 'c-pg-ha',
          name: 'pg-17 Patroni',
          test: { id: 't-pg-ha-tpcc' },
          enabled: true,
          generated: true,
        },
        {
          id: 'c-ydb-m',
          name: 'ydb mirror-3-dc',
          test: { id: 't-ydb' },
          enabled: true,
          generated: true,
        },
        {
          id: 'c-mysql',
          name: 'mysql 8.4',
          test: { id: 't-mysql' },
          enabled: true,
          generated: true,
        },
        {
          id: 'c-smoke',
          name: 'pg smoke',
          test: { id: 't-pg-smoke' },
          enabled: false,
          generated: true,
        },
      ],
      concurrency: 3,
      defaults: { rating: { tenant: true, global: false } },
      summary: {
        cell_count: 4,
        enabled_cell_count: 3,
        run_count: 1,
        last_run: { id: 'sr-2', name: 'Nightly regression #12' },
        schedules: [{ id: 'sch-nightly', name: 'nightly 02:00' }],
      },
    },
    {
      ...entity('suite-pgver', 'PostgreSQL versions', olga, 6, {
        description: '15 → 18 on the HA topology.',
      }),
      tests: [{ ref: { id: 't-pg-ha-tpcc' } }],
      axes: { database_versions: ['15', '16', '17', '18'], provider_profiles: ['prov-yc-main'] },
      cells: ['15', '16', '17', '18'].map((v) => ({
        id: `c-pg-${v}`,
        name: `pg ${v}`,
        test: { id: 't-pg-ha-tpcc' },
        enabled: true,
        generated: true,
        axis: { database_version: v, provider_profile_id: 'prov-yc-main' },
      })),
      concurrency: 2,
      summary: { cell_count: 4, enabled_cell_count: 4, run_count: 0 },
    },
  ]
  const suiteRuns: SuiteRun[] = [
    {
      id: 'sr-1',
      name: 'MySQL sizes matrix #1',
      suite: { id: 'suite-mysql', name: 'MySQL sizes matrix' },
      status: 'completed',
      trigger: 'manual',
      concurrency: 2,
      progress: { total: 3, done: 3, failed: 0, running: 0, pending: 0, pct: 100 },
      cells: [
        {
          cell_id: 'c-mysql-s',
          name: 'mysql / S',
          run: { id: 'run-c2', name: 'mysql 8.4 TPC-C #6' },
          status: 'completed',
          summary: runs.find((r) => r.id === 'run-c2')?.summary,
        },
        {
          cell_id: 'c-mysql-m',
          name: 'mysql / M',
          run: { id: 'run-c7', name: 'mysql 8.4 TPC-C #5' },
          status: 'completed',
          summary: runs.find((r) => r.id === 'run-c7')?.summary,
        },
        {
          cell_id: 'c-mysql-l',
          name: 'mysql / L',
          run: { id: 'run-c8', name: 'mysql 8.4 TPC-C L #5' },
          status: 'completed',
          summary: runs.find((r) => r.id === 'run-c8')?.summary,
        },
      ],
      author: lena,
      created_at: hoursAgo(71),
      started_at: hoursAgo(71),
      finished_at: hoursAgo(70),
      duration: '58m12s',
    },
    {
      id: 'sr-2',
      name: 'Nightly regression #12',
      suite: { id: 'suite-nightly', name: 'Nightly regression' },
      status: 'running',
      trigger: 'schedule',
      trigger_ref: { schedule_id: 'sch-nightly' },
      concurrency: 3,
      progress: { total: 3, done: 1, failed: 0, running: 1, pending: 1, pct: 33 },
      cells: [
        {
          cell_id: 'c-pg-ha',
          name: 'pg-17 Patroni',
          run: { id: 'run-kept', name: 'pg-17 Patroni TPC-C 64vu #41' },
          status: 'completed',
          summary: runs.find((r) => r.id === 'run-kept')?.summary,
        },
        {
          cell_id: 'c-ydb-m',
          name: 'ydb mirror-3-dc',
          run: { id: 'run-live-3', name: 'ydb mirror-3-dc KV #3' },
          status: 'running',
        },
        { cell_id: 'c-mysql', name: 'mysql 8.4', status: 'pending' },
      ],
      author: admin,
      created_at: minutesAgo(100),
      started_at: minutesAgo(100),
      finished_at: null,
      duration: null,
    },
  ]
  const schedules: Schedule[] = [
    {
      id: 'sch-smoke',
      name: 'pg smoke every 6h',
      target: { kind: 'test', id: 't-pg-smoke', name: 'pg-17 smoke' },
      cron: '0 */6 * * *',
      timezone: 'Europe/Moscow',
      enabled: true,
      overrides: { provider_profile_id: 'prov-yc-main' },
      next_run_at: iso(now + 3 * 3600_000),
      last_run: {
        kind: 'run',
        id: 'run-c1',
        name: 'pg-17 smoke #130',
        status: 'completed',
        at: hoursAgo(6),
      } as Schedule['last_run'],
      author: admin,
      created_at: daysAgo(30),
      updated_at: daysAgo(3),
    },
    {
      id: 'sch-nightly',
      name: 'nightly 02:00',
      target: { kind: 'suite', id: 'suite-nightly', name: 'Nightly regression' },
      cron: '0 2 * * *',
      timezone: 'Europe/Moscow',
      enabled: true,
      next_run_at: iso(now + 10 * 3600_000),
      last_run: {
        kind: 'suite_run',
        id: 'sr-2',
        name: 'Nightly regression #12',
        status: 'running',
        at: minutesAgo(100),
      } as Schedule['last_run'],
      author: max,
      created_at: daysAgo(40),
    },
    {
      id: 'sch-weekly',
      name: 'weekly full split',
      target: { kind: 'test', id: 't-split', name: 'OLTP→OLAP split' },
      cron: '0 3 * * 6',
      timezone: 'UTC',
      enabled: false,
      overrides: { keep: '4h', provider_profile_id: 'prov-aws-eu' },
      next_run_at: null,
      last_run: {
        kind: 'run',
        id: 'run-c3',
        name: 'OLTP→OLAP split #4',
        status: 'completed',
        at: hoursAgo(30),
      } as Schedule['last_run'],
      author: max,
      created_at: daysAgo(20),
    },
  ]
  const shares: Share[] = [
    {
      id: 'share-1',
      token: 'shr_9f2k1a',
      url: '/s/shr_9f2k1a',
      target: { kind: 'run', id: 'run-kept', name: 'pg-17 Patroni TPC-C 64vu #41' },
      scope: 'metrics',
      title: 'PG 17 HA baseline',
      active: true,
      expires_at: iso(now + 6 * 86400_000),
      captured_at: hoursAgo(1),
      view_count: 17,
      created_by: max,
      created_at: hoursAgo(1),
    },
    {
      id: 'share-2',
      token: 'shr_cmp77',
      url: '/s/shr_cmp77',
      target: { kind: 'comparison', id: 'cmp-1', run_ids: ['run-kept', 'run-c10', 'run-c11'] },
      scope: 'configs',
      title: 'shared_buffers 4G vs 8G vs OrioleDB',
      active: true,
      expires_at: null,
      captured_at: daysAgo(1),
      view_count: 4,
      created_by: olga,
      created_at: daysAgo(1),
    },
    {
      id: 'share-3',
      token: 'shr_old01',
      url: '/s/shr_old01',
      target: { kind: 'suite_run', id: 'sr-1', name: 'MySQL sizes matrix #1' },
      scope: 'overview',
      active: false,
      revoked_at: daysAgo(2),
      captured_at: daysAgo(3),
      view_count: 2,
      created_by: lena,
      created_at: daysAgo(3),
    },
  ]
  const members: Member[] = [
    { user: { ...admin, email: 'alex@stroppy.example' }, role: 'owner', joined_at: daysAgo(90) },
    { user: { ...max, email: 'max@example.com' }, role: 'admin', joined_at: daysAgo(80) },
    { user: { ...olga, email: 'olga@example.com' }, role: 'member', joined_at: daysAgo(60) },
    { user: { ...lena, email: 'lena@example.com' }, role: 'member', joined_at: daysAgo(30) },
  ]
  const invites: Invite[] = [
    {
      id: 'inv-1',
      tenant,
      email: 'newbie@example.com',
      role: 'viewer',
      status: 'pending',
      invited_by: max,
      created_at: daysAgo(2),
      expires_at: iso(now + 5 * 86400_000),
    },
    {
      id: 'inv-2',
      tenant,
      email: 'gone@example.com',
      role: 'member',
      status: 'expired',
      invited_by: admin,
      created_at: daysAgo(20),
      expires_at: daysAgo(13),
    },
  ]
  const tokens: ApiToken[] = [
    {
      id: 'tok-ci',
      name: 'gitlab-ci',
      prefix: 'stc_ci7q',
      kind: 'service',
      tenant: { id: tenant.id, name: tenant.name },
      role: 'member',
      expires_at: iso(now + 200 * 86400_000),
      last_used_at: minutesAgo(35),
      created_at: daysAgo(20),
    },
    {
      id: 'tok-ro',
      name: 'grafana-readonly',
      prefix: 'stc_ro12',
      kind: 'service',
      tenant: { id: tenant.id, name: tenant.name },
      role: 'viewer',
      expires_at: null,
      last_used_at: daysAgo(1),
      created_at: daysAgo(50),
    },
  ]
  const { webhooks, deliveries } = webhooksFor()
  const audit: AuditEntry[] = [
    {
      id: 'a1',
      at: minutesAgo(9),
      actor: { kind: 'user', id: max.id, display_name: max.display_name },
      tenant: { id: tenant.id, name: tenant.name },
      action: 'run.launch',
      target: { kind: 'run', id: 'run-live-1', name: 'pg-17 Patroni TPC-C 64vu #42' },
    },
    {
      id: 'a2',
      at: minutesAgo(35),
      actor: { kind: 'token', id: 'tok-ci', display_name: 'gitlab-ci' },
      tenant: { id: tenant.id, name: tenant.name },
      action: 'run.launch',
      target: { kind: 'run', id: 'run-c4', name: 'pg-17 Patroni TPC-C 64vu #40' },
      details: { via: 'api' },
    },
    {
      id: 'a3',
      at: hoursAgo(1),
      actor: { kind: 'user', id: max.id, display_name: max.display_name },
      tenant: { id: tenant.id, name: tenant.name },
      action: 'share.create',
      target: { kind: 'share', id: 'share-1', name: 'PG 17 HA baseline' },
    },
    {
      id: 'a4',
      at: daysAgo(2),
      actor: { kind: 'user', id: max.id, display_name: max.display_name },
      tenant: { id: tenant.id, name: tenant.name },
      action: 'member.invite',
      target: { kind: 'invite', id: 'inv-1', name: 'newbie@example.com' },
      details: { role: 'viewer' },
    },
    {
      id: 'a5',
      at: daysAgo(3),
      actor: { kind: 'user', id: admin.id, display_name: admin.display_name },
      tenant: { id: tenant.id, name: tenant.name },
      action: 'settings.update',
      target: { kind: 'tenant', id: tenant.id },
      details: { run_retention_days: { from: 30, to: 60 } },
    },
    {
      id: 'a6',
      at: daysAgo(12),
      actor: { kind: 'user', id: olga.id, display_name: olga.display_name },
      tenant: { id: tenant.id, name: tenant.name },
      action: 'provider.create',
      target: { kind: 'provider', id: 'prov-aws-eu', name: 'aws-eu-central' },
    },
    {
      id: 'a7',
      at: daysAgo(26),
      actor: { kind: 'user', id: max.id, display_name: max.display_name },
      tenant: { id: tenant.id, name: tenant.name },
      action: 'run.cancel',
      target: { kind: 'run', id: 'run-x1', name: 'ydb mirror-3-dc KV #2' },
    },
  ]
  const favorites = new Set<string>()
  for (const r of runs) if (r.is_favorite) favorites.add(`run:${r.id}`)
  for (const d of dbs) if (d.is_favorite) favorites.add(`database:${d.id}`)
  for (const w of wls) if (w.is_favorite) favorites.add(`workload:${w.id}`)
  for (const t of ts) if (t.is_favorite) favorites.add(`test:${t.id}`)
  for (const s of suites) if (s.is_favorite) favorites.add(`suite:${s.id}`)
  return {
    tenant,
    members,
    invites,
    tokens,
    settings: {
      run_retention_days: 60,
      default_rating: { tenant: true, global: false },
      default_keep: '0s',
      notification_emails: ['bench-alerts@example.com'],
    },
    limits: {
      max_concurrent_runs: 5,
      max_machines_per_run: 24,
      max_size: 'XL',
      max_keep: '12h',
      run_retention_max_days: 180,
      source: 'tenant_override',
    },
    providers,
    quotas: quotasFor(),
    webhooks,
    deliveries,
    databases: dbs,
    workloads: wls,
    tests: ts,
    runs,
    runLive,
    suites,
    suiteRuns,
    schedules,
    shares,
    audit,
    favorites,
  }
}

function tenantSandbox(): TenantData {
  const tenant: Tenant = {
    id: 'ten-sandbox',
    slug: 'sandbox',
    name: 'Olga sandbox',
    status: 'active',
    owner: olga,
    member_count: 2,
    created_at: daysAgo(15),
  }
  const providers: ProviderProfile[] = [
    {
      id: 'prov-sb-yc',
      name: 'yc-sandbox',
      kind: 'yandex',
      status: 'verifying',
      settings: { cloud_id: 'b1gsandbox', folder_id: 'b1gsbfolder', zone: 'ru-central1-d' },
      created_at: minutesAgo(3),
      created_by: olga,
    },
  ]
  const dbs = databases()
    .slice(0, 1)
    .map((d) => ({ ...d, id: 'sb-db-1', author: olga }))
  const wls = workloads()
    .slice(1, 2)
    .map((w) => ({ ...w, id: 'sb-wl-1', author: olga }))
  return {
    tenant,
    members: [
      { user: { ...olga, email: 'olga@example.com' }, role: 'owner', joined_at: daysAgo(15) },
      { user: { ...admin, email: 'alex@stroppy.example' }, role: 'viewer', joined_at: daysAgo(14) },
    ],
    invites: [],
    tokens: [],
    settings: {
      run_retention_days: 30,
      default_rating: { tenant: false, global: false },
      default_keep: '0s',
    },
    limits: {
      max_concurrent_runs: 2,
      max_machines_per_run: 8,
      max_size: 'M',
      max_keep: '2h',
      run_retention_max_days: 90,
      source: 'platform_default',
    },
    providers,
    quotas: {
      'prov-sb-yc': {
        observed_at: null,
        stale: true,
        unavailable_reason: 'verification in progress',
        quotas: [],
      },
    },
    webhooks: [],
    deliveries: {},
    databases: dbs,
    workloads: wls,
    tests: [],
    runs: [],
    runLive: {},
    suites: [],
    suiteRuns: [],
    schedules: [],
    shares: [],
    audit: [],
    favorites: new Set(),
  }
}

// Brings seeded non-terminal runs to the state the simulation would have them in now: workload
// segments by their plan, `expected_finish_at`, live headline and QPS sparkline.
function syncSeededLive(t: TenantData): void {
  for (const run of t.runs) {
    const live = t.runLive[run.id]
    if (!live || !['running', 'cancelling'].includes(run.status)) continue
    // Before the workload nothing has been measured yet; the simulation samples from its start.
    if (run.phase === 'provisioning' || run.phase === 'deploying') live.metrics = {}
    if (run.phase === 'workload') progressSegments(run, live, now, segmentBudgets(run, 90_000))
    else if (run.phase === 'collecting' || run.phase === 'teardown') {
      const ended = live.overview.phases.find((p) => p.id === 'workload')?.finished_at
      for (const s of live.overview.workload_segments ?? []) {
        s.status = 'completed'
        s.finished_at = ended ?? null
      }
    }
    syncLiveSummary(run, live)
  }
}

export function seed(): Omit<MockStore, 'tenant' | 'roleIn' | 'audit'> {
  const main = tenantMain()
  expandCapacityDemo(main, buildRun, buildLive)
  const sandbox = tenantSandbox()
  for (const t of [main, sandbox]) syncSeededLive(t)
  const me: Me = {
    id: ADMIN_ID,
    email: 'alex@stroppy.example',
    display_name: 'Alex Morgan',
    avatar: 'identicon',
    is_platform_admin: true,
    tenants: [
      { tenant: main.tenant, role: 'owner', joined_at: daysAgo(90) },
      { tenant: sandbox.tenant, role: 'viewer', joined_at: daysAgo(14) },
    ],
    owned_tenant_id: main.tenant.id,
    preferences: { theme: 'system', timezone: 'Europe/Moscow', default_tenant: 'main' },
    notifications: { run_finished: true, run_failed: true, suite_finished: false },
    created_at: daysAgo(120),
  }
  return {
    version: '0.3.0-mock',
    me,
    users: users.map(
      (u) =>
        ({
          id: u.ref.id,
          email: u.email,
          display_name: u.ref.display_name ?? u.email,
          is_platform_admin: u.admin,
          admin_source: u.admin ? 'config' : undefined,
          owned_tenant:
            u.ref.id === ADMIN_ID
              ? { id: main.tenant.id, name: main.tenant.name }
              : u.ref.id === olga.id
                ? { id: sandbox.tenant.id, name: sandbox.tenant.name }
                : undefined,
          created_at: daysAgo(100),
          last_seen_at: hoursAgo(2),
        }) as Schemas['AdminUser']
    ),
    tenants: { main, sandbox },
    catalog: {
      databases: catalogDatabases(),
      providers: catalogProviders(),
      stroppy: stroppyCatalog(),
      examples: examples(),
      metrics: metricDefs(),
    },
    system: {
      tenant_creation: 'anyone',
      public_rating_enabled: true,
      examples_enabled: true,
      default_limits: {
        max_concurrent_runs: 2,
        max_machines_per_run: 8,
        max_size: 'M',
        max_keep: '2h',
        run_retention_max_days: 90,
      },
      run_retention_max_days: 365,
      stroppy_catalog: {},
      updated_at: daysAgo(3),
      updated_by: admin,
    },
    adminAudit: [...main.audit],
    myInvites: [
      {
        id: 'inv-me-1',
        tenant: {
          id: 'ten-acme',
          slug: 'acme',
          name: 'ACME perf lab',
          status: 'active',
          owner: lena,
          created_at: daysAgo(50),
        },
        email: 'alex@stroppy.example',
        role: 'member',
        status: 'pending',
        invited_by: lena,
        message: 'Join us for the Q4 comparison',
        created_at: daysAgo(1),
        expires_at: iso(now + 6 * 86400_000),
      },
    ],
    personalTokens: [
      {
        id: 'tok-me-1',
        name: 'laptop cli',
        prefix: 'stc_lp9x',
        kind: 'personal',
        tenant: { id: main.tenant.id, name: main.tenant.name },
        role: 'owner',
        owner: admin,
        expires_at: iso(now + 90 * 86400_000),
        last_used_at: hoursAgo(3),
        created_at: daysAgo(10),
      },
    ],
  }
}

export { uuid }

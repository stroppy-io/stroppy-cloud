import type {
  ApiToken,
  AuditEntry,
  CatalogDatabase,
  CatalogProvider,
  Example,
  Invite,
  LogLine,
  Me,
  Member,
  MetricDef,
  ProviderProfile,
  QuotaReport,
  Run,
  RunEvent,
  RunOverview,
  Schedule,
  Schemas,
  Share,
  StroppyCatalog,
  SuiteRun,
  SystemSettings,
  Tenant,
  TenantLimits,
  TenantRole,
  TenantSettings,
  Webhook,
  WebhookDelivery,
} from '@api/types'
import { seed } from './seed'

export type Database = Schemas['Database']
export type Workload = Schemas['Workload']
export type Test = Schemas['Test']
export type Suite = Schemas['Suite']

// Live per-run data produced by the simulation.
export interface RunLive {
  overview: RunOverview
  events: RunEvent[]
  logs: LogLine[]
  // key → [ts, value][]
  metrics: Record<string, number[][]>
  // simulation bookkeeping
  phaseIndex: number
  phaseStartedAt: number
  segmentIndex: number
  seq: number
}

export interface TenantData {
  tenant: Tenant
  members: Member[]
  invites: Invite[]
  tokens: ApiToken[]
  settings: TenantSettings
  limits: TenantLimits
  providers: ProviderProfile[]
  quotas: Record<string, QuotaReport>
  webhooks: Webhook[]
  deliveries: Record<string, WebhookDelivery[]>
  databases: Database[]
  workloads: Workload[]
  tests: Test[]
  runs: Run[]
  runLive: Record<string, RunLive>
  suites: Suite[]
  suiteRuns: SuiteRun[]
  schedules: Schedule[]
  shares: Share[]
  audit: AuditEntry[]
  favorites: Set<string> // `${kind}:${id}`
}

export interface MockStore {
  version: string
  me: Me
  users: Schemas['AdminUser'][]
  tenants: Record<string, TenantData>
  catalog: {
    databases: CatalogDatabase[]
    providers: CatalogProvider[]
    stroppy: StroppyCatalog
    examples: Example[]
    metrics: MetricDef[]
  }
  system: SystemSettings
  adminAudit: AuditEntry[]
  myInvites: Invite[]
  personalTokens: ApiToken[]
  // helpers
  tenant(slug: string): TenantData | undefined
  roleIn(slug: string): TenantRole | undefined
  audit(
    slug: string | undefined,
    action: string,
    target: AuditEntry['target'],
    details?: Record<string, unknown>
  ): void
}

function createStore(): MockStore {
  const data = seed()
  const store: MockStore = {
    ...data,
    tenant(slug) {
      return this.tenants[slug]
    },
    roleIn(slug) {
      return this.me.tenants.find((m) => m.tenant.slug === slug)?.role
    },
    audit(slug, action, target, details) {
      const entry: AuditEntry = {
        id: crypto.randomUUID(),
        at: new Date().toISOString(),
        actor: { kind: 'user', id: this.me.id, display_name: this.me.display_name },
        tenant: slug
          ? { id: this.tenants[slug]?.tenant.id ?? slug, name: this.tenants[slug]?.tenant.name }
          : undefined,
        action,
        target,
        details,
      }
      if (slug) this.tenants[slug]?.audit.unshift(entry)
      this.adminAudit.unshift(entry)
    },
  }
  return store
}

export const store: MockStore = createStore()

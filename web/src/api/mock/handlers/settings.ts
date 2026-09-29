import type {
  ProviderProfile,
  QuotaReport,
  TenantLimits,
  TenantSettings,
  Webhook,
  WebhookDelivery,
} from '@api/types'
import { noContent, notFound, problem, route } from '../router'
import { store as globalStore } from '../store'
import { iso, paginate, parseListQuery, uuid } from '../util'

type SettingsPatch = Partial<TenantSettings>
type ProviderCreate = {
  name: string
  kind: ProviderProfile['kind']
  settings: Record<string, unknown>
  credentials: Record<string, unknown>
}
type ProviderPatch = Partial<ProviderCreate>
type WebhookWrite = Partial<Pick<Webhook, 'url' | 'events' | 'enabled' | 'description'>>

const VERIFY_MS = 4000

// Profiles whose credentials contain the word "bad" fail verification every time.
const badCredentials = new Set<string>(['prov-yc-broken'])

function hasBadCredentials(creds: Record<string, unknown> | undefined): boolean {
  return JSON.stringify(creds ?? {})
    .toLowerCase()
    .includes('bad')
}

function required(path: string, message: string) {
  return problem(400, 'validation_failed', message, {
    validation: { errors: [{ path, code: 'REQUIRED', severity: 'ERROR', message }] },
  })
}

// ---------- settings / limits ----------

route('GET', '/api/v1/t/:slug/settings', ({ store, params }) => {
  const t = store.tenant(params.slug)
  return t ? { json: t.settings } : notFound('tenant')
})

route('PATCH', '/api/v1/t/:slug/settings', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const b = (body ?? {}) as SettingsPatch
  if (b.run_retention_days !== undefined) {
    const max = t.limits.run_retention_max_days
    if (b.run_retention_days < 1 || b.run_retention_days > max)
      return problem(400, 'validation_failed', 'retention out of range', {
        validation: {
          errors: [
            {
              path: 'run_retention_days',
              code: 'RANGE',
              severity: 'ERROR',
              message: `Between 1 and ${max} days (tenant limit)`,
            },
          ],
        },
      })
    t.settings.run_retention_days = b.run_retention_days
  }
  if (b.default_rating)
    t.settings.default_rating = { ...t.settings.default_rating, ...b.default_rating }
  if (b.default_keep !== undefined) {
    if (!/^(\d+h)?(\d+m)?(\d+s)?$/.test(b.default_keep) || b.default_keep === '')
      return problem(400, 'validation_failed', 'bad duration', {
        validation: {
          errors: [
            {
              path: 'default_keep',
              code: 'FORMAT',
              severity: 'ERROR',
              message: 'Use a Go duration like 0s, 30m or 2h',
            },
          ],
        },
      })
    t.settings.default_keep = b.default_keep
  }
  if (b.notification_emails !== undefined) t.settings.notification_emails = b.notification_emails
  store.audit(
    params.slug,
    'settings.update',
    { kind: 'tenant', id: t.tenant.id, name: t.tenant.name },
    b as Record<string, unknown>
  )
  return { json: t.settings }
})

route('GET', '/api/v1/t/:slug/limits', ({ store, params }) => {
  const t = store.tenant(params.slug)
  return t ? { json: t.limits satisfies TenantLimits } : notFound('tenant')
})

// ---------- providers ----------

function scheduleVerification(slug: string, id: string): void {
  window.setTimeout(() => {
    const t = globalStore.tenant(slug)
    const p = t?.providers.find((x) => x.id === id)
    if (!t || !p || p.status !== 'verifying') return
    if (badCredentials.has(id)) {
      p.status = 'failed'
      p.status_reason = 'credentials rejected by the cloud (iam: 401 Unauthorized)'
      p.verified_at = null
      t.quotas[id] = {
        observed_at: null,
        stale: true,
        unavailable_reason: 'profile failed verification',
        quotas: [],
      }
    } else {
      p.status = 'ready'
      p.status_reason = undefined
      p.verified_at = iso()
      p.quotas_observed_at = iso()
      t.quotas[id] = freshQuotas(p)
    }
    globalStore.audit(
      slug,
      'provider.verified',
      { kind: 'provider', id, name: p.name },
      {
        status: p.status,
      }
    )
  }, VERIFY_MS)
}

function freshQuotas(p: ProviderProfile): QuotaReport {
  const scope =
    p.kind === 'yandex'
      ? `folder ${String(p.settings?.folder_id ?? '').slice(0, 6)}…`
      : String(p.settings?.region ?? 'eu-central-1')
  const q = (name: string, title: string, unit: string, limit: number, ratio: number) => ({
    name,
    title,
    unit,
    limit,
    used: Math.round(limit * ratio * 10) / 10,
  })
  return {
    observed_at: iso(),
    stale: false,
    scope,
    quotas:
      p.kind === 'yandex'
        ? [
            q('compute.instances', 'Instances', 'count', 100, 0.12),
            q('compute.cores', 'vCPU', 'count', 512, 0.2),
            q('compute.memory', 'RAM', 'GB', 2048, 0.18),
            q('compute.ssd', 'Network SSD', 'GB', 8192, 0.31),
            q('vpc.addresses', 'Public IPs', 'count', 32, 0.25),
          ]
        : [
            q('ec2.vcpus.standard', 'On-demand vCPU (standard)', 'count', 256, 0.15),
            q('ebs.gp3', 'gp3 storage', 'TB', 50, 0.06),
            q('vpc.eip', 'Elastic IPs', 'count', 5, 0.4),
          ],
  }
}

route('GET', '/api/v1/t/:slug/providers', ({ store, params }) => {
  const t = store.tenant(params.slug)
  return t ? { json: { data: t.providers } } : notFound('tenant')
})

route('POST', '/api/v1/t/:slug/providers', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const b = (body ?? {}) as ProviderCreate
  if (!b.name?.trim()) return required('name', 'Give the profile a name, e.g. yc-benchmarks')
  if (b.kind !== 'yandex' && b.kind !== 'aws')
    return problem(400, 'validation_failed', 'unknown provider kind', {
      validation: {
        errors: [{ path: 'kind', code: 'ENUM', severity: 'ERROR', message: 'Pick yandex or aws' }],
      },
    })
  if (t.providers.some((p) => p.name === b.name.trim()))
    return problem(409, 'name_taken', 'A profile with this name already exists', {
      validation: {
        errors: [
          {
            path: 'name',
            code: 'TAKEN',
            severity: 'ERROR',
            message: 'A profile with this name already exists — pick another',
          },
        ],
      },
    })
  const id = `prov-${uuid().slice(0, 8)}`
  const profile: ProviderProfile = {
    id,
    name: b.name.trim(),
    kind: b.kind,
    status: 'verifying',
    settings: b.settings ?? {},
    secret_names: [`provider-${id}`],
    created_at: iso(),
    created_by: { id: store.me.id, display_name: store.me.display_name },
  }
  if (hasBadCredentials(b.credentials)) badCredentials.add(id)
  t.providers.unshift(profile)
  t.quotas[id] = {
    observed_at: null,
    stale: true,
    unavailable_reason: 'verification in progress',
    quotas: [],
  }
  store.audit(
    params.slug,
    'provider.create',
    { kind: 'provider', id, name: profile.name },
    {
      kind: b.kind,
    }
  )
  scheduleVerification(params.slug, id)
  return { status: 201, json: profile }
})

route('GET', '/api/v1/t/:slug/providers/:id', ({ store, params }) => {
  const p = store.tenant(params.slug)?.providers.find((x) => x.id === params.id)
  return p ? { json: p } : notFound('provider')
})

route('PATCH', '/api/v1/t/:slug/providers/:id', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  const p = t?.providers.find((x) => x.id === params.id)
  if (!t || !p) return notFound('provider')
  const b = (body ?? {}) as ProviderPatch
  if (b.name !== undefined) {
    if (!b.name.trim()) return required('name', 'Name is required')
    p.name = b.name.trim()
  }
  if (b.settings !== undefined) p.settings = b.settings
  if (b.credentials !== undefined) {
    if (hasBadCredentials(b.credentials)) badCredentials.add(p.id)
    else badCredentials.delete(p.id)
    p.status = 'verifying'
    p.status_reason = undefined
    scheduleVerification(params.slug, p.id)
  }
  store.audit(params.slug, 'provider.update', { kind: 'provider', id: p.id, name: p.name })
  return { json: p }
})

route('DELETE', '/api/v1/t/:slug/providers/:id', ({ store, params }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const i = t.providers.findIndex((x) => x.id === params.id)
  if (i < 0) return notFound('provider')
  const p = t.providers[i]
  const inUse = t.runs.some(
    (r) =>
      r.snapshot.provider_profile.id === p.id &&
      (r.status === 'running' || r.status === 'pending' || r.status === 'cancelling')
  )
  if (inUse)
    return problem(
      409,
      'provider_in_use',
      'A live run uses this profile. Wait for it to finish or cancel it first.'
    )
  t.providers.splice(i, 1)
  delete t.quotas[p.id]
  badCredentials.delete(p.id)
  store.audit(params.slug, 'provider.delete', { kind: 'provider', id: p.id, name: p.name })
  return noContent()
})

route('POST', '/api/v1/t/:slug/providers/:id:verify', ({ store, params }) => {
  const t = store.tenant(params.slug)
  const p = t?.providers.find((x) => x.id === params.id)
  if (!t || !p) return notFound('provider')
  p.status = 'verifying'
  p.status_reason = undefined
  store.audit(params.slug, 'provider.verify', { kind: 'provider', id: p.id, name: p.name })
  scheduleVerification(params.slug, p.id)
  return { status: 202, json: p }
})

route('GET', '/api/v1/t/:slug/providers/:id/quotas', ({ store, params }) => {
  const t = store.tenant(params.slug)
  const p = t?.providers.find((x) => x.id === params.id)
  if (!t || !p) return notFound('provider')
  const q = t.quotas[p.id]
  if (!q)
    return {
      json: {
        observed_at: null,
        stale: true,
        unavailable_reason: p.status === 'ready' ? 'not observed yet' : `profile is ${p.status}`,
        quotas: [],
      } satisfies QuotaReport,
    }
  // Freshness window is one minute.
  if (q.observed_at && Date.now() - new Date(q.observed_at).getTime() > 60_000) q.stale = true
  return { json: q }
})

route('POST', '/api/v1/t/:slug/providers/:id/quotas:refresh', ({ store, params }) => {
  const t = store.tenant(params.slug)
  const p = t?.providers.find((x) => x.id === params.id)
  if (!t || !p) return notFound('provider')
  if (p.status !== 'ready')
    return problem(
      409,
      'provider_not_ready',
      `Quotas are read only for ready profiles (now: ${p.status}).`
    )
  const prev = t.quotas[p.id]
  const next: QuotaReport = prev?.quotas.length
    ? {
        ...prev,
        observed_at: iso(),
        stale: false,
        unavailable_reason: undefined,
        quotas: prev.quotas.map((q) => ({
          ...q,
          used: Math.min(
            q.limit,
            Math.max(0, Math.round((q.used + (Math.random() - 0.5) * q.limit * 0.04) * 10) / 10)
          ),
        })),
      }
    : freshQuotas(p)
  t.quotas[p.id] = next
  p.quotas_observed_at = next.observed_at
  return { json: next }
})

// ---------- webhooks ----------

const secret = () => `whsec_${uuid().replace(/-/g, '')}`

function validateWebhook(b: WebhookWrite, partial: boolean) {
  if (!partial || b.url !== undefined) {
    if (!b.url || !/^https:\/\/[^\s/$.?#].[^\s]*$/i.test(b.url))
      return problem(400, 'validation_failed', 'bad url', {
        validation: {
          errors: [
            {
              path: 'url',
              code: 'FORMAT',
              severity: 'ERROR',
              message: 'Enter an https:// URL, e.g. https://ci.example.com/hooks/stroppy',
            },
          ],
        },
      })
  }
  if (!partial || b.events !== undefined) {
    if (!b.events?.length)
      return problem(400, 'validation_failed', 'events required', {
        validation: {
          errors: [
            {
              path: 'events',
              code: 'REQUIRED',
              severity: 'ERROR',
              message: 'Pick at least one event',
            },
          ],
        },
      })
  }
  return undefined
}

route('GET', '/api/v1/t/:slug/webhooks', ({ store, params }) => {
  const t = store.tenant(params.slug)
  return t ? { json: { data: t.webhooks } } : notFound('tenant')
})

route('POST', '/api/v1/t/:slug/webhooks', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const b = (body ?? {}) as WebhookWrite
  const err = validateWebhook(b, false)
  if (err) return err
  const wh: Webhook = {
    id: `wh-${uuid().slice(0, 8)}`,
    url: b.url as string,
    events: b.events ?? [],
    enabled: b.enabled ?? true,
    description: b.description,
    created_at: iso(),
  }
  t.webhooks.unshift(wh)
  t.deliveries[wh.id] = []
  store.audit(params.slug, 'webhook.create', { kind: 'webhook', id: wh.id, name: wh.url })
  return { status: 201, json: { ...wh, secret: secret() } }
})

route('PATCH', '/api/v1/t/:slug/webhooks/:id', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  const wh = t?.webhooks.find((x) => x.id === params.id)
  if (!t || !wh) return notFound('webhook')
  const b = (body ?? {}) as WebhookWrite
  const err = validateWebhook(b, true)
  if (err) return err
  if (b.url !== undefined) wh.url = b.url
  if (b.events !== undefined) wh.events = b.events
  if (b.enabled !== undefined) wh.enabled = b.enabled
  if (b.description !== undefined) wh.description = b.description
  store.audit(params.slug, 'webhook.update', { kind: 'webhook', id: wh.id, name: wh.url }, b)
  return { json: wh }
})

route('DELETE', '/api/v1/t/:slug/webhooks/:id', ({ store, params }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const i = t.webhooks.findIndex((x) => x.id === params.id)
  if (i < 0) return notFound('webhook')
  const [wh] = t.webhooks.splice(i, 1)
  delete t.deliveries[wh.id]
  store.audit(params.slug, 'webhook.delete', { kind: 'webhook', id: wh.id, name: wh.url })
  return noContent()
})

route('POST', '/api/v1/t/:slug/webhooks/:id:rotate-secret', ({ store, params }) => {
  const t = store.tenant(params.slug)
  const wh = t?.webhooks.find((x) => x.id === params.id)
  if (!t || !wh) return notFound('webhook')
  store.audit(params.slug, 'webhook.rotate_secret', { kind: 'webhook', id: wh.id, name: wh.url })
  return { json: { ...wh, secret: secret() } }
})

route('GET', '/api/v1/t/:slug/webhooks/:id/deliveries', ({ store, params, query }) => {
  const t = store.tenant(params.slug)
  const wh = t?.webhooks.find((x) => x.id === params.id)
  if (!t || !wh) return notFound('webhook')
  const items = [...(t.deliveries[wh.id] ?? [])].sort((a, b) =>
    a.created_at < b.created_at ? 1 : -1
  )
  return { json: paginate(items, parseListQuery(query)) }
})

route('POST', '/api/v1/t/:slug/webhooks/:id/deliveries/:deliveryId:replay', ({ store, params }) => {
  const t = store.tenant(params.slug)
  const wh = t?.webhooks.find((x) => x.id === params.id)
  if (!t || !wh) return notFound('webhook')
  const src = (t.deliveries[wh.id] ?? []).find((d) => d.id === params.deliveryId)
  if (!src) return notFound('delivery')
  const replay: WebhookDelivery = {
    id: `d-${uuid().slice(0, 8)}`,
    event: src.event,
    status: 'success',
    attempts: 1,
    last_attempt_at: iso(),
    response_status: 200,
    payload: src.payload,
    created_at: iso(),
  }
  t.deliveries[wh.id] = [replay, ...(t.deliveries[wh.id] ?? [])]
  wh.last_delivery = { at: replay.created_at, status: 'success' }
  return { status: 202, json: replay }
})

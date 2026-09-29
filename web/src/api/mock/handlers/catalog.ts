import type { SchemaInfo } from '@api/types'
import { notFound, problem, route } from '../router'

// schemapb schemas exported by `make schemas-export` (protoJSON) — served as the catalog.
const schemaFiles = import.meta.glob('../../../schemas/*.json', {
  eager: true,
  import: 'default',
}) as Record<string, Record<string, unknown>>

function schemaId(path: string): { id: string; version: string } {
  const base =
    path
      .split('/')
      .pop()
      ?.replace(/\.json$/, '') ?? ''
  // `db.postgres.params_v1` → id db.postgres.params, version 1 ; `cfg.mariadb.cnf_v10.11` → version 10.11
  const m = base.match(/^(.*)_v([\d.]+)$/)
  return m ? { id: m[1], version: m[2] } : { id: base, version: '1' }
}

const schemas: (SchemaInfo & { body: Record<string, unknown> })[] = Object.entries(schemaFiles).map(
  ([path, body]) => {
    const { id, version } = schemaId(path)
    const [namespace, ...rest] = id.split('.')
    return {
      id: `${id}@${version}`,
      version,
      namespace,
      name: rest.join('.'),
      title: (body.title as string) ?? id,
      description: body.description as string | undefined,
      body,
    }
  }
)

export function findSchema(ref: string): (typeof schemas)[number] | undefined {
  // accepts `db.postgres.params`, `db.postgres.params@1`, `cfg.postgresql.conf@17`
  const [id, ver] = ref.split('@')
  const candidates = schemas.filter((s) => s.id.split('@')[0] === id)
  if (!candidates.length) return undefined
  if (ver)
    return (
      candidates.find((s) => s.version === ver) ??
      candidates.find((s) => s.version.startsWith(ver)) ??
      candidates[0]
    )
  return candidates.sort((a, b) => Number.parseFloat(b.version) - Number.parseFloat(a.version))[0]
}

route('GET', '/api/v1/catalog/databases', ({ store }) => ({
  json: { data: store.catalog.databases },
}))
route('GET', '/api/v1/catalog/providers', ({ store }) => ({
  json: { data: store.catalog.providers },
}))
route('GET', '/api/v1/catalog/stroppy', ({ store }) => ({ json: store.catalog.stroppy }))
route('GET', '/api/v1/catalog/metrics', ({ store }) => ({ json: { data: store.catalog.metrics } }))

route('GET', '/api/v1/catalog/examples', ({ store, query }) => {
  const kind = query.get('kind')
  const db = query.get('db_kind')
  let items = store.catalog.examples
  if (kind) items = items.filter((e) => e.kind === kind)
  if (db) items = items.filter((e) => e.db_kind === db)
  return { json: { data: items } }
})

route('GET', '/api/v1/catalog/schemas', ({ query }) => {
  const ns = query.get('namespace')
  const items = schemas
    .filter((s) => !ns || s.namespace === ns)
    .map(({ body: _b, ...info }) => info)
  return { json: { data: items } }
})

route('GET', '/api/v1/catalog/schemas/:schemaId', ({ params }) => {
  const s = findSchema(params.schemaId)
  return s ? { json: s.body } : notFound(`schema ${params.schemaId}`)
})

// Minimal server-side validation: required fields + int64 bounds + string length/pattern.
route('POST', '/api/v1/catalog/schemas/:schemaId:validate', ({ params, body }) => {
  const s = findSchema(params.schemaId)
  if (!s) return notFound('schema')
  const value = ((body as { value?: Record<string, unknown> })?.value ?? {}) as Record<
    string,
    unknown
  >
  const errors: {
    path: string
    code: string
    severity: 'ERROR' | 'WARNING'
    message: string
    constraint?: string
  }[] = []
  for (const f of (s.body.fields as Record<string, unknown>[]) ?? []) {
    const name = f.name as string
    const v = value[name]
    if (f.required && (v === undefined || v === null || v === ''))
      errors.push({
        path: name,
        code: 'REQUIRED',
        severity: 'ERROR',
        message: `${(f.title as string) ?? name} is required`,
      })
    const i64 = f.int64 as { gte?: string; lte?: string } | undefined
    if (i64 && v !== undefined && v !== '' && v !== null) {
      const n = Number(v)
      if (Number.isNaN(n))
        errors.push({ path: name, code: 'TYPE', severity: 'ERROR', message: 'Must be an integer' })
      else if (i64.gte !== undefined && n < Number(i64.gte))
        errors.push({
          path: name,
          code: 'GTE_VIOLATED',
          severity: 'ERROR',
          message: `Must be ≥ ${i64.gte}`,
          constraint: `gte ${i64.gte}`,
        })
      else if (i64.lte !== undefined && n > Number(i64.lte))
        errors.push({
          path: name,
          code: 'LTE_VIOLATED',
          severity: 'ERROR',
          message: `Must be ≤ ${i64.lte}`,
          constraint: `lte ${i64.lte}`,
        })
    }
    const str = f.string as { minLen?: string; maxLen?: string; pattern?: string } | undefined
    if (str && typeof v === 'string' && v !== '') {
      if (str.minLen && v.length < Number(str.minLen))
        errors.push({
          path: name,
          code: 'MIN_LEN',
          severity: 'ERROR',
          message: `At least ${str.minLen} characters`,
        })
      if (str.maxLen && v.length > Number(str.maxLen))
        errors.push({
          path: name,
          code: 'MAX_LEN',
          severity: 'ERROR',
          message: `At most ${str.maxLen} characters`,
        })
      if (str.pattern && !new RegExp(str.pattern).test(v))
        errors.push({
          path: name,
          code: 'PATTERN',
          severity: 'ERROR',
          message: `Must match ${str.pattern}`,
        })
    }
  }
  return { json: { result: { errors }, resolved: value } }
})

route('POST', '/api/v1/catalog/schemas/:schemaId:render', ({ params, body }) => {
  const s = findSchema(params.schemaId)
  if (!s) return notFound('schema')
  const value = ((body as { value?: Record<string, unknown> })?.value ?? {}) as Record<
    string,
    unknown
  >
  const lines = Object.entries(value).map(
    ([k, v]) => `${k} = ${typeof v === 'string' ? `'${v}'` : JSON.stringify(v)}`
  )
  return { text: `# rendered from ${s.id}\n${lines.join('\n')}\n` }
})

route('POST', '/api/v1/t/:slug/examples/:exampleId:clone', ({ store, params, body }) => {
  const t = store.tenant(params.slug)
  if (!t) return notFound('tenant')
  const ex = store.catalog.examples.find((e) => e.id === params.exampleId)
  if (!ex) return notFound('example')
  const name = (body as { name?: string })?.name ?? ex.title
  const base = {
    id: crypto.randomUUID(),
    name,
    description: ex.description,
    tags: ex.tags,
    author: { id: store.me.id, display_name: store.me.display_name },
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  }
  if (ex.kind === 'database') {
    const src = t.databases[0] ?? store.tenants.main?.databases[0]
    if (!src) return problem(500, 'no_template')
    const d = { ...src, ...base }
    t.databases.unshift(d)
    return { status: 201, json: { created: [{ kind: 'database', id: d.id, name: d.name }] } }
  }
  if (ex.kind === 'workload') {
    const src = t.workloads[0] ?? store.tenants.main?.workloads[0]
    if (!src) return problem(500, 'no_template')
    const w = { ...src, ...base }
    t.workloads.unshift(w)
    return { status: 201, json: { created: [{ kind: 'workload', id: w.id, name: w.name }] } }
  }
  if (ex.kind === 'suite') {
    const src = store.tenants.main?.suites[0]
    if (!src) return problem(500, 'no_template')
    const s = { ...src, ...base }
    t.suites.unshift(s)
    return { status: 201, json: { created: [{ kind: 'suite', id: s.id, name: s.name }] } }
  }
  const src = store.tenants.main?.tests[0]
  if (!src) return problem(500, 'no_template')
  const test = { ...src, ...base, status: 'draft' as const }
  t.tests.unshift(test)
  return { status: 201, json: { created: [{ kind: 'test', id: test.id, name: test.name }] } }
})

import { useQueries } from '@tanstack/react-query'

// What a config schema says about one key: its unit, the engine's default
// and what the key does. The schemas ship with the app (web/src/schemas),
// so a guest reading a shared report needs no catalog API.
export interface ConfigField {
  title?: string
  description?: string
  unit?: string
  default?: unknown
}

export type SchemaFile = { fields?: Array<Record<string, unknown>> }

const SCHEMAS = import.meta.glob<SchemaFile>('../../schemas/cfg.*_v*.json', { import: 'default' })

/** cfg.postgresql.conf@17 → its bundled schema file. */
const pathOf = (id: string) => {
  const [name, version] = id.split('@')
  return `../../schemas/${name}_v${version}.json`
}

// { stringValue: "try" } → "try"; plain values pass.
function unwrap(v: unknown): unknown {
  if (v && typeof v === 'object' && !Array.isArray(v)) {
    const entries = Object.entries(v as Record<string, unknown>)
    if (entries.length === 1 && entries[0][0].endsWith('Value')) return entries[0][1]
  }
  return v
}

/** The keys of a bundled schemapb schema with their unit, default and description. */
export function schemaFields(schema: SchemaFile | null): Record<string, ConfigField> {
  const out: Record<string, ConfigField> = {}
  for (const f of schema?.fields ?? []) {
    const name = f.name as string | undefined
    if (!name) continue
    // The default sits on the typed part: { int64: { default: "128" } }, or
    // as a protoJSON value on a choice: { default: { stringValue: "try" } }.
    let def: unknown
    for (const v of Object.values(f)) {
      if (v && typeof v === 'object' && 'default' in (v as object))
        def = unwrap((v as { default: unknown }).default)
    }
    out[name] = {
      title: f.title as string | undefined,
      description: f.description as string | undefined,
      unit: f.unit as string | undefined,
      default: def,
    }
  }
  return out
}

/** The fields of every config schema id given (cfg.x@v), keyed by id. */
export function useConfigFields(ids: string[]): Record<string, Record<string, ConfigField>> {
  const unique = [...new Set(ids)].sort()
  const results = useQueries({
    queries: unique.map((id) => ({
      queryKey: ['public', 'config-schema', id],
      queryFn: async () => {
        const load = SCHEMAS[pathOf(id)]
        return schemaFields(load ? await load() : null)
      },
      staleTime: Number.POSITIVE_INFINITY,
      retry: false,
    })),
  })
  const out: Record<string, Record<string, ConfigField>> = {}
  unique.forEach((id, i) => {
    out[id] = results[i]?.data ?? {}
  })
  return out
}

const PARAMS = import.meta.glob<SchemaFile>('../../schemas/db.*.params_v*.json', {
  import: 'default',
})

/** The fields of a database kind's newest bundled params schema. */
export function useParamsFields(kind: string | undefined): Record<string, ConfigField> {
  const [q] = useQueries({
    queries: [
      {
        queryKey: ['public', 'params-schema', kind],
        queryFn: async () => {
          const prefix = `../../schemas/db.${kind}.params_v`
          const version = (p: string) => Number(p.slice(prefix.length, -'.json'.length))
          const path = Object.keys(PARAMS)
            .filter((p) => p.startsWith(prefix))
            .sort((a, b) => version(b) - version(a))[0]
          return schemaFields(path ? await PARAMS[path]() : null)
        },
        enabled: !!kind,
        staleTime: Number.POSITIVE_INFINITY,
        retry: false,
      },
    ],
  })
  return q?.data ?? {}
}

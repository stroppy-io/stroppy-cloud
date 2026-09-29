// schemapb protoJSON schema model + the tiny expression engine used by `when`.
export interface ChoiceValue {
  stringValue?: string
  int64Value?: string
  boolValue?: boolean
  doubleValue?: number
}

export interface SchemaField {
  name: string
  title?: string
  description?: string
  group?: string
  required?: boolean
  when?: string
  computed?: boolean
  secret?: boolean
  immutable?: boolean
  deprecated?: boolean
  nullable?: boolean
  unit?: string
  examples?: ChoiceValue[]
  placeholder?: string
  string?: {
    default?: string
    minLen?: string
    maxLen?: string
    pattern?: string
    multiline?: boolean
  }
  int64?: { default?: string; gte?: string; lte?: string; gt?: string; lt?: string }
  uint64?: { default?: string; gte?: string; lte?: string }
  double?: { default?: number; gte?: number; lte?: number; gt?: number; lt?: number }
  bool?: { default?: boolean }
  duration?: { default?: string; gte?: string; lte?: string }
  timestamp?: Record<string, never>
  json?: Record<string, never>
  choice?: {
    default?: ChoiceValue
    options: { label?: string; value: ChoiceValue; description?: string }[]
  }
  list?: {
    items?: SchemaField[]
    item?: SchemaField
    maxItems?: string
    minItems?: string
    unique?: boolean
  }
  map?: { valueField?: SchemaField; maxEntries?: string }
  object?: { schema?: { fields: SchemaField[] }; fields?: SchemaField[] }
  oneOf?: {
    discriminator: string
    variants: Record<string, { fields: SchemaField[]; title?: string }>
  }
  ref?: { name: string }
  rules?: { id?: string; expr: string; message?: string }[]
}

export interface Schema {
  title?: string
  description?: string
  fields: SchemaField[]
  coerce?: boolean
}

export type Values = Record<string, unknown>

export function choiceRaw(v: ChoiceValue | undefined): unknown {
  if (!v) return undefined
  if (v.stringValue !== undefined) return v.stringValue
  if (v.int64Value !== undefined) return Number(v.int64Value)
  if (v.boolValue !== undefined) return v.boolValue
  if (v.doubleValue !== undefined) return v.doubleValue
  return undefined
}

export function fieldKind(f: SchemaField): string {
  for (const k of [
    'string',
    'int64',
    'uint64',
    'double',
    'bool',
    'duration',
    'timestamp',
    'json',
    'choice',
    'list',
    'map',
    'object',
    'oneOf',
    'ref',
  ] as const)
    if (f[k] !== undefined) return k
  return 'string'
}

export function fieldDefault(f: SchemaField): unknown {
  if (f.string?.default !== undefined) return f.string.default
  if (f.int64?.default !== undefined) return Number(f.int64.default)
  if (f.uint64?.default !== undefined) return Number(f.uint64.default)
  if (f.double?.default !== undefined) return f.double.default
  if (f.bool?.default !== undefined) return f.bool.default
  if (f.duration?.default !== undefined) return f.duration.default
  if (f.choice?.default) return choiceRaw(f.choice.default)
  if (f.object) return defaults(objectFields(f))
  if (f.list) return []
  if (f.map) return {}
  if (f.oneOf) {
    const first = Object.keys(f.oneOf.variants)[0]
    return first
      ? { [f.oneOf.discriminator]: first, ...defaults(f.oneOf.variants[first].fields) }
      : {}
  }
  if (f.bool) return false
  return undefined
}

export function objectFields(f: SchemaField): SchemaField[] {
  return f.object?.schema?.fields ?? f.object?.fields ?? []
}

export function listItem(f: SchemaField): SchemaField | undefined {
  return f.list?.item ?? f.list?.items?.[0]
}

// Values object with every field's default filled in (deep).
export function defaults(fields: SchemaField[]): Values {
  const out: Values = {}
  for (const f of fields) {
    const d = fieldDefault(f)
    if (d !== undefined) out[f.name] = d
  }
  return out
}

export function applyDefaults(fields: SchemaField[], value: Values | undefined): Values {
  return { ...defaults(fields), ...(value ?? {}) }
}

// --- `when` expressions: a CEL subset — `root.a == "x"`, `root.b`, `!root.c`, `&&`, `||`, `!=`, `>`, `<`, parentheses.
type Tok = { t: 'id' | 'str' | 'num' | 'op' | 'lp' | 'rp'; v: string }

function tokenize(src: string): Tok[] {
  const out: Tok[] = []
  let i = 0
  while (i < src.length) {
    const c = src[i]
    if (/\s/.test(c)) {
      i++
      continue
    }
    if (c === '(') {
      out.push({ t: 'lp', v: c })
      i++
      continue
    }
    if (c === ')') {
      out.push({ t: 'rp', v: c })
      i++
      continue
    }
    if (c === '"' || c === "'") {
      const j = src.indexOf(c, i + 1)
      out.push({ t: 'str', v: src.slice(i + 1, j < 0 ? src.length : j) })
      i = j < 0 ? src.length : j + 1
      continue
    }
    const two = src.slice(i, i + 2)
    if (['==', '!=', '&&', '||', '>=', '<='].includes(two)) {
      out.push({ t: 'op', v: two })
      i += 2
      continue
    }
    if ('!<>'.includes(c)) {
      out.push({ t: 'op', v: c })
      i++
      continue
    }
    const m = src.slice(i).match(/^-?\d+(\.\d+)?/)
    if (m) {
      out.push({ t: 'num', v: m[0] })
      i += m[0].length
      continue
    }
    const id = src.slice(i).match(/^[A-Za-z_][A-Za-z0-9_.]*/)
    if (id) {
      out.push({ t: 'id', v: id[0] })
      i += id[0].length
      continue
    }
    i++
  }
  return out
}

function lookup(path: string, root: Values, self: Values): unknown {
  const parts = path.split('.')
  let cur: unknown = parts[0] === 'root' ? root : parts[0] === 'this' ? self : self
  const rest = parts[0] === 'root' || parts[0] === 'this' ? parts.slice(1) : parts
  for (const p of rest) {
    if (cur && typeof cur === 'object') cur = (cur as Values)[p]
    else return undefined
  }
  return cur
}

export function evalWhen(expr: string | undefined, root: Values, self: Values = root): boolean {
  if (!expr) return true
  const toks = tokenize(expr)
  let pos = 0
  const peek = () => toks[pos]
  const next = () => toks[pos++]
  const primary = (): unknown => {
    const tk = next()
    if (!tk) return undefined
    if (tk.t === 'lp') {
      const v = or()
      if (peek()?.t === 'rp') next()
      return v
    }
    if (tk.t === 'op' && tk.v === '!') return !primary()
    if (tk.t === 'str') return tk.v
    if (tk.t === 'num') return Number(tk.v)
    if (tk.t === 'id') {
      if (tk.v === 'true') return true
      if (tk.v === 'false') return false
      return lookup(tk.v, root, self)
    }
    return undefined
  }
  const cmp = (): unknown => {
    let l = primary()
    while (peek()?.t === 'op' && ['==', '!=', '>', '<', '>=', '<='].includes(peek().v)) {
      const op = next().v
      const r = primary()
      // loose equality so "3" == 3 works for coerced schemas
      // biome-ignore lint/suspicious/noDoubleEquals: intentional loose compare
      if (op === '==') l = l == r
      // biome-ignore lint/suspicious/noDoubleEquals: intentional loose compare
      else if (op === '!=') l = l != r
      else if (op === '>') l = Number(l) > Number(r)
      else if (op === '<') l = Number(l) < Number(r)
      else if (op === '>=') l = Number(l) >= Number(r)
      else if (op === '<=') l = Number(l) <= Number(r)
    }
    return l
  }
  const and = (): unknown => {
    let l = cmp()
    while (peek()?.t === 'op' && peek().v === '&&') {
      next()
      const r = cmp()
      l = !!l && !!r
    }
    return l
  }
  const or = (): unknown => {
    let l = and()
    while (peek()?.t === 'op' && peek().v === '||') {
      next()
      const r = and()
      l = !!l || !!r
    }
    return l
  }
  return !!or()
}

// Client-side pre-validation mirroring the server's basic constraints (required/bounds/pattern).
export function validateLocal(
  fields: SchemaField[],
  values: Values,
  root: Values = values,
  prefix = ''
): Record<string, string> {
  const errors: Record<string, string> = {}
  for (const f of fields) {
    if (!evalWhen(f.when, root, values)) continue
    const path = prefix ? `${prefix}.${f.name}` : f.name
    const v = values[f.name]
    const empty = v === undefined || v === null || v === ''
    if (f.required && empty && !f.computed) errors[path] = 'required'
    if (empty) continue
    if (f.int64 || f.uint64) {
      const n = Number(v)
      if (!Number.isInteger(n)) errors[path] = 'integer'
      else {
        const s = f.int64 ?? f.uint64
        if (s?.gte !== undefined && n < Number(s.gte)) errors[path] = `gte:${s.gte}`
        if (s?.lte !== undefined && n > Number(s.lte)) errors[path] = `lte:${s.lte}`
        if (f.int64?.gt !== undefined && n <= Number(f.int64.gt)) errors[path] = `gt:${f.int64.gt}`
        if (f.int64?.lt !== undefined && n >= Number(f.int64.lt)) errors[path] = `lt:${f.int64.lt}`
      }
    }
    if (f.double) {
      const n = Number(v)
      if (Number.isNaN(n)) errors[path] = 'number'
      else {
        if (f.double.gte !== undefined && n < f.double.gte) errors[path] = `gte:${f.double.gte}`
        if (f.double.lte !== undefined && n > f.double.lte) errors[path] = `lte:${f.double.lte}`
        if (f.double.gt !== undefined && n <= f.double.gt) errors[path] = `gt:${f.double.gt}`
      }
    }
    if (f.string && typeof v === 'string') {
      if (f.string.minLen && v.length < Number(f.string.minLen))
        errors[path] = `minLen:${f.string.minLen}`
      if (f.string.maxLen && v.length > Number(f.string.maxLen))
        errors[path] = `maxLen:${f.string.maxLen}`
      if (f.string.pattern) {
        try {
          if (!new RegExp(f.string.pattern).test(v)) errors[path] = `pattern:${f.string.pattern}`
        } catch {
          // invalid pattern in schema — skip
        }
      }
    }
    if (f.object && v && typeof v === 'object')
      Object.assign(errors, validateLocal(objectFields(f), v as Values, root, path))
    if (f.oneOf && v && typeof v === 'object') {
      const disc = (v as Values)[f.oneOf.discriminator] as string
      const variant = f.oneOf.variants[disc]
      if (variant) Object.assign(errors, validateLocal(variant.fields, v as Values, root, path))
    }
  }
  return errors
}

export function groupFields(fields: SchemaField[]): { group: string; fields: SchemaField[] }[] {
  const out: { group: string; fields: SchemaField[] }[] = []
  for (const f of fields) {
    const g = f.group ?? ''
    let bucket = out.find((b) => b.group === g)
    if (!bucket) {
      bucket = { group: g, fields: [] }
      out.push(bucket)
    }
    bucket.fields.push(f)
  }
  return out
}

// Minimal YAML ⇄ JSON for library export/import documents (`kind`, `api_version`, `metadata`, `spec`).
// Supports block maps, block lists, flow `[a, b]`/`{}` scalars, quoted strings, `|` literal blocks
// and comments — the subset the server emits. Anything exotic (anchors, tags) is out of scope;
// JSON input is accepted as-is.

export function parseDocument(text: string): unknown {
  const trimmed = text.trim()
  if (!trimmed) throw new Error('empty document')
  if (trimmed.startsWith('{') || trimmed.startsWith('[')) return JSON.parse(trimmed)
  return parseYaml(trimmed)
}

interface Line {
  indent: number
  text: string
  raw: string
}

function splitLines(src: string): Line[] {
  return src
    .replace(/\r\n?/g, '\n')
    .split('\n')
    .map((raw) => ({ raw, indent: raw.match(/^ */)?.[0].length ?? 0, text: raw.trim() }))
    .filter((l) => l.text !== '' && !l.text.startsWith('#') && l.text !== '---')
}

function scalar(s: string): unknown {
  const v = s.trim()
  if (v === '' || v === '~' || v === 'null') return null
  if (v === 'true') return true
  if (v === 'false') return false
  if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) {
    return v.startsWith('"') ? JSON.parse(v) : v.slice(1, -1).replace(/''/g, "'")
  }
  if (v.startsWith('[') && v.endsWith(']')) {
    const inner = v.slice(1, -1).trim()
    return inner ? splitFlow(inner).map(scalar) : []
  }
  if (v.startsWith('{') && v.endsWith('}')) {
    const inner = v.slice(1, -1).trim()
    const out: Record<string, unknown> = {}
    if (!inner) return out
    for (const part of splitFlow(inner)) {
      const i = part.indexOf(':')
      if (i < 0) continue
      out[scalarKey(part.slice(0, i))] = scalar(part.slice(i + 1))
    }
    return out
  }
  // Integers beyond the safe range stay strings (bigint-safe round trip).
  if (/^-?\d+$/.test(v)) {
    const n = Number(v)
    return Number.isSafeInteger(n) ? n : v
  }
  if (/^-?\d+\.\d+$/.test(v)) return Number(v)
  return v
}

function scalarKey(s: string): string {
  const k = scalar(s)
  return typeof k === 'string' ? k : String(k)
}

function splitFlow(s: string): string[] {
  const out: string[] = []
  let depth = 0
  let cur = ''
  let quote: string | null = null
  for (const ch of s) {
    if (quote) {
      cur += ch
      if (ch === quote) quote = null
      continue
    }
    if (ch === '"' || ch === "'") {
      quote = ch
      cur += ch
      continue
    }
    if (ch === '[' || ch === '{') depth++
    if (ch === ']' || ch === '}') depth--
    if (ch === ',' && depth === 0) {
      out.push(cur)
      cur = ''
      continue
    }
    cur += ch
  }
  if (cur.trim()) out.push(cur)
  return out
}

function stripComment(s: string): string {
  let quote: string | null = null
  for (let i = 0; i < s.length; i++) {
    const ch = s[i]
    if (quote) {
      if (ch === quote) quote = null
      continue
    }
    if (ch === '"' || ch === "'") quote = ch
    else if (ch === '#' && (i === 0 || s[i - 1] === ' ')) return s.slice(0, i).trimEnd()
  }
  return s
}

// Split `key: value` at the first `:` that is followed by a space/end and not inside quotes.
function splitKey(text: string): [string, string] | null {
  let quote: string | null = null
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]
    if (quote) {
      if (ch === quote) quote = null
      continue
    }
    if (ch === '"' || ch === "'") quote = ch
    else if (ch === ':' && (i === text.length - 1 || text[i + 1] === ' ')) {
      return [text.slice(0, i), text.slice(i + 1)]
    }
  }
  return null
}

function parseYaml(src: string): unknown {
  const lines = splitLines(src)
  let pos = 0

  const literalBlock = (indent: number): string => {
    const parts: string[] = []
    while (pos < lines.length && lines[pos].indent > indent) {
      parts.push(lines[pos].raw.slice(Math.min(lines[pos].indent, indent + 2)))
      pos++
    }
    return `${parts.join('\n')}\n`
  }

  const parseBlock = (indent: number): unknown => {
    if (pos >= lines.length) return null
    const first = lines[pos]
    if (first.text.startsWith('- ') || first.text === '-') return parseList(first.indent)
    return parseMap(indent < 0 ? first.indent : first.indent)
  }

  const parseValue = (rest: string, indent: number): unknown => {
    const v = stripComment(rest).trim()
    if (v === '|' || v === '|-' || v === '>')
      return literalBlock(indent).replace(/\n$/, v === '|-' ? '' : '\n')
    if (v === '') {
      if (pos < lines.length && lines[pos].indent > indent) return parseBlock(lines[pos].indent)
      if (pos < lines.length && lines[pos].indent === indent && lines[pos].text.startsWith('- '))
        return parseList(indent)
      return null
    }
    return scalar(v)
  }

  const parseMap = (indent: number): Record<string, unknown> => {
    const out: Record<string, unknown> = {}
    while (pos < lines.length) {
      const l = lines[pos]
      if (l.indent < indent) break
      if (l.indent > indent) throw new Error(`bad indentation at line: ${l.raw}`)
      if (l.text.startsWith('- ')) break
      const kv = splitKey(l.text)
      if (!kv) throw new Error(`expected "key: value" at: ${l.raw}`)
      pos++
      out[scalarKey(kv[0])] = parseValue(kv[1], indent)
    }
    return out
  }

  const parseList = (indent: number): unknown[] => {
    const out: unknown[] = []
    while (pos < lines.length) {
      const l = lines[pos]
      if (l.indent !== indent || !(l.text.startsWith('- ') || l.text === '-')) break
      pos++
      const rest = l.text === '-' ? '' : l.text.slice(2)
      const kv = splitKey(stripComment(rest))
      if (
        kv &&
        !rest.trim().startsWith('[') &&
        !rest.trim().startsWith('{') &&
        !rest.trim().startsWith('"')
      ) {
        // inline map item: `- key: value` followed by more keys at indent+2
        const item: Record<string, unknown> = {}
        item[scalarKey(kv[0])] = parseValue(kv[1], indent + 2)
        if (
          pos < lines.length &&
          lines[pos].indent === indent + 2 &&
          !lines[pos].text.startsWith('- ')
        )
          Object.assign(item, parseMap(indent + 2))
        out.push(item)
      } else out.push(parseValue(rest, indent))
    }
    return out
  }

  return parseBlock(-1)
}

export function toYaml(value: unknown, indent = 0): string {
  const pad = ' '.repeat(indent)
  if (value === null || value === undefined) return 'null'
  if (typeof value === 'string') return yamlString(value, indent)
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  if (Array.isArray(value)) {
    if (!value.length) return '[]'
    return value
      .map((v) => {
        if (v !== null && typeof v === 'object' && !Array.isArray(v) && Object.keys(v).length) {
          const body = toYaml(v, indent + 2).trimStart()
          return `${pad}- ${body}`
        }
        return `${pad}- ${toYaml(v, indent + 2)}`
      })
      .join('\n')
  }
  const entries = Object.entries(value as Record<string, unknown>).filter(
    ([, v]) => v !== undefined
  )
  if (!entries.length) return '{}'
  return entries
    .map(([k, v]) => {
      const key = /^[A-Za-z0-9_./-]+$/.test(k) ? k : JSON.stringify(k)
      if (
        v !== null &&
        typeof v === 'object' &&
        (Array.isArray(v) ? v.length : Object.keys(v).length)
      )
        return `${pad}${key}:\n${toYaml(v, indent + 2)}`
      return `${pad}${key}: ${toYaml(v, indent + 2)}`
    })
    .join('\n')
}

function yamlString(s: string, indent: number): string {
  if (s.includes('\n')) {
    const pad = ' '.repeat(indent)
    return `|\n${s
      .replace(/\n$/, '')
      .split('\n')
      .map((l) => pad + l)
      .join('\n')}`
  }
  if (
    s === '' ||
    /^[\s#&*!|>'"%@`{}[\],:-]|[:#]\s|\s$/.test(s) ||
    ['true', 'false', 'null', '~'].includes(s) ||
    /^-?\d+(\.\d+)?$/.test(s)
  )
    return JSON.stringify(s)
  return s
}

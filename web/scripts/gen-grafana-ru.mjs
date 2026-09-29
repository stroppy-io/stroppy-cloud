// Builds src/locales/grafana/ru.json — Russian for the built-in strings of @grafana/ui and
// @grafana/data. Grafana ships no translations in its npm packages (it passes the English default
// with every `t()` and loads bundles at runtime), so we take its official ru-RU `grafana.json`
// for the installed version, keep only the keys the installed dist actually references, and
// apply src/locales/grafana/ru.overrides.json (keys Grafana's Crowdin left empty + our wording).
//
// Run after bumping @grafana/ui: `node scripts/gen-grafana-ru.mjs` (needs network). Exits 1 when
// a used key ends up without a Russian string — add it to the overrides file.
import { readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = dirname(dirname(fileURLToPath(import.meta.url)))
const version = JSON.parse(
  readFileSync(join(root, 'node_modules/@grafana/ui/package.json'), 'utf8')
).version
const base = `https://raw.githubusercontent.com/grafana/grafana/v${version}/public/locales`

async function load(lang) {
  const res = await fetch(`${base}/${lang}/grafana.json`)
  if (!res.ok) throw new Error(`${lang}: HTTP ${res.status}`)
  return flat(await res.json())
}

function flat(o, pre = '', out = {}) {
  for (const [k, v] of Object.entries(o)) {
    const key = pre ? `${pre}.${k}` : k
    if (v && typeof v === 'object') flat(v, key, out)
    else out[key] = v
  }
  return out
}

// Keys referenced by the dist: `t('key', …)`, `<Trans i18nKey="key">` and key constants
// (`NO_OPTIONS_I18N_KEY = "combobox.options.no-found"`) — the latter as any dotted literal that
// is a known key.
const called = new Set()
const literals = new Set()
function walk(dir) {
  for (const f of readdirSync(dir)) {
    const p = join(dir, f)
    if (statSync(p).isDirectory()) walk(p)
    else if (p.endsWith('.mjs')) {
      const src = readFileSync(p, 'utf8')
      if (!src.includes('@grafana/i18n')) continue
      for (const m of src.matchAll(/\bt\(\s*["'`]([a-zA-Z][\w.-]+)["'`]/g)) called.add(m[1])
      for (const m of src.matchAll(/i18nKey:\s*["']([^"']+)["']/g)) called.add(m[1])
      for (const m of src.matchAll(/["']([a-z][a-z0-9-]*\.[\w.-]+)["']/g)) literals.add(m[1])
    }
  }
}
walk(join(root, 'node_modules/@grafana/ui/dist/esm'))
walk(join(root, 'node_modules/@grafana/data/dist/esm'))

const [en, ru] = await Promise.all([load('en-US'), load('ru-RU')])
const overrides = JSON.parse(
  readFileSync(join(root, 'src/locales/grafana/ru.overrides.json'), 'utf8')
)

// A key plus its plural forms (`key_one`, `key_few`, …).
const forms = (dict, k) =>
  Object.keys(dict).filter(
    (x) => x === k || (x.startsWith(`${k}_`) && !x.slice(k.length + 1).includes('.'))
  )
for (const l of literals) if (forms(en, l).length) called.add(l)

const out = {}
const missing = []
let leaves = 0
for (const k of [...called].sort()) {
  const all = new Set([...forms(en, k), ...forms(ru, k)])
  if (!all.size) missing.push(`${k} (unknown to Grafana ${version})`)
  for (const key of all) {
    const val = overrides[key] ?? ru[key]
    if (!val) {
      missing.push(`${key} | ${en[key]}`)
      continue
    }
    const parts = key.split('.')
    let node = out
    for (const p of parts.slice(0, -1)) {
      node[p] ??= {}
      node = node[p]
    }
    node[parts.at(-1)] = val
    leaves++
  }
}
writeFileSync(join(root, 'src/locales/grafana/ru.json'), `${JSON.stringify(out, null, 2)}\n`)
console.log(`[gen-grafana-ru] grafana ${version}: ${called.size} keys, ${leaves} strings`)
if (missing.length) {
  console.error(`[gen-grafana-ru] no Russian for:\n  ${missing.join('\n  ')}`)
  process.exit(1)
}

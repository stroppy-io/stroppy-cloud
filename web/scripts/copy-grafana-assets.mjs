// @grafana/ui <Icon> loads SVGs from `${window.__grafana_public_path__}build/img/icons/`.
// The package ships them under dist/public/img/icons; copy into public/ so vite serves them
// in dev and bundles them in build. Fonts (public/gf/fonts) are committed separately.
import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = dirname(dirname(fileURLToPath(import.meta.url)))
const src = join(root, 'node_modules/@grafana/ui/dist/public/img/icons')
const dest = join(root, 'public/gf/build/img/icons')
if (!existsSync(src)) {
  console.error('[copy-grafana-assets] source missing:', src)
  process.exit(1)
}
rmSync(dest, { recursive: true, force: true })
mkdirSync(dirname(dest), { recursive: true })
cpSync(src, dest, { recursive: true })
console.log('[copy-grafana-assets] icons ->', dest)

// Builds the kit bundle a release ships for HTML reports: the kit's CSS,
// fonts, tokens and logo as they are, plus chart.js (uPlot and src/kit/chart.ts
// in one script without modules, exposing window.StroppyChart) and chart.css
// (uPlot's stylesheet and the kit's). Output: dist-kit/stroppy-kit/ and
// dist-kit/stroppy-kit-<version>.tar.gz.
//
//   node scripts/kit-bundle.mjs            version from `git describe`
//   KIT_VERSION=0.4.0 node scripts/...     an explicit version (the release)
import { execFileSync } from 'node:child_process'
import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { build } from 'vite'

const root = dirname(dirname(fileURLToPath(import.meta.url)))
const kit = join(root, 'src/kit')
const out = join(root, 'dist-kit')
const dir = join(out, 'stroppy-kit')

const version =
  process.env.KIT_VERSION ||
  execFileSync('git', ['describe', '--tags', '--always', '--dirty'], { cwd: root })
    .toString()
    .trim()
    .replace(/^v/, '')

rmSync(out, { recursive: true, force: true })
mkdirSync(dir, { recursive: true })

await build({
  configFile: false,
  logLevel: 'warn',
  build: {
    outDir: dir,
    emptyOutDir: false,
    copyPublicDir: false,
    lib: {
      entry: join(kit, 'chart.ts'),
      name: 'StroppyChart',
      formats: ['iife'],
      fileName: () => 'chart.js',
    },
  },
})

for (const f of ['tokens.json', 'tokens.css', 'fonts.css', 'base.css', 'README.md']) {
  cpSync(join(kit, f), join(dir, f))
}
cpSync(join(kit, 'fonts'), join(dir, 'fonts'), { recursive: true })
cpSync(join(kit, 'logo'), join(dir, 'logo'), { recursive: true })
writeFileSync(
  join(dir, 'chart.css'),
  `${readFileSync(join(root, 'node_modules/uplot/dist/uPlot.min.css'), 'utf8')}\n${readFileSync(join(kit, 'chart.css'), 'utf8')}`
)
cpSync(join(kit, 'example.html'), join(dir, 'example.html'))
writeFileSync(join(dir, 'VERSION'), `${version}\n`)

const tarball = `stroppy-kit-${version}.tar.gz`
execFileSync('tar', ['-czf', tarball, 'stroppy-kit'], { cwd: out })
console.log(`[kit-bundle] ${join(out, tarball)}`)

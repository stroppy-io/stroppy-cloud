// Builds the kit bundle a release ships for HTML reports: the kit's CSS,
// fonts, tokens and logo as they are, plus chart.js (uPlot and src/kit/chart.ts
// in one script without modules, exposing window.StroppyChart) and chart.css
// (uPlot's stylesheet and the kit's). Output: dist-kit/stroppy-kit/ and
// dist-kit/stroppy-kit-<version>.tar.gz.
//
//   node scripts/kit-bundle.mjs            version from `git describe` (product tags v*)
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
  execFileSync('git', ['describe', '--tags', '--match', 'v[0-9]*', '--always', '--dirty'], { cwd: root })
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

for (const f of ['tokens.json', 'tokens.css', 'fonts.css', 'base.css', 'hatch.svg', 'README.md']) {
  cpSync(join(kit, f), join(dir, f))
}
cpSync(join(kit, 'fonts'), join(dir, 'fonts'), { recursive: true })
cpSync(join(kit, 'logo'), join(dir, 'logo'), { recursive: true })
writeFileSync(
  join(dir, 'chart.css'),
  `${readFileSync(join(root, 'node_modules/uplot/dist/uPlot.min.css'), 'utf8')}\n${readFileSync(join(kit, 'chart.css'), 'utf8')}`
)
cpSync(join(kit, 'example.html'), join(dir, 'example.html'))

// What the bundle carries from others, with versions and licence texts:
// uPlot is inside chart.js and chart.css; the fonts carry their OFL next to them.
const uplot = JSON.parse(readFileSync(join(root, 'node_modules/uplot/package.json'), 'utf8'))
mkdirSync(join(dir, 'licenses'), { recursive: true })
cpSync(join(root, 'node_modules/uplot/LICENSE'), join(dir, 'licenses/uPlot.txt'))
writeFileSync(
  join(dir, 'THIRD_PARTY_NOTICES.md'),
  `# Third-party software in the Stroppy UI kit ${version}

| Component | Version | Where | Licence | Text |
|---|---|---|---|---|
| uPlot | ${uplot.version} | inside \`chart.js\` and \`chart.css\` | MIT | \`licenses/uPlot.txt\` |
| Inter | woff2 | \`fonts/inter/\` | SIL Open Font License 1.1 | \`fonts/inter/OFL.txt\` |
| Roboto Mono | woff2 | \`fonts/roboto/\` | SIL Open Font License 1.1 | \`fonts/roboto/OFL.txt\` |

A single-file report that inlines these files keeps these notices with it.
`
)
writeFileSync(join(dir, 'VERSION'), `${version}\n`)

const tarball = `stroppy-kit-${version}.tar.gz`
execFileSync('tar', ['-czf', tarball, 'stroppy-kit'], { cwd: out })
console.log(`[kit-bundle] ${join(out, tarball)}`)

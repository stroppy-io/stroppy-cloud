import { describe, expect, it } from 'vitest'

// The Russian UI says everything in Russian. A ru value with no Cyrillic is
// almost always a string copied from en and forgotten; the names below are
// the deliberate ones (products, formats, units, technical terms).
const LATIN_BY_DESIGN = new Set([
  'admin.sort.email',
  'common.appName',
  'common.trigger.api',
  'common.fields.email',
  'common.fields.url',
  'common.fields.cron',
  'common.lang.en',
  'common.topologyRole.etcd',
  'common.topologyMode.galera',
  'common.topologyMode.patroni',
  'common.topologyMode.block-4-2',
  'common.topologyMode.mirror-3-dc',
  'library.requirements.reasons.proxy',
  'common.topologyRole.patroni',
  'common.topologyRole.pgbouncer',
  'common.quota.ec2_spot_standard_vcpus',
  'common.quota.vpc_vpcs',
  'common.quota.ec2_elastic_ips',
  'common.quotaUnit.vcpus',
  'dashboard.providers.quotaOf',
  'kit.title',
  'landing.footer.legal',
  'landing.events.checkpoint',
  'landing.events.vacuum',
  'landing.link.db-replica',
  'landing.link.runner',
  'library.columns.stroppy',
  'library.requirements.cpu',
  'library.list.vus',
  'library.build.steps.kinds.sysctl',
  'onboarding.concepts.test.sample',
  'onboarding.concepts.provider.sample',
  'public.export.md',
  'public.export.json',
  'public.export.csv',
  'public.export.pdf',
  'public.results.segment.tps',
  'public.suite.tps',
  'results.rating.filters.stroppy',
  'results.rating.columns.stroppy',
  'runs.columns.tps',
  'runs.progress',
  'runs.overview.snapshot.segmentsCount',
  'runs.overview.snapshot.stroppy',
  'runs.logs.raw.button',
  'runs.topology.server',
  'runs.topology.fields.cpu',
  'runs.metrics.raw.button',
  'runs.spec.runSpec',
  'runs.compare.count',
  'runs.list.vus',
  'schedules.columns.cron',
  'settings.general.emailsPlaceholder',
  'suites.form.tagsPlaceholder',
  'suites.cells.quotaFail',
  'suites.import.placeholder',
  'suites.suiteRuns.summary.columns.tps',
  'suites.suiteRuns.cells.columns.tps',
])
// The architecture diagram names technologies, and the onboarding fact
// lists name products — neither is UI wording.
const LATIN_PREFIXES = ['how.arch.']
const PRODUCT_LIST = /\.items\.\d+$/

const ru = import.meta.glob('../locales/ru/*.json', { eager: true, import: 'default' }) as Record<
  string,
  Record<string, unknown>
>

function* flat(node: unknown, path: string): Generator<[string, string]> {
  if (typeof node === 'string') yield [path, node]
  else if (node && typeof node === 'object')
    for (const [k, v] of Object.entries(node)) yield* flat(v, path ? `${path}.${k}` : k)
}

describe('ru locale', () => {
  it('has no English left in it', () => {
    const english: string[] = []
    for (const [file, tree] of Object.entries(ru)) {
      const ns = file.replace(/^.*\/(.+)\.json$/, '$1')
      for (const [key, value] of flat(tree, ns)) {
        if (
          LATIN_BY_DESIGN.has(key) ||
          PRODUCT_LIST.test(key) ||
          LATIN_PREFIXES.some((p) => key.startsWith(p))
        )
          continue
        if (/[A-Za-z]{3,}/.test(value) && !/[А-Яа-яЁё]/.test(value))
          english.push(`${key} = ${value}`)
      }
    }
    expect(english).toEqual([])
  })
})

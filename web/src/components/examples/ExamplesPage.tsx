import { catalogQueries } from '@api/queries/catalog'
import { meQueries } from '@api/queries/me'
import { exampleQueries } from '@api/queries/results'
import type { Example } from '@api/types'
import { Page, PageFill } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { css } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import {
  Alert,
  Button,
  Card,
  EmptyState,
  Icon,
  LoadingPlaceholder,
  RadioButtonGroup,
  Select,
  TagList,
  useStyles2,
} from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { useMe } from '@hooks/useMe'
import { useQuery } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { CloneModal } from './CloneModal'
import { QuickRunModal } from './QuickRunModal'

export const examplesSearchSchema = z.object({
  kind: z.enum(['database', 'workload', 'test', 'suite']).optional().catch(undefined),
  db: z.string().optional().catch(undefined),
  tenant: z.string().optional().catch(undefined),
})
export type ExamplesSearch = z.infer<typeof examplesSearchSchema>

const KINDS: Example['kind'][] = ['test', 'suite', 'database', 'workload']

const KIND_ICON: Record<Example['kind'], IconName> = {
  database: 'database',
  workload: 'bolt',
  test: 'vial',
  suite: 'layer-group',
}

// ---- what an example contains, read from its document (the shape the clone would create) ----

type Params = Record<string, unknown>
interface DbSpec {
  kind?: string
  version?: string
  params?: Params
}
interface Segment {
  name?: string
  run?: { duration?: string; vus?: number; iterations?: number; executor?: string }
  workload?: { script?: string; scale_factor?: number }
}
interface WlSpec {
  protocol?: string
  stroppy_version?: string
  segments?: Segment[]
}
type Sizes = Record<string, { size?: string }>

interface Fact {
  label: string
  value: string
  // One value per line (segments, suite tests).
  lines?: string[]
}

function dbLine(db: DbSpec | undefined, engineTitle: (k: string) => string): string | undefined {
  if (!db?.kind) return undefined
  const params = Object.entries(db.params ?? {})
    .filter(([k, v]) => k !== 'version' && v !== undefined && v !== null && v !== false)
    .map(([k, v]) => (v === true ? k : `${k} ${v}`))
  return [`${engineTitle(db.kind)} ${db.version ?? ''}`.trim(), ...params].join(' · ')
}

function segmentLine(s: Segment, t: TFunction): string {
  const run = s.run ?? {}
  const parts = [
    s.workload?.script,
    s.workload?.scale_factor !== undefined ? `SF ${s.workload.scale_factor}` : undefined,
    run.vus ? `${run.vus} VU` : undefined,
    run.duration
      ? formatDuration(run.duration)
      : run.iterations
        ? t('examples.card.iterations', { count: run.iterations })
        : undefined,
  ].filter(Boolean)
  return `${s.name ?? '—'}: ${parts.join(' · ')}`
}

function wlFacts(wl: WlSpec | undefined, t: TFunction): Fact[] {
  if (!wl) return []
  const segs = wl.segments ?? []
  return [
    {
      label: t('examples.card.protocol'),
      value: [wl.protocol, wl.stroppy_version ? `stroppy ${wl.stroppy_version}` : undefined]
        .filter(Boolean)
        .join(' · '),
    },
    {
      label: t('examples.card.segments', { count: segs.length }),
      value: '',
      lines: segs.map((s) => segmentLine(s, t)),
    },
  ]
}

function sizesLine(sizes: Sizes | undefined): string | undefined {
  const e = Object.entries(sizes ?? {})
  return e.length ? e.map(([role, v]) => `${role}: ${v?.size ?? '?'}`).join(' · ') : undefined
}

function factsOf(e: Example, t: TFunction, engineTitle: (k: string) => string): Fact[] {
  const spec = ((e.document as { spec?: unknown } | undefined)?.spec ?? {}) as Record<
    string,
    unknown
  >
  switch (e.kind) {
    case 'database': {
      const db = spec as DbSpec
      const params = Object.entries(db.params ?? {}).filter(([k]) => k !== 'version')
      return [
        {
          label: t('examples.card.engine'),
          value: `${engineTitle(db.kind ?? '')} ${db.version ?? ''}`.trim(),
        },
        {
          label: t('examples.card.topology'),
          value: params.length ? '' : t('examples.card.singleNode'),
          lines: params.map(([k, v]) => `${k}: ${String(v)}`),
        },
      ]
    }
    case 'workload':
      return wlFacts(spec as WlSpec, t)
    case 'test': {
      const db = (spec.database as { inline?: DbSpec } | undefined)?.inline
      const wl = (spec.workload as { inline?: WlSpec } | undefined)?.inline
      return [
        { label: t('examples.card.database'), value: dbLine(db, engineTitle) ?? '—' },
        ...wlFacts(wl, t),
        { label: t('examples.card.sizes'), value: sizesLine(spec.sizes as Sizes) ?? '—' },
      ]
    }
    case 'suite': {
      const tests = (spec.tests ?? []) as {
        inline_name?: string
        inline?: { database_inline?: DbSpec; workload_inline?: WlSpec }
      }[]
      const sizes = ((spec.axes as { sizes?: Sizes[] } | undefined)?.sizes ?? [])
        .map(sizesLine)
        .filter(Boolean) as string[]
      return [
        {
          label: t('examples.card.tests', { count: tests.length }),
          value: '',
          lines: tests.map((x) => {
            const segs = x.inline?.workload_inline?.segments ?? []
            return `${x.inline_name ?? '—'}: ${dbLine(x.inline?.database_inline, engineTitle) ?? '—'}${
              segs.length
                ? ` — ${segs
                    .map((s) => s.workload?.script)
                    .filter(Boolean)
                    .join(', ')}`
                : ''
            }`
          }),
        },
        {
          label: t('examples.card.sizeVariants', { count: sizes.length }),
          value: sizes.length ? '' : '—',
          lines: sizes,
        },
        {
          label: t('examples.card.concurrency'),
          value: String((spec.concurrency as number | undefined) ?? 1),
        },
      ]
    }
  }
}

const getStyles = (theme: GrafanaTheme2) => ({
  filters: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    flexWrap: 'wrap',
    marginBottom: theme.spacing(2),
  }),
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(420px, 1fr))',
    gap: theme.spacing(2),
    alignContent: 'start',
    paddingBottom: theme.spacing(2),
  }),
  card: css({ height: '100%', marginBottom: 0 }),
  facts: css({
    display: 'grid',
    gridTemplateColumns: 'max-content minmax(0, 1fr)',
    gap: theme.spacing(0.5, 2),
    marginTop: theme.spacing(1),
    fontSize: theme.typography.bodySmall.fontSize,
    dt: { color: theme.colors.text.secondary },
    dd: { margin: 0, minWidth: 0, overflowWrap: 'anywhere' },
  }),
  line: css({ fontFamily: theme.typography.fontFamilyMonospace, display: 'block' }),
  figure: css({ color: theme.colors.text.secondary }),
})

// Example gallery (web/AGENTS.md: the catalog is bounded, filters go to the server).
// Each card shows everything the example would create — engine and topology, every workload
// segment, sizes, suite tests and axes — so the choice is made on the card, not after cloning.
export function ExamplesPage({
  search,
  onSearchChange,
}: {
  search: ExamplesSearch
  onSearchChange: (next: Partial<ExamplesSearch>) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const me = useMe()
  const config = useQuery(meQueries.publicConfig())
  const enabled = config.data?.examples_enabled ?? true
  const list = useQuery(
    exampleQueries.list({ kind: search.kind, db_kind: search.db as Example['db_kind'] })
  )
  const databases = useQuery(catalogQueries.databases())
  const [clone, setClone] = useState<Example | undefined>()
  const [quick, setQuick] = useState<Example | undefined>()
  const defaultSlug = search.tenant ?? me.preferences.default_tenant ?? me.tenants[0]?.tenant.slug
  const rows = list.data?.data ?? []
  const engines = databases.data?.data ?? []
  const engineTitle = (k: string) => engines.find((d) => d.kind === k)?.title ?? k
  const dbOptions = engines.map((d) => ({ value: d.kind as string, label: d.title ?? d.kind }))
  const filtered = !!search.kind || !!search.db

  return (
    <Page width="wide" fill>
      <PageHeader title={t('examples.title')} subtitle={t('examples.subtitle')} icon="book-open" />
      {config.isSuccess && !enabled && <Alert severity="info" title={t('examples.disabled')} />}
      <div className={styles.filters}>
        <RadioButtonGroup<string>
          value={search.kind ?? ''}
          onChange={(v) => onSearchChange({ kind: (v || undefined) as ExamplesSearch['kind'] })}
          options={[
            { value: '', label: t('examples.filters.all') },
            ...KINDS.map((k) => ({ value: k, label: t(`examples.kind.${k}`), icon: KIND_ICON[k] })),
          ]}
        />
        <Select<string>
          width={28}
          isClearable
          placeholder={t('examples.filters.anyDb')}
          options={dbOptions}
          value={search.db ?? null}
          onChange={(o) => onSearchChange({ db: o?.value || undefined })}
          aria-label={t('examples.filters.db')}
        />
      </div>
      <PageFill scroll>
        {list.isPending ? (
          <LoadingPlaceholder text={t('common.misc.loading')} />
        ) : rows.length === 0 ? (
          <EmptyState
            variant="not-found"
            message={filtered ? t('examples.empty.title') : t('examples.empty.none')}
            image={<Icon name="book-open" size="xxxl" />}
            button={
              filtered ? (
                <Button
                  variant="secondary"
                  icon="times"
                  onClick={() => onSearchChange({ kind: undefined, db: undefined })}
                >
                  {t('common.empty.clearFilters')}
                </Button>
              ) : undefined
            }
          />
        ) : (
          <div className={styles.grid}>
            {rows.map((e) => {
              const facts = factsOf(e, t, engineTitle)
              const tags = Object.entries(e.tags ?? {}).map(([k, v]) => `${k}: ${v}`)
              return (
                <Card key={e.id} className={styles.card}>
                  <Card.Heading>{e.title}</Card.Heading>
                  <Card.Figure>
                    <Icon name={KIND_ICON[e.kind]} size="xl" className={styles.figure} />
                  </Card.Figure>
                  <Card.Meta>
                    {
                      [
                        t(`examples.kindOne.${e.kind}`),
                        e.db_kind ? engineTitle(e.db_kind) : undefined,
                      ].filter(Boolean) as string[]
                    }
                  </Card.Meta>
                  <Card.Description>
                    {e.description}
                    <dl className={styles.facts}>
                      {facts.map((f) => (
                        <FactRow key={f.label} fact={f} lineClass={styles.line} />
                      ))}
                    </dl>
                  </Card.Description>
                  {tags.length > 0 && (
                    <Card.Tags>
                      <TagList tags={tags} />
                    </Card.Tags>
                  )}
                  <Card.Actions>
                    {e.kind === 'test' && (
                      <Button
                        icon="play"
                        disabled={!enabled}
                        tooltip={!enabled ? t('examples.table.disabled') : undefined}
                        onClick={() => setQuick(e)}
                      >
                        {t('examples.actions.quickRun')}
                      </Button>
                    )}
                    <Button
                      variant="secondary"
                      icon="copy"
                      disabled={!enabled}
                      tooltip={!enabled ? t('examples.table.disabled') : undefined}
                      onClick={() => setClone(e)}
                    >
                      {t('examples.actions.clone')}
                    </Button>
                  </Card.Actions>
                </Card>
              )
            })}
          </div>
        )}
      </PageFill>
      {clone && (
        <CloneModal
          example={clone}
          tenants={me.tenants}
          defaultSlug={defaultSlug}
          onClose={() => setClone(undefined)}
        />
      )}
      {quick && (
        <QuickRunModal
          example={quick}
          tenants={me.tenants}
          defaultSlug={defaultSlug}
          onClose={() => setQuick(undefined)}
        />
      )}
    </Page>
  )
}

function FactRow({ fact, lineClass }: { fact: Fact; lineClass: string }) {
  return (
    <>
      <dt>{fact.label}</dt>
      <dd>
        {fact.value}
        {fact.lines?.map((l) => (
          <span key={l} className={lineClass}>
            {l}
          </span>
        ))}
      </dd>
    </>
  )
}

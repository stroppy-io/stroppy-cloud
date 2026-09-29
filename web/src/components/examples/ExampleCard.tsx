import type { Example } from '@api/types'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Badge, Button, Icon, Text, useStyles2 } from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import type { TFunction } from 'i18next'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

// One example in the gallery: everything it would create is readable on the card itself —
// engine and topology, every workload segment as a row, sizes, a suite's tests and axes.

export const KIND_ICON: Record<Example['kind'], IconName> = {
  database: 'database',
  workload: 'bolt',
  test: 'vial',
  suite: 'layer-group',
}

type Params = Record<string, unknown>
interface DbSpec {
  kind?: string
  version?: string
  params?: Params
}
interface Segment {
  name?: string
  run?: { duration?: string; vus?: number; iterations?: number }
  workload?: { script?: string; scale_factor?: number }
}
interface WlSpec {
  protocol?: string
  stroppy_version?: string
  segments?: Segment[]
}
type Sizes = Record<string, { size?: string }>

const getStyles = (theme: GrafanaTheme2) => ({
  card: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
    padding: theme.spacing(2.5),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.secondary,
    minWidth: 0,
    transition: theme.transitions.create(['border-color']),
    '&:hover': { borderColor: theme.colors.border.medium },
  }),
  head: css({ display: 'flex', gap: theme.spacing(1.5), alignItems: 'flex-start' }),
  kindIcon: css({
    flex: 'none',
    width: 36,
    height: 36,
    borderRadius: theme.shape.radius.default,
    display: 'grid',
    placeItems: 'center',
    background: theme.colors.background.canvas,
    border: `1px solid ${theme.colors.border.weak}`,
    color: theme.colors.text.secondary,
  }),
  headText: css({ minWidth: 0, display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5) }),
  title: css({
    margin: 0,
    fontSize: theme.typography.h5.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: theme.typography.h5.lineHeight,
  }),
  kindLine: css({
    display: 'flex',
    gap: theme.spacing(0.75),
    alignItems: 'center',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  dot: css({ color: theme.colors.text.disabled }),
  desc: css({
    margin: 0,
    color: theme.colors.text.secondary,
    display: '-webkit-box',
    WebkitBoxOrient: 'vertical',
    WebkitLineClamp: 2,
    overflow: 'hidden',
  }),
  body: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  block: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.75) }),
  blockLabel: css({
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: 0.4,
  }),
  line: css({
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: theme.spacing(0.75),
  }),
  strong: css({ fontWeight: theme.typography.fontWeightMedium }),
  muted: css({ color: theme.colors.text.secondary }),
  table: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1.2fr) minmax(0, 1.4fr) auto auto auto',
    columnGap: theme.spacing(2),
    rowGap: theme.spacing(0.5),
    fontSize: theme.typography.bodySmall.fontSize,
    padding: theme.spacing(1, 1.5),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.canvas,
    border: `1px solid ${theme.colors.border.weak}`,
  }),
  th: css({ color: theme.colors.text.secondary }),
  td: css({ minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }),
  num: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    textAlign: 'right',
    whiteSpace: 'nowrap',
  }),
  mono: css({ fontFamily: theme.typography.fontFamilyMonospace }),
  foot: css({
    marginTop: 'auto',
    paddingTop: theme.spacing(1.5),
    borderTop: `1px solid ${theme.colors.border.weak}`,
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
  }),
  tags: css({ display: 'flex', gap: theme.spacing(0.5), flexWrap: 'wrap', minWidth: 0 }),
  actions: css({ marginLeft: 'auto', display: 'flex', gap: theme.spacing(1) }),
})

function Block({ label, children }: { label: string; children: ReactNode }) {
  const styles = useStyles2(getStyles)
  return (
    <div className={styles.block}>
      <span className={styles.blockLabel}>{label}</span>
      {children}
    </div>
  )
}

// «PostgreSQL 17» + topology params as chips (replicas 1, haproxy 1, nodes 3).
function DatabaseView({ db, engineTitle }: { db: DbSpec; engineTitle: (k: string) => string }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const params = Object.entries(db.params ?? {}).filter(
    ([k, v]) => k !== 'version' && v !== undefined && v !== null && v !== false
  )
  return (
    <div className={styles.line}>
      <span className={styles.strong}>
        {engineTitle(db.kind ?? '')} {db.version}
      </span>
      {params.length === 0 ? (
        <Badge color="darkgrey" text={t('examples.card.singleNode')} />
      ) : (
        params.map(([k, v]) => (
          <Badge key={k} color="darkgrey" text={v === true ? k : `${k} ${String(v)}`} />
        ))
      )}
    </div>
  )
}

// Protocol line + segments as a small aligned table: name · script · SF · VU · length.
function WorkloadView({ wl, t }: { wl: WlSpec; t: TFunction }) {
  const styles = useStyles2(getStyles)
  const segs = wl.segments ?? []
  return (
    <>
      <div className={styles.line}>
        <span className={styles.strong}>{wl.protocol}</span>
        {wl.stroppy_version && <span className={styles.muted}>stroppy {wl.stroppy_version}</span>}
      </div>
      {segs.length > 0 && (
        <div className={styles.table}>
          <span className={styles.th}>{t('examples.card.segment')}</span>
          <span className={styles.th}>{t('examples.card.script')}</span>
          <span className={cx(styles.th, styles.num)}>SF</span>
          <span className={cx(styles.th, styles.num)}>VU</span>
          <span className={cx(styles.th, styles.num)}>{t('examples.card.length')}</span>
          {segs.map((s, i) => (
            <SegmentRow key={`${s.name}-${i}`} s={s} t={t} />
          ))}
        </div>
      )}
    </>
  )
}

function SegmentRow({ s, t }: { s: Segment; t: TFunction }) {
  const styles = useStyles2(getStyles)
  const run = s.run ?? {}
  const length = run.duration
    ? formatDuration(run.duration)
    : run.iterations
      ? t('examples.card.iterations', { count: run.iterations })
      : '—'
  return (
    <>
      <span className={styles.td} title={s.name}>
        {s.name ?? '—'}
      </span>
      <span className={cx(styles.td, styles.mono)} title={s.workload?.script}>
        {s.workload?.script ?? '—'}
      </span>
      <span className={styles.num}>{s.workload?.scale_factor ?? '—'}</span>
      <span className={styles.num}>{run.vus ?? '—'}</span>
      <span className={styles.num}>{length}</span>
    </>
  )
}

function SizesView({ sizes }: { sizes: Sizes | undefined }) {
  const styles = useStyles2(getStyles)
  const e = Object.entries(sizes ?? {})
  if (!e.length) return <span className={styles.muted}>—</span>
  return (
    <div className={styles.line}>
      {e.map(([role, v]) => (
        <Badge key={role} color="blue" text={`${role} ${v?.size ?? '?'}`} />
      ))}
    </div>
  )
}

export function ExampleCard({
  example: e,
  engineTitle,
  enabled,
  onQuickRun,
  onClone,
}: {
  example: Example
  engineTitle: (k: string) => string
  enabled: boolean
  onQuickRun: () => void
  onClone: () => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const spec = ((e.document as { spec?: unknown } | undefined)?.spec ?? {}) as Record<
    string,
    unknown
  >
  const tags = Object.entries(e.tags ?? {}).map(([k, v]) => `${k}: ${v}`)

  let body: ReactNode = null
  switch (e.kind) {
    case 'database':
      body = (
        <Block label={t('examples.card.engine')}>
          <DatabaseView db={spec as DbSpec} engineTitle={engineTitle} />
        </Block>
      )
      break
    case 'workload':
      body = (
        <Block label={t('examples.card.workload')}>
          <WorkloadView wl={spec as WlSpec} t={t} />
        </Block>
      )
      break
    case 'test': {
      const db = (spec.database as { inline?: DbSpec } | undefined)?.inline
      const wl = (spec.workload as { inline?: WlSpec } | undefined)?.inline
      body = (
        <>
          {db && (
            <Block label={t('examples.card.database')}>
              <DatabaseView db={db} engineTitle={engineTitle} />
            </Block>
          )}
          {wl && (
            <Block label={t('examples.card.workload')}>
              <WorkloadView wl={wl} t={t} />
            </Block>
          )}
          <Block label={t('examples.card.sizes')}>
            <SizesView sizes={spec.sizes as Sizes} />
          </Block>
        </>
      )
      break
    }
    case 'suite': {
      const tests = (spec.tests ?? []) as {
        inline_name?: string
        inline?: { database_inline?: DbSpec; workload_inline?: WlSpec }
      }[]
      const variants = (spec.axes as { sizes?: Sizes[] } | undefined)?.sizes ?? []
      body = (
        <>
          <Block label={t('examples.card.tests', { count: tests.length })}>
            {tests.map((x, i) => (
              <div key={`${x.inline_name}-${i}`} className={styles.line}>
                <Icon name="vial" size="sm" className={styles.muted} />
                <span className={styles.strong}>{x.inline_name}</span>
                {x.inline?.database_inline && (
                  <span className={styles.muted}>
                    {engineTitle(x.inline.database_inline.kind ?? '')}{' '}
                    {x.inline.database_inline.version}
                  </span>
                )}
                {(x.inline?.workload_inline?.segments ?? []).map((s, j) => (
                  <Badge key={j} color="darkgrey" text={s.workload?.script ?? s.name ?? '—'} />
                ))}
              </div>
            ))}
          </Block>
          <Block label={t('examples.card.sizeVariants', { count: variants.length })}>
            {variants.length ? (
              variants.map((v, i) => <SizesView key={i} sizes={v} />)
            ) : (
              <span className={styles.muted}>—</span>
            )}
          </Block>
          <Block label={t('examples.card.concurrency')}>
            <span>{String((spec.concurrency as number | undefined) ?? 1)}</span>
          </Block>
        </>
      )
      break
    }
  }

  return (
    <article className={styles.card} aria-label={e.title}>
      <div className={styles.head}>
        <span className={styles.kindIcon}>
          <Icon name={KIND_ICON[e.kind]} size="lg" />
        </span>
        <div className={styles.headText}>
          <h3 className={styles.title}>{e.title}</h3>
          <span className={styles.kindLine}>
            {t(`examples.kindOne.${e.kind}`)}
            {e.db_kind && (
              <>
                <span className={styles.dot}>·</span>
                {engineTitle(e.db_kind)}
              </>
            )}
          </span>
        </div>
      </div>
      {e.description && (
        <p className={styles.desc} title={e.description}>
          {e.description}
        </p>
      )}
      <div className={styles.body}>{body}</div>
      <div className={styles.foot}>
        <div className={styles.tags}>
          {tags.map((tag) => (
            <Text key={tag} variant="bodySmall" color="secondary">
              #{tag}
            </Text>
          ))}
        </div>
        <div className={styles.actions}>
          <Button
            size="sm"
            variant="secondary"
            icon="copy"
            disabled={!enabled}
            tooltip={!enabled ? t('examples.table.disabled') : t('examples.actions.clone')}
            onClick={onClone}
          >
            {t('examples.actions.cloneShort')}
          </Button>
          {e.kind === 'test' && (
            <Button
              size="sm"
              icon="play"
              disabled={!enabled}
              tooltip={!enabled ? t('examples.table.disabled') : undefined}
              onClick={onQuickRun}
            >
              {t('examples.actions.quickRun')}
            </Button>
          )}
        </div>
      </div>
    </article>
  )
}

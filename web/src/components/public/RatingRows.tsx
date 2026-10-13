import type { RatingEntry } from '@api/types'
import { ProductIcon } from '@components/ProductIcon'
import { RankBadge } from '@components/RankBadge'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, useStyles2 } from '@grafana/ui'
import { formatMetric } from '@helpers/format'
import { hardwareLabel } from '@helpers/rating'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { useTranslation } from 'react-i18next'

// A short rating as cards: place, database and stand, the value. The public
// pieces show it — the landing's rating screen and the embeddable widget —
// each in its own colours (`palette`: the landing is always dark, the widget
// follows the page that embeds it).

export type RatingPalette = {
  raised: string
  line: string
  text: string
  muted: string
}

export const KIND_TITLE: Record<string, string> = {
  postgres: 'PostgreSQL',
  mysql: 'MySQL',
  mariadb: 'MariaDB',
  picodata: 'Picodata',
  ydb: 'YDB',
  ydb_managed: 'YDB Managed',
  cockroach: 'CockroachDB',
  orioledb: 'OrioleDB',
  external: 'External',
}

const getStyles = (theme: GrafanaTheme2, p: RatingPalette) => ({
  rows: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.75) }),
  row: css({
    display: 'grid',
    gridTemplateColumns: '48px minmax(0, 1fr) auto',
    alignItems: 'center',
    gap: theme.spacing(2),
    padding: theme.spacing(1.25, 1.5),
    borderRadius: 6,
    border: `1px solid ${p.line}`,
    background: p.raised,
    color: p.text,
    fontSize: theme.typography.bodySmall.fontSize,
    textDecoration: 'none',
  }),
  link: css({
    '&:hover': { borderColor: theme.colors.primary.border, textDecoration: 'none', color: p.text },
    '&:focus-visible': { outline: `2px solid ${theme.colors.primary.main}`, outlineOffset: 2 },
  }),
  rank: css({ fontSize: theme.typography.body.fontSize }),
  slot: css({
    borderStyle: 'dashed',
    background: 'none',
    color: p.muted,
  }),
  slotRank: css({ opacity: 0.45 }),
  name: css({ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }),
  meta: css({ color: p.muted, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }),
  value: css({ fontFamily: theme.typography.fontFamilyMonospace, textAlign: 'right' }),
  skeleton: css({ height: 50, borderRadius: 6, background: p.raised, opacity: 0.5 }),
  state: css({
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    gap: theme.spacing(1.5),
    textAlign: 'center',
    color: p.muted,
    padding: theme.spacing(4),
  }),
  stateTitle: css({ color: p.text, fontSize: theme.typography.h5.fontSize }),
})

export function RatingRows({
  rows,
  unit,
  palette,
  loading,
  error,
  animateKey,
  shareHref,
  slots = 0,
}: {
  rows: RatingEntry[]
  unit?: string
  palette: RatingPalette
  loading?: boolean
  error?: boolean
  /** Rows re-enter when this changes (another scenario, another filter). */
  animateKey?: string
  /** Where a row with a public report links to; rows without one are plain. */
  shareHref?: (token: string) => string
  /** Show this many places: the free ones as empty rows, so a short list still reads as a board. */
  slots?: number
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles, palette)
  const reduce = useReducedMotion() ?? false

  if (loading) {
    return (
      <div className={styles.rows}>
        {['a', 'b', 'c', 'd', 'e', 'f'].map((k) => (
          <div key={k} className={styles.skeleton} />
        ))}
      </div>
    )
  }
  if (error) {
    return (
      <div className={styles.state}>
        <Icon name="exclamation-triangle" size="xl" />
        <div className={styles.stateTitle}>{t('landing.rating.error')}</div>
      </div>
    )
  }
  if (rows.length === 0) {
    return (
      <div className={styles.state}>
        <ProductIcon name="trophy" size="xxl" />
        <div className={styles.stateTitle}>{t('landing.rating.emptyTitle')}</div>
        <div>{t('landing.rating.emptyBody')}</div>
      </div>
    )
  }
  return (
    <div className={styles.rows}>
      <AnimatePresence initial={false} mode="popLayout">
        {rows.map((e, i) => {
          const meta = [e.topology_label, hardwareLabel(t, e.hardware), e.provider_kind].filter(
            Boolean
          )
          const href = e.share_token && shareHref ? shareHref(e.share_token) : undefined
          const body = (
            <>
              <RankBadge rank={e.rank} size="lg" className={styles.rank} />
              <div style={{ minWidth: 0 }}>
                <div className={styles.name}>
                  {KIND_TITLE[e.db_kind] ?? e.db_kind} {e.db_version}
                </div>
                <div className={styles.meta}>{meta.join(' · ')}</div>
              </div>
              <span className={styles.value}>{formatMetric(e.value, e.unit ?? unit)}</span>
            </>
          )
          return (
            <motion.div
              key={`${animateKey ?? ''}-${e.rank}-${e.run_at}`}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0 }}
              transition={reduce ? { duration: 0 } : { duration: 0.25, delay: i * 0.03 }}
            >
              {href ? (
                <a
                  className={cx(styles.row, styles.link)}
                  href={href}
                  target="_blank"
                  rel="noreferrer"
                >
                  {body}
                </a>
              ) : (
                <div className={styles.row}>{body}</div>
              )}
            </motion.div>
          )
        })}
      </AnimatePresence>
      {Array.from({ length: Math.max(0, slots - rows.length) }, (_, i) => rows.length + i + 1).map(
        (place) => (
          <div key={`slot-${place}`} className={cx(styles.row, styles.slot)}>
            <RankBadge rank={place} size="lg" className={cx(styles.rank, styles.slotRank)} />
            <span>{t('landing.rating.openSpot')}</span>
            <span />
          </div>
        )
      )}
    </div>
  )
}

import { ratingQueries } from '@api/queries/results'
import { AppLink } from '@app/AppLink'
import { type RatingPalette, RatingRows } from '@components/public/RatingRows'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, useStyles2 } from '@grafana/ui'
import { scenarioLabel } from '@helpers/rating'
import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { brand } from '@/styles/brand'

// The landing's rating screen: the real public rating, a short top per workload
// scenario (tps of different scenarios never share a list), right after the
// story ends with «your run takes its place». The full list with every filter
// is /leaderboard.

const TOP = 8
const PALETTE: RatingPalette = {
  raised: brand.raised,
  line: brand.line,
  text: brand.text,
  muted: brand.muted,
}

const getStyles = (theme: GrafanaTheme2) => ({
  slide: css({
    position: 'absolute',
    inset: 0,
    maxWidth: 1360,
    margin: '0 auto',
    padding: theme.spacing(0, 8),
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1.3fr)',
    alignItems: 'start',
    alignContent: 'center',
    gap: theme.spacing(8),
    [theme.breakpoints.down('lg')]: { gridTemplateColumns: '1fr', gap: theme.spacing(3) },
    [theme.breakpoints.down('md')]: { padding: theme.spacing(0, 2) },
  }),
  heading: css({
    margin: 0,
    fontSize: 'clamp(28px, 2.6vw, 38px)',
    lineHeight: 1.1,
    fontWeight: 700,
    letterSpacing: '-0.025em',
    color: brand.text,
  }),
  sub: css({ marginTop: theme.spacing(1.5), color: brand.muted, maxWidth: 460, lineHeight: 1.55 }),
  chips: css({
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(1),
    marginTop: theme.spacing(4),
    padding: 0,
    border: 'none',
  }),
  chip: css({
    height: 30,
    padding: theme.spacing(0, 1.5),
    borderRadius: theme.shape.radius.pill,
    border: `1px solid ${brand.line}`,
    background: 'none',
    color: brand.muted,
    cursor: 'pointer',
    fontSize: theme.typography.bodySmall.fontSize,
    '&:hover': { color: brand.text, borderColor: brand.muted },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 2 },
  }),
  chipActive: css({
    color: brand.text,
    background: brand.raised,
    borderColor: theme.colors.primary.border,
  }),
  actions: css({
    display: 'flex',
    gap: theme.spacing(2),
    marginTop: theme.spacing(5),
    flexWrap: 'wrap',
  }),
  primary: css({
    display: 'inline-flex',
    alignItems: 'center',
    height: 44,
    padding: theme.spacing(0, 2.5),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.primary.main,
    color: theme.colors.primary.contrastText,
    fontWeight: theme.typography.fontWeightMedium,
    '&:hover': {
      color: theme.colors.primary.contrastText,
      textDecoration: 'none',
      background: theme.colors.primary.shade,
    },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 3 },
  }),
  secondary: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    height: 44,
    padding: theme.spacing(0, 2.5),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${brand.line}`,
    color: brand.text,
    '&:hover': { color: brand.text, textDecoration: 'none', borderColor: brand.muted },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 3 },
  }),
  panel: css({
    display: 'flex',
    flexDirection: 'column',
    border: `1px solid ${brand.line}`,
    borderRadius: 10,
    background: brand.panel,
    padding: theme.spacing(3),
    minHeight: 480,
  }),
  panelHead: css({
    display: 'flex',
    alignItems: 'baseline',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(2),
    color: brand.text,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  unit: css({ color: brand.muted, fontWeight: theme.typography.fontWeightRegular }),
})

export function RatingScreen() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const [picked, setPicked] = useState<string | undefined>(undefined)
  const facets = useInfiniteQuery({ ...ratingQueries.global({ limit: 1 }) })
  const scenarios = facets.data?.pages[0]?.scenarios ?? []
  // The first scenario until the reader picks one: a top never mixes them.
  const scenario = picked ?? scenarios[0]
  const list = useInfiniteQuery({
    ...ratingQueries.global({ scenario: scenario ? [scenario] : undefined, limit: TOP }),
    enabled: facets.isSuccess,
    placeholderData: keepPreviousData,
  })
  const page = list.data?.pages[0]
  const rows = (page?.data ?? []).filter((r) => r.scenario === scenario).slice(0, TOP)

  return (
    <section className={styles.slide} aria-label={t('landing.rating.title')}>
      <div>
        <h2 className={styles.heading}>{t('landing.rating.title')}</h2>
        <p className={styles.sub}>{t('landing.rating.sub')}</p>
        {scenarios.length > 0 && (
          <fieldset className={styles.chips} aria-label={t('results.rating.filters.scenario')}>
            {scenarios.map((s) => (
              <button
                key={s}
                type="button"
                className={cx(styles.chip, scenario === s && styles.chipActive)}
                aria-pressed={scenario === s}
                onClick={() => setPicked(s)}
              >
                {scenarioLabel(s)}
              </button>
            ))}
          </fieldset>
        )}
        <div className={styles.actions}>
          <AppLink to="/leaderboard" plain className={styles.secondary}>
            {t('landing.rating.all')}
            <Icon name="arrow-right" size="sm" />
          </AppLink>
          <AppLink to="/register" plain className={styles.primary}>
            {t('landing.rating.join')}
          </AppLink>
        </div>
      </div>

      <div className={styles.panel}>
        <div className={styles.panelHead}>
          {page?.metric.title ?? t('landing.rating.metric')}
          {page?.metric.unit && <span className={styles.unit}>{page.metric.unit}</span>}
        </div>
        <RatingRows
          rows={rows}
          unit={page?.metric.unit}
          palette={PALETTE}
          loading={facets.isPending || (!!scenario && list.isPending)}
          error={facets.isError || list.isError}
          animateKey={scenario ?? 'none'}
          shareHref={(token) => `/s/${token}`}
          slots={TOP}
        />
      </div>
    </section>
  )
}

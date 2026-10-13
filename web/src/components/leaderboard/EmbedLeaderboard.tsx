import { ratingQueries } from '@api/queries/results'
import { type RatingPalette, RatingRows } from '@components/public/RatingRows'
import { css } from '@emotion/css'
import { type GrafanaTheme2, ThemeContext } from '@grafana/data'
import { Icon, useStyles2, useTheme2 } from '@grafana/ui'
import { useInfiniteQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { getAppTheme } from '@/styles/theme'

// The rating for an <iframe> elsewhere (stroppy.io): no chrome, the host
// picks the theme, the scenario and how many rows; every row and the footer
// open Stroppy Cloud in a new tab.

export const embedSearchSchema = z.object({
  theme: z.enum(['dark', 'light']).default('dark').catch('dark'),
  scenario: z.string().optional().catch(undefined),
  metric: z.string().default('tps').catch('tps'),
  period: z.enum(['7d', '30d', '90d', '1y', 'all']).default('all').catch('all'),
  limit: z.coerce.number().int().min(1).max(50).default(10).catch(10),
})
export type EmbedSearch = z.infer<typeof embedSearchSchema>

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    minHeight: '100vh',
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1.5),
    padding: theme.spacing(2),
    background: theme.colors.background.primary,
    color: theme.colors.text.primary,
    fontFamily: theme.typography.fontFamily,
  }),
  head: css({
    display: 'flex',
    alignItems: 'baseline',
    gap: theme.spacing(1),
    fontWeight: theme.typography.fontWeightMedium,
  }),
  unit: css({ color: theme.colors.text.secondary, fontWeight: theme.typography.fontWeightRegular }),
  foot: css({
    marginTop: 'auto',
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    alignSelf: 'flex-end',
    color: theme.colors.text.link,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
})

function Board({ search }: { search: EmbedSearch }) {
  const { t } = useTranslation()
  const theme = useTheme2()
  const styles = useStyles2(getStyles)
  const palette = useMemo<RatingPalette>(
    () => ({
      raised: theme.colors.background.secondary,
      line: theme.colors.border.weak,
      text: theme.colors.text.primary,
      muted: theme.colors.text.secondary,
    }),
    [theme]
  )
  const list = useInfiniteQuery(
    ratingQueries.global({
      metric: search.metric,
      scenario: search.scenario ? [search.scenario] : undefined,
      period: search.period,
      limit: search.limit,
    })
  )
  const page = list.data?.pages[0]
  const origin = window.location.origin
  const full = new URL('/leaderboard', origin)
  if (search.scenario) full.searchParams.set('scenario', JSON.stringify([search.scenario]))
  return (
    <div className={styles.root}>
      <div className={styles.head}>
        {page?.metric.title ?? t('landing.rating.metric')}
        {page?.metric.unit && <span className={styles.unit}>{page.metric.unit}</span>}
      </div>
      <RatingRows
        rows={page?.data.slice(0, search.limit) ?? []}
        unit={page?.metric.unit}
        palette={palette}
        loading={list.isPending}
        error={list.isError}
        shareHref={(token) => `${origin}/s/${token}`}
      />
      <a className={styles.foot} href={full.toString()} target="_blank" rel="noreferrer">
        {t('landing.rating.embedFooter')}
        <Icon name="external-link-alt" size="xs" />
      </a>
    </div>
  )
}

export function EmbedLeaderboard({ search }: { search: EmbedSearch }) {
  const theme = useMemo(() => getAppTheme(search.theme), [search.theme])
  return (
    <ThemeContext.Provider value={theme}>
      <Board search={search} />
    </ThemeContext.Provider>
  )
}

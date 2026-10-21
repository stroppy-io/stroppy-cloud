import { meQueries } from '@api/queries/me'
import { ratingQueries } from '@api/queries/results'
import { type RatingPalette, RatingRows } from '@components/public/RatingRows'
import { RatingBoard, type RatingFilters, toRatingQuery } from '@components/results/RatingBoard'
import { css, cx } from '@emotion/css'
import { type GrafanaTheme2, ThemeContext } from '@grafana/data'
import { Icon, useStyles2, useTheme2 } from '@grafana/ui'
import { type EmbedTheme, useEmbedBridge, useEmbedNavigationGuard } from '@hooks/useEmbedBridge'
import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { getAppTheme } from '@/styles/theme'

// The rating for an <iframe> elsewhere (stroppy.io): no chrome, the host
// picks the theme, the scenario and how many rows; every row and the footer
// open Stroppy Cloud in a new tab. With filters=1 it is the whole public
// rating with its filter bar, the filters in the frame's URL. The frame
// tells its page its height and takes theme changes by postMessage
// (useEmbedBridge; protocol in web/AGENTS.md §6a).

const list = z.array(z.string()).optional().catch(undefined)
export const embedSearchSchema = z.object({
  theme: z.enum(['dark', 'light']).default('dark').catch('dark'),
  /** One scenario (the simple widget) or several (filters=1, as /leaderboard writes it). */
  scenario: z
    .union([z.array(z.string()), z.string()])
    .optional()
    .catch(undefined),
  metric: z.string().default('tps').catch('tps'),
  period: z.enum(['7d', '30d', '90d', '1y', 'all']).default('all').catch('all'),
  limit: z.coerce.number().int().min(1).max(50).default(10).catch(10),
  /** 1 = the full board with its filters. */
  filters: z.coerce.number().int().optional().catch(undefined),
  /** transparent = no background of its own (the page shows through). */
  bg: z.enum(['transparent']).optional().catch(undefined),
  kind: list,
  version: list,
  provider: list,
  stroppy: list,
  hardware: list,
})
export type EmbedSearch = z.infer<typeof embedSearchSchema>

const asList = (v: string | string[] | undefined) =>
  v === undefined ? undefined : Array.isArray(v) ? v : [v]

const getStyles = (theme: GrafanaTheme2) => ({
  // No minimum height: the frame takes this height (a 100vh minimum would
  // feed the frame's own height back into it).
  root: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1.5),
    padding: theme.spacing(2),
    background: theme.colors.background.primary,
    color: theme.colors.text.primary,
    fontFamily: theme.typography.fontFamily,
  }),
  transparent: css({ background: 'transparent' }),
  // Out of the column's flow (no extra gap); the menus inside are fixed-positioned.
  portal: css({ position: 'absolute', top: 0, left: 0 }),
  rootBox: css({ position: 'relative' }),
  head: css({
    display: 'flex',
    alignItems: 'baseline',
    gap: theme.spacing(1),
    fontWeight: theme.typography.fontWeightMedium,
  }),
  unit: css({ color: theme.colors.text.secondary, fontWeight: theme.typography.fontWeightRegular }),
  foot: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    alignSelf: 'flex-end',
    color: theme.colors.text.link,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
})

function Rows({ search, onReady }: { search: EmbedSearch; onReady: () => void }) {
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
  const scenario = asList(search.scenario)?.[0]
  const rating = useInfiniteQuery(
    ratingQueries.global({
      metric: search.metric,
      scenario: scenario ? [scenario] : undefined,
      period: search.period,
      limit: search.limit,
    })
  )
  const settled = !rating.isPending
  useEffect(() => {
    if (settled) onReady()
  }, [settled, onReady])
  const page = rating.data?.pages[0]
  const origin = window.location.origin
  const full = new URL('/leaderboard', origin)
  if (scenario) full.searchParams.set('scenario', JSON.stringify([scenario]))
  return (
    <>
      <div className={styles.head}>
        {page?.metric.title ?? t('landing.rating.metric')}
        {page?.metric.unit && <span className={styles.unit}>{page.metric.unit}</span>}
      </div>
      <RatingRows
        rows={page?.data.slice(0, search.limit) ?? []}
        unit={page?.metric.unit}
        palette={palette}
        loading={rating.isPending}
        error={rating.isError}
        shareHref={(token) => `${origin}/s/${token}`}
      />
      <a className={styles.foot} href={full.toString()} target="_blank" rel="noreferrer">
        {t('landing.rating.embedFooter')}
        <Icon name="external-link-alt" size="xs" />
      </a>
    </>
  )
}

function Board({
  search,
  onSearchChange,
  onReady,
}: {
  search: EmbedSearch
  onSearchChange: (next: Partial<EmbedSearch>) => void
  onReady: () => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const filters: RatingFilters = {
    metric: search.metric,
    period: search.period,
    kind: search.kind,
    version: search.version,
    provider: search.provider,
    stroppy: search.stroppy,
    scenario: asList(search.scenario),
    hardware: search.hardware,
  }
  // The same query as the board's (shared cache): ready once it settled.
  const rating = useInfiniteQuery(ratingQueries.global(toRatingQuery(filters)))
  const settled = !rating.isPending
  useEffect(() => {
    if (settled) onReady()
  }, [settled, onReady])
  const full = new URL('/leaderboard', window.location.origin)
  for (const [k, v] of Object.entries(filters))
    if (v !== undefined && !(k === 'metric' && v === 'tps') && !(k === 'period' && v === 'all'))
      full.searchParams.set(k, Array.isArray(v) ? JSON.stringify(v) : v)
  return (
    <>
      <RatingBoard
        scope="global"
        catalog="public"
        search={filters}
        onSearchChange={onSearchChange}
        embedded
      />
      <a className={styles.foot} href={full.toString()} target="_blank" rel="noreferrer">
        {t('landing.rating.embedFooter')}
        <Icon name="external-link-alt" size="xs" />
      </a>
    </>
  )
}

function Embed({
  search,
  onSearchChange,
  onTheme,
}: {
  search: EmbedSearch
  onSearchChange: (next: Partial<EmbedSearch>) => void
  onTheme: (t: EmbedTheme) => void
}) {
  const styles = useStyles2(getStyles)
  const root = useRef<HTMLDivElement>(null)
  const config = useQuery(meQueries.publicConfig())
  const origins = useMemo(() => config.data?.embed_origins ?? [], [config.data])
  const [ready, setReady] = useState(false)
  const markReady = useMemo(() => () => setReady(true), [])
  useEmbedBridge(root, { ready, origins, onTheme })
  useEmbedNavigationGuard()
  const transparent = search.bg === 'transparent'
  useEffect(() => {
    if (!transparent) return
    // The app's global styles paint the page; the host page shows through
    // instead. A browser paints an opaque backdrop under a frame whose
    // color-scheme differs from its <iframe> element's (the app says dark),
    // so the page drops to `normal`; the host gives the <iframe> the same.
    for (const el of [document.documentElement, document.body]) {
      el.style.setProperty('background', 'transparent', 'important')
      el.style.setProperty('color-scheme', 'normal', 'important')
    }
  }, [transparent])
  return (
    <div ref={root} className={cx(styles.root, styles.rootBox, transparent && styles.transparent)}>
      {search.filters === 1 ? (
        <Board search={search} onSearchChange={onSearchChange} onReady={markReady} />
      ) : (
        <Rows search={search} onReady={markReady} />
      )}
      {/* Dropdowns render here, inside the frame's measured root (Grafana's
          Portal looks this id up), not at the end of <body>. */}
      <div id="grafana-portal-container" className={styles.portal} />
    </div>
  )
}

export function EmbedLeaderboard({
  search,
  onSearchChange,
}: {
  search: EmbedSearch
  onSearchChange: (next: Partial<EmbedSearch>) => void
}) {
  // The page may switch the theme later by postMessage, without a reload.
  const [override, setOverride] = useState<EmbedTheme>()
  const name = override ?? search.theme
  const theme = useMemo(() => getAppTheme(name), [name])
  return (
    <ThemeContext.Provider value={theme}>
      <Embed search={search} onSearchChange={onSearchChange} onTheme={setOverride} />
    </ThemeContext.Provider>
  )
}

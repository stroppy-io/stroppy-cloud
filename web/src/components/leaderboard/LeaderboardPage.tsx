import { meQueries } from '@api/queries/me'
import { AppLink } from '@app/AppLink'
import { Page } from '@app/Page'
import { SiteBar } from '@components/public/SiteBar'
import { RatingBoard, type RatingFilters } from '@components/results/RatingBoard'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, useStyles2 } from '@grafana/ui'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { brandOf } from '@/styles/brand'

// The public rating: the whole list with every filter (in the URL), for
// guests. Dark like the landing it comes from, whatever theme the viewer
// set inside the app.

const getStyles = (theme: GrafanaTheme2) => {
  const brand = brandOf(theme)
  return {
    root: css({
      height: '100vh',
      display: 'flex',
      flexDirection: 'column',
      background: brand.ink,
      color: brand.text,
    }),
    main: css({ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }),
    page: css({ maxWidth: 1360 }),
    intro: css({
      display: 'flex',
      alignItems: 'flex-end',
      gap: theme.spacing(3),
      flexWrap: 'wrap',
      margin: theme.spacing(3, 0, 2),
    }),
    heading: css({
      margin: 0,
      fontSize: 'clamp(26px, 2.4vw, 34px)',
      lineHeight: 1.1,
      fontWeight: 700,
      letterSpacing: '-0.025em',
    }),
    sub: css({ marginTop: theme.spacing(1), color: brand.muted, maxWidth: 640, lineHeight: 1.5 }),
    join: css({
      marginLeft: 'auto',
      display: 'inline-flex',
      alignItems: 'center',
      height: 40,
      padding: theme.spacing(0, 2),
      borderRadius: theme.shape.radius.default,
      background: theme.colors.primary.main,
      color: theme.colors.primary.contrastText,
      fontWeight: theme.typography.fontWeightMedium,
      '&:hover': {
        color: theme.colors.primary.contrastText,
        textDecoration: 'none',
        background: theme.colors.primary.shade,
      },
    }),
  }
}

function Content({
  search,
  onSearchChange,
}: {
  search: RatingFilters
  onSearchChange: (next: Partial<RatingFilters>) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const config = useQuery(meQueries.publicConfig())
  const enabled = config.data?.public_rating_enabled ?? true
  return (
    <div className={styles.root}>
      <SiteBar />
      <main className={styles.main}>
        <Page width="wide" fill className={styles.page}>
          <div className={styles.intro}>
            <div>
              <h1 className={styles.heading}>{t('landing.rating.title')}</h1>
              <p className={styles.sub}>{t('landing.rating.sub')}</p>
            </div>
            <AppLink to="/register" plain className={styles.join}>
              {t('landing.rating.join')}
            </AppLink>
          </div>
          {enabled ? (
            <RatingBoard
              scope="global"
              catalog="public"
              search={search}
              onSearchChange={onSearchChange}
            />
          ) : (
            <Alert severity="info" title={t('results.rating.globalDisabled')} />
          )}
        </Page>
      </main>
    </div>
  )
}

export function LeaderboardPage(props: {
  search: RatingFilters
  onSearchChange: (next: Partial<RatingFilters>) => void
}) {
  return <Content {...props} />
}

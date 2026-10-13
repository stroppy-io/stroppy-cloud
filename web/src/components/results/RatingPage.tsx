import { meQueries } from '@api/queries/me'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Tab, TabsBar, useStyles2 } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { RatingBoard, ratingFiltersSchema } from './RatingBoard'

export { toRatingQuery } from './RatingBoard'

export const ratingSearchSchema = ratingFiltersSchema.extend({
  tab: z.enum(['tenant', 'global']).default('tenant').catch('tenant'),
})
export type RatingSearch = z.infer<typeof ratingSearchSchema>

const getStyles = (theme: GrafanaTheme2) => ({
  tabs: css({ marginBottom: theme.spacing(2) }),
})

/** The tenant's rating: its own opted-in runs, and the global one in a second tab. */
export function RatingPage({
  search,
  onSearchChange,
}: {
  search: RatingSearch
  onSearchChange: (next: Partial<RatingSearch>) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug } = useTenant()
  const config = useQuery(meQueries.publicConfig())
  const globalEnabled = config.data?.public_rating_enabled ?? false
  const tab = search.tab === 'global' && globalEnabled ? 'global' : 'tenant'

  return (
    <Page width="wide" fill>
      <PageHeader title={t('results.rating.title')} icon="star" />
      <TabsBar className={styles.tabs}>
        <Tab
          label={t('results.rating.tabs.tenant')}
          active={tab === 'tenant'}
          onChangeTab={() => onSearchChange({ tab: 'tenant' })}
        />
        <Tab
          label={t('results.rating.tabs.global')}
          active={tab === 'global'}
          icon="globe"
          onChangeTab={() => onSearchChange({ tab: 'global' })}
          tooltip={globalEnabled ? undefined : t('results.rating.globalDisabled')}
        />
      </TabsBar>
      {search.tab === 'global' && !globalEnabled && config.isSuccess && (
        <Alert severity="info" title={t('results.rating.globalDisabled')} />
      )}
      <RatingBoard
        scope={tab}
        slug={slug}
        catalog="signed-in"
        search={search}
        onSearchChange={onSearchChange}
      />
    </Page>
  )
}

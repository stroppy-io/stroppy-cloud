import { catalogQueries } from '@api/queries/catalog'
import { meQueries } from '@api/queries/me'
import { exampleQueries } from '@api/queries/results'
import type { Example } from '@api/types'
import { Page, PageFill } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Button,
  EmptyState,
  Icon,
  LoadingPlaceholder,
  RadioButtonGroup,
  Select,
  useStyles2,
} from '@grafana/ui'
import { useMe } from '@hooks/useMe'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { CloneModal } from './CloneModal'
import { ExampleCard, KIND_ICON } from './ExampleCard'
import { QuickRunModal } from './QuickRunModal'

export const examplesSearchSchema = z.object({
  kind: z.enum(['database', 'workload', 'test', 'suite']).optional().catch(undefined),
  db: z.string().optional().catch(undefined),
  tenant: z.string().optional().catch(undefined),
})
export type ExamplesSearch = z.infer<typeof examplesSearchSchema>

const KINDS: Example['kind'][] = ['test', 'suite', 'database', 'workload']

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
    gridTemplateColumns: 'repeat(auto-fill, minmax(440px, 1fr))',
    gap: theme.spacing(2),
    alignItems: 'stretch',
    paddingBottom: theme.spacing(2),
  }),
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
            {rows.map((e) => (
              <ExampleCard
                key={e.id}
                example={e}
                engineTitle={engineTitle}
                enabled={enabled}
                onQuickRun={() => setQuick(e)}
                onClone={() => setClone(e)}
              />
            ))}
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

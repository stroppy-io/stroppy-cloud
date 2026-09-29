import { catalogQueries } from '@api/queries/catalog'
import { meQueries } from '@api/queries/me'
import { exampleQueries } from '@api/queries/results'
import type { Example } from '@api/types'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { type ActivePill, DataTableToolbar } from '@components/DataTable/Toolbar'
import type { IconName } from '@grafana/data'
import { Alert } from '@grafana/ui'
import { useMe } from '@hooks/useMe'
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
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

const KINDS: Example['kind'][] = ['test', 'database', 'workload', 'suite']

const KIND_ICON: Record<Example['kind'], IconName> = {
  database: 'database',
  workload: 'rocket',
  test: 'vial',
  suite: 'apps',
}

// Example gallery: a bounded catalog list, so the table sorts on the client; kind and database
// filter on the server (`kind`, `db_kind`).
export function ExamplesPage({
  search,
  onSearchChange,
}: {
  search: ExamplesSearch
  onSearchChange: (next: Partial<ExamplesSearch>) => void
}) {
  const { t } = useTranslation()
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
  const dbOptions = (databases.data?.data ?? []).map((d) => ({
    value: d.kind as string,
    label: d.title ?? d.kind,
  }))

  const rowActions = (e: Example): RowAction[] => [
    {
      key: 'quickRun',
      label: t('examples.actions.quickRun'),
      icon: 'play',
      disabled: !enabled || e.kind !== 'test',
      disabledReason: !enabled ? t('examples.table.disabled') : t('examples.table.onlyTests'),
      onClick: () => setQuick(e),
    },
    {
      key: 'clone',
      label: t('examples.actions.clone'),
      icon: 'copy',
      disabled: !enabled,
      disabledReason: t('examples.table.disabled'),
      onClick: () => setClone(e),
    },
  ]

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure over `enabled`
  const columns = useMemo<DataTableColumn<Example>[]>(
    () => [
      col.identity<Example>({
        id: 'title',
        header: t('examples.table.example'),
        render: (e) => ({ title: e.title, icon: KIND_ICON[e.kind], subtitle: e.description }),
      }),
      col.text<Example>({
        id: 'kind',
        header: t('examples.filters.kind'),
        width: 150,
        value: (e) => t(`examples.kindOne.${e.kind}`),
        filter: {
          kind: 'checklist',
          single: true,
          options: KINDS.map((k) => ({ value: k, label: t(`examples.kindOne.${k}`) })),
          value: search.kind ? [search.kind] : undefined,
          onChange: (v) => onSearchChange({ kind: v?.[0] as ExamplesSearch['kind'] }),
        },
      }),
      col.text<Example>({
        id: 'db',
        header: t('examples.filters.db'),
        width: 170,
        value: (e) => e.db_kind,
        filter: {
          kind: 'checklist',
          single: true,
          options: dbOptions,
          value: search.db ? [search.db] : undefined,
          onChange: (v) => onSearchChange({ db: v?.[0] }),
        },
      }),
      col.tags<Example>({
        id: 'tags',
        header: t('common.fields.tags'),
        width: 260,
        items: (e) => Object.entries(e.tags ?? {}).map(([k, v]) => `${k}: ${v}`),
      }),
      col.actions<Example>({ title: (e) => e.title, actions: rowActions }),
    ],
    [t, search.kind, search.db, onSearchChange, databases.data, enabled]
  )

  const pills: ActivePill[] = [
    ...(search.kind
      ? [
          {
            key: 'kind',
            label: `${t('examples.filters.kind')}: ${t(`examples.kindOne.${search.kind}`)}`,
            onRemove: () => onSearchChange({ kind: undefined }),
          },
        ]
      : []),
    ...(search.db
      ? [
          {
            key: 'db',
            label: `${t('examples.filters.db')}: ${dbOptions.find((o) => o.value === search.db)?.label ?? search.db}`,
            onRemove: () => onSearchChange({ db: undefined }),
          },
        ]
      : []),
  ]
  const clearAll = () => onSearchChange({ kind: undefined, db: undefined })

  return (
    <Page width="wide" fill>
      <PageHeader title={t('examples.title')} subtitle={t('examples.subtitle')} icon="book-open" />
      {config.isSuccess && !enabled && <Alert severity="info" title={t('examples.disabled')} />}
      <DataTableToolbar
        pills={pills}
        onClearAll={pills.length ? clearAll : undefined}
        count={rows.length}
      />
      <DataTable<Example>
        fill
        columns={columns}
        data={rows}
        getRowId={(e) => e.id}
        clientSort
        loading={list.isPending}
        error={list.isError ? list.error : undefined}
        onRetry={() => void list.refetch()}
        filtered={pills.length > 0}
        onClearFilters={clearAll}
        empty={{ message: t('examples.empty.none') }}
      />
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

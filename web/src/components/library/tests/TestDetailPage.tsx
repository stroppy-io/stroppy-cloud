import { catalogQueries } from '@api/queries/catalog'
import { keys } from '@api/queries/keys'
import { libraryMutations, type TestRunsQuery, testQueries } from '@api/queries/library'
import { providerQueries } from '@api/queries/settings'
import type { Run, Schemas } from '@api/types'
import { AppLink } from '@app/AppLink'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { TagsCell } from '@components/DataTable/cells'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import { orderSchema } from '@components/DataTable/list-search'
import { useTablePrefs } from '@components/DataTable/prefs'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { TableSettings } from '@components/DataTable/TableSettings'
import { DataTableToolbar } from '@components/DataTable/Toolbar'
import { FavoriteButton } from '@components/FavoriteButton'
import { KeyValueList } from '@components/KeyValueList'
import {
  providerView,
  RunMetricsCell,
  RunTimeCell,
  TriggerMark,
} from '@components/runs/list/RunCells'
import { StatusBadge } from '@components/StatusBadge'
import { UserLabel } from '@components/UserAvatar'
import { css } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import {
  Badge,
  Button,
  Icon,
  IconButton,
  Stack,
  Switch,
  Tab,
  TabsBar,
  Text,
  Tooltip,
  useStyles2,
} from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { useTenant } from '@hooks/useTenant'
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  useSuspenseQuery,
} from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { copyToClipboard, FavoriteMark } from '../shared/cells'
import { DiffPanel } from '../shared/DiffPanel'
import { EntityAbout, RenameModal } from '../shared/EntityAbout'
import { EntityActionsMenu } from '../shared/EntityActionsMenu'
import { Panel, WithSidebar } from '../shared/Panel'
import { RequirementsTable } from '../shared/RequirementsTable'
import { TopologyPreview } from '../shared/TopologyPreview'
import { TpsTrend } from '../shared/TpsTrend'
import { UsagesList } from '../shared/UsagesList'
import { ValidationIssues } from '../shared/ValidationIssues'
import { LaunchDrawer } from './LaunchDrawer'

type Test = Schemas['Test']

// Server sort keys of listTestRuns (the listRuns set; the endpoint defaults to created_at).
const TEST_RUN_SORT_KEYS = [
  'default',
  'started_at',
  'finished_at',
  'duration',
  'tps',
  'qps',
  'p50',
  'p99',
  'errors',
  'status',
  'name',
  'db_kind',
  'workload',
  'topology',
  'provider',
  'trigger',
  'author',
  'created_at',
  'updated_at',
] as const satisfies readonly NonNullable<TestRunsQuery['sort']>[]

// `rsort`/`rorder` sort the «Запуски» tab; prefixed so other tabs can own `sort` later.
export const testDetailSearchSchema = z.object({
  tab: z
    .enum(['overview', 'validation', 'runs', 'usages', 'diff'])
    .default('overview')
    .catch('overview'),
  launch: z.boolean().optional().catch(undefined),
  diff: z.string().optional().catch(undefined),
  rsort: z.enum(TEST_RUN_SORT_KEYS).default('created_at').catch('created_at'),
  rorder: orderSchema.default('desc').catch('desc'),
})
export type TestDetailSearch = z.infer<typeof testDetailSearchSchema>
export const TEST_DETAIL_DEFAULTS = {
  tab: 'overview',
  rsort: 'created_at',
  rorder: 'desc',
} as const

const getStyles = (theme: GrafanaTheme2) => ({
  tabs: css({ marginBottom: theme.spacing(2) }),
  pair: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))',
    gap: theme.spacing(2),
  }),
  refTitle: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1), minWidth: 0 }),
  mono: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  trend: css({ padding: theme.spacing(1, 0) }),
  labels: css({
    display: 'inline-flex',
    verticalAlign: 'middle',
    marginRight: theme.spacing(0.75),
  }),
})

// Wizard step that owns a validation path (for "go to" links).
export function stepForPath(path: string): number {
  if (path.startsWith('database')) return 1
  if (path.startsWith('workload')) return 2
  if (path.startsWith('sizes') || path.startsWith('provider') || path.startsWith('keep')) return 3
  return 4
}

export function TestDetailPage({
  id,
  search,
  onSearchChange,
}: {
  id: string
  search: TestDetailSearch
  onSearchChange: (next: Partial<TestDetailSearch>) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { data: test } = useSuspenseQuery(testQueries.detail(slug, id))
  const providers = useQuery(providerQueries.list(slug))
  const catalogProviders = useQuery(catalogQueries.providers())
  const canEdit = can('edit-library')
  const canRun = can('run')
  const [renaming, setRenaming] = useState(false)
  const provider = providers.data?.data.find((p) => p.id === test.provider_profile_id)
  const sizeTable = (
    catalogProviders.data?.data.find((c) => c.kind === provider?.kind) ??
    catalogProviders.data?.data[0]
  )?.sizes?.db

  const patch = useMutation({
    mutationFn: (body: Schemas['TestPatch']) => libraryMutations.patchTest(slug, id, body),
    onSuccess: (next) => {
      qc.setQueryData(testQueries.detail(slug, id).queryKey, next)
      void qc.invalidateQueries({ queryKey: [...keys.t(slug), 'tests'] })
    },
  })
  const revalidate = useMutation({
    mutationFn: () => libraryMutations.revalidateTest(slug, id),
    onSuccess: (next) => {
      qc.setQueryData(testQueries.detail(slug, id).queryKey, next)
      void qc.invalidateQueries({ queryKey: [...keys.t(slug), 'tests'] })
      toast.success(
        next.validation?.fits
          ? t('library.tests.revalidated')
          : t('library.tests.revalidatedIssues')
      )
    },
    onError: (e) => toast.error(e),
  })

  const issues = test.validation?.issues ?? []
  const errors = issues.filter((i) => i.severity === 'ERROR').length
  const stale = test.validation?.stale
  const db = test.resolved?.database
  const wl = test.resolved?.workload
  const tabs: { id: TestDetailSearch['tab']; label: string; counter?: number }[] = [
    { id: 'overview', label: t('library.tabs.overview') },
    { id: 'validation', label: t('library.tabs.validation'), counter: issues.length },
    { id: 'runs', label: t('library.tabs.runs'), counter: test.summary?.run_count ?? 0 },
    { id: 'usages', label: t('library.tabs.usages'), counter: test.usages?.length ?? 0 },
    { id: 'diff', label: t('library.tabs.diff') },
  ]
  const openWizard = (step: number) =>
    void navigate({
      to: '/t/$slug/library/tests/new',
      params: { slug },
      search: { id: test.id, step } as never,
    })

  return (
    <Page width="wide">
      <PageHeader
        title={test.name}
        icon="vial"
        breadcrumbs={[
          { label: t('nav.library') },
          { label: t('nav.tests'), to: '/t/$slug/library/tests', params: { slug } },
          { label: test.name },
        ]}
        badge={
          <Stack gap={0.5} alignItems="center">
            <StatusBadge status={test.status} />
            {stale?.database && (
              <Badge color="orange" icon="database" text={t('library.tests.staleDatabase')} />
            )}
            {stale?.workload && (
              <Badge color="orange" icon="bolt" text={t('library.tests.staleWorkload')} />
            )}
            {test.summary?.db_kind && (
              <Badge
                color="blue"
                text={`${test.summary.db_kind} ${test.summary.db_version ?? ''}`}
              />
            )}
          </Stack>
        }
        subtitle={test.description}
        actions={
          <>
            <FavoriteButton kind="test" id={test.id} value={test.is_favorite} />
            {canEdit && (
              <IconButton
                name="pen"
                tooltip={t('library.about.rename')}
                onClick={() => setRenaming(true)}
              />
            )}
            {(test.status === 'needs_attention' || stale?.database || stale?.workload) && (
              <Button
                variant="secondary"
                icon="sync"
                disabled={revalidate.isPending}
                onClick={() => revalidate.mutate()}
              >
                {t('library.tests.revalidate')}
              </Button>
            )}
            <Button
              variant="secondary"
              icon="edit"
              disabled={!canEdit}
              onClick={() => openWizard(1)}
            >
              {t('library.tests.editInWizard')}
            </Button>
            <Tooltip
              content={
                test.status === 'draft'
                  ? t('library.tests.draftCannotLaunch')
                  : t('library.launch.subtitle')
              }
            >
              <Button
                icon="play"
                disabled={!canRun || test.status === 'draft'}
                onClick={() => onSearchChange({ launch: true })}
              >
                {t('common.actions.launch')}
              </Button>
            </Tooltip>
            <EntityActionsMenu
              kind="test"
              entity={test}
              usages={test.usages}
              asButton
              onDeleted={() => void navigate({ to: '/t/$slug/library/tests', params: { slug } })}
            />
          </>
        }
      />
      <TabsBar className={styles.tabs}>
        {tabs.map((tab) => (
          <Tab
            key={tab.id}
            label={tab.label}
            active={search.tab === tab.id}
            counter={tab.counter}
            onChangeTab={() => onSearchChange({ tab: tab.id })}
          />
        ))}
      </TabsBar>

      {search.tab === 'overview' && (
        <WithSidebar
          sidebar={
            <Stack direction="column" gap={2}>
              <EntityAbout
                entity={test}
                canEdit={canEdit}
                onSave={(p) => patch.mutateAsync(p)}
                extra={[
                  {
                    label: t('common.fields.provider'),
                    value: provider ? (
                      <Stack gap={0.5} alignItems="center">
                        <StatusBadge status={provider.status} iconOnly />
                        {provider.name}
                      </Stack>
                    ) : (
                      <Text color="secondary">{t('library.tests.pickAtLaunch')}</Text>
                    ),
                  },
                  {
                    label: t('library.launch.keep'),
                    value:
                      test.keep && test.keep !== '0s'
                        ? formatDuration(test.keep)
                        : t('common.misc.no'),
                  },
                  {
                    label: t('library.tests.rating'),
                    value: (
                      <Stack gap={2}>
                        <Stack gap={0.5} alignItems="center">
                          <Switch
                            value={test.rating?.tenant ?? true}
                            disabled={!canEdit || patch.isPending}
                            onChange={(e) =>
                              patch.mutate({
                                rating: { ...test.rating, tenant: e.currentTarget.checked },
                              })
                            }
                          />
                          <Text variant="bodySmall">{t('library.tests.ratingTenant')}</Text>
                        </Stack>
                        <Stack gap={0.5} alignItems="center">
                          <Switch
                            value={test.rating?.global ?? false}
                            disabled={!canEdit || patch.isPending}
                            onChange={(e) =>
                              patch.mutate({
                                rating: { ...test.rating, global: e.currentTarget.checked },
                              })
                            }
                          />
                          <Text variant="bodySmall">{t('library.tests.ratingGlobal')}</Text>
                        </Stack>
                      </Stack>
                    ),
                  },
                  {
                    label: t('library.tabs.runs'),
                    value: t('library.tests.runCount', { count: test.summary?.run_count ?? 0 }),
                  },
                  {
                    label: t('library.columns.lastRun'),
                    value: test.summary?.last_run ? (
                      <Stack gap={0.5} alignItems="center">
                        <StatusBadge status={test.summary.last_run.status} iconOnly />
                        <AppLink
                          to="/t/$slug/runs/$id"
                          params={{ slug, id: test.summary.last_run.id }}
                        >
                          {test.summary.last_run.name ?? test.summary.last_run.id}
                        </AppLink>
                      </Stack>
                    ) : (
                      t('library.tests.neverRun')
                    ),
                  },
                ]}
              />
            </Stack>
          }
        >
          {issues.length > 0 && (
            <Panel
              title={t('library.validation.title')}
              description={
                test.validation?.fits
                  ? t('library.validation.fitsWithWarnings')
                  : t('library.validation.doesNotFit', { count: errors })
              }
              actions={
                <Button
                  size="sm"
                  variant="secondary"
                  fill="text"
                  onClick={() => onSearchChange({ tab: 'validation' })}
                >
                  {t('common.actions.viewAll')}
                </Button>
              }
            >
              <ValidationIssues
                issues={issues.slice(0, 3)}
                compact
                onGoTo={(p) => openWizard(stepForPath(p))}
              />
            </Panel>
          )}
          <div className={styles.pair}>
            <Panel
              title={
                <span className={styles.refTitle}>
                  <Icon name="database" /> {t('library.kind.database')}
                </span>
              }
              description={
                test.database && 'inline' in test.database
                  ? t('library.tests.inlineSpec')
                  : undefined
              }
              actions={
                db && test.database && 'ref' in test.database ? (
                  <AppLink to="/t/$slug/library/databases/$id" params={{ slug, id: db.id }}>
                    {t('common.actions.open')}
                  </AppLink>
                ) : (
                  <Button
                    size="sm"
                    variant="secondary"
                    fill="text"
                    icon="pen"
                    onClick={() => openWizard(1)}
                  >
                    {t('common.actions.edit')}
                  </Button>
                )
              }
            >
              {db ? (
                <Stack direction="column" gap={1.5}>
                  <Stack gap={1} alignItems="center">
                    <Text weight="medium">{db.name}</Text>
                    <Badge color="blue" text={`${db.kind} ${db.version}`} />
                    {stale?.database && <Badge color="orange" text={t('common.status.stale')} />}
                  </Stack>
                  <TopologyPreview topology={db.topology_preview} compact />
                </Stack>
              ) : (
                <Stack direction="column" gap={1}>
                  <Text color="secondary">{t('library.tests.noDatabase')}</Text>
                  <Button size="sm" variant="secondary" onClick={() => openWizard(1)}>
                    {t('library.tests.pickDatabase')}
                  </Button>
                </Stack>
              )}
            </Panel>
            <Panel
              title={
                <span className={styles.refTitle}>
                  <Icon name="bolt" /> {t('library.kind.workload')}
                </span>
              }
              description={
                test.workload && 'inline' in test.workload
                  ? t('library.tests.inlineSpec')
                  : undefined
              }
              actions={
                wl && test.workload && 'ref' in test.workload ? (
                  <AppLink to="/t/$slug/library/workloads/$id" params={{ slug, id: wl.id }}>
                    {t('common.actions.open')}
                  </AppLink>
                ) : (
                  <Button
                    size="sm"
                    variant="secondary"
                    fill="text"
                    icon="pen"
                    onClick={() => openWizard(2)}
                  >
                    {t('common.actions.edit')}
                  </Button>
                )
              }
            >
              {wl ? (
                <Stack direction="column" gap={1}>
                  <Stack gap={1} alignItems="center">
                    <Text weight="medium">{wl.name}</Text>
                    <Badge color="purple" text={wl.protocol} />
                    <Badge color="darkgrey" text={`stroppy ${wl.stroppy_version}`} />
                    {stale?.workload && <Badge color="orange" text={t('common.status.stale')} />}
                  </Stack>
                  <KeyValueList
                    items={wl.segments.map((s, i) => {
                      const seg = s as {
                        name?: string
                        workload?: { script?: string }
                        run?: { vus?: number; duration?: string; iterations?: number }
                      }
                      return {
                        label: `#${i + 1} ${seg.name ?? ''}`,
                        value: (
                          <span className={styles.mono}>
                            {seg.workload?.script ?? '—'}
                            {seg.run?.vus ? ` · ${seg.run.vus} VU` : ''}
                            {seg.run?.duration
                              ? ` · ${seg.run.duration}`
                              : seg.run?.iterations
                                ? ` · ${seg.run.iterations} it`
                                : ''}
                          </span>
                        ),
                      }
                    })}
                  />
                </Stack>
              ) : (
                <Stack direction="column" gap={1}>
                  <Text color="secondary">{t('library.tests.noWorkload')}</Text>
                  <Button size="sm" variant="secondary" onClick={() => openWizard(2)}>
                    {t('library.tests.pickWorkload')}
                  </Button>
                </Stack>
              )}
            </Panel>
          </div>
          <Panel
            title={t('common.fields.sizes')}
            description={t('library.tests.sizesHint')}
            actions={
              <Button
                size="sm"
                variant="secondary"
                fill="text"
                icon="pen"
                disabled={!canEdit}
                onClick={() => openWizard(3)}
              >
                {t('common.actions.edit')}
              </Button>
            }
          >
            <RequirementsTable
              requirements={test.requirements}
              sizes={test.sizes}
              sizeTable={sizeTable}
            />
          </Panel>
        </WithSidebar>
      )}

      {search.tab === 'validation' && (
        <Panel
          title={t('library.validation.title')}
          description={
            test.validation?.fits
              ? t('library.validation.fits')
              : t('library.validation.doesNotFit', { count: errors })
          }
          actions={
            <Button
              size="sm"
              variant="secondary"
              icon="sync"
              disabled={revalidate.isPending}
              onClick={() => revalidate.mutate()}
            >
              {t('library.tests.revalidate')}
            </Button>
          }
        >
          <ValidationIssues issues={issues} onGoTo={(p) => openWizard(stepForPath(p))} />
        </Panel>
      )}

      {search.tab === 'runs' && (
        <RunsTab
          test={test}
          sort={search.rsort}
          order={search.rorder}
          onSortChange={(rsort, rorder) => onSearchChange({ rsort, rorder })}
        />
      )}

      {search.tab === 'usages' && (
        <Panel
          title={t('library.usages.title')}
          description={t('library.usages.hint', { kind: t('library.kind.test') })}
        >
          <UsagesList usages={test.usages} emptyHint={t('library.usages.emptyTest')} />
        </Panel>
      )}

      {search.tab === 'diff' && (
        <Panel title={t('library.diff.title')}>
          <DiffPanel
            kind="test"
            currentId={test.id}
            currentName={test.name}
            otherId={search.diff}
            onOtherChange={(diff) => onSearchChange({ diff })}
          />
        </Panel>
      )}

      <RenameModal
        isOpen={renaming}
        name={test.name}
        onClose={() => setRenaming(false)}
        onSave={(name) => patch.mutateAsync({ name })}
      />
      <LaunchDrawer
        test={test}
        isOpen={!!search.launch}
        onClose={() => onSearchChange({ launch: undefined })}
      />
    </Page>
  )
}

const TRIGGER_ICON: Record<Run['trigger'], IconName> = {
  manual: 'user',
  suite: 'layer-group',
  api: 'brackets-curly',
  schedule: 'clock-nine',
}

// Run history of the test: the runs table (tables-guide §14) reduced to what differs between
// runs of one test — status, run, metrics, time. A secondary table: grows with the tab.
function RunsTab({
  test,
  sort,
  order,
  onSortChange,
}: {
  test: Test
  sort: TestDetailSearch['rsort']
  order: TestDetailSearch['rorder']
  onSortChange: (sort: TestDetailSearch['rsort'], order: TestDetailSearch['rorder']) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const navigate = useNavigate()
  const history = useQuery({
    ...testQueries.runs(slug, test.id, { sort, order, limit: 50 }),
    placeholderData: keepPreviousData,
  })
  const runs = useMemo(() => history.data?.data ?? [], [history.data])
  const trend = history.data?.trend

  const columns = useMemo<DataTableColumn<Run>[]>(() => {
    // The newest run of this test created before `r` (the list holds the test's history).
    const previous = (r: Run) =>
      runs
        .filter((x) => x.id !== r.id && new Date(x.created_at) < new Date(r.created_at))
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())[0]
    const actions = (r: Run): RowAction[] => {
      const open = (tab?: 'logs' | 'metrics') =>
        void navigate({
          to: tab ? `/t/$slug/runs/$id/${tab}` : '/t/$slug/runs/$id',
          params: { slug, id: r.id },
        } as never)
      const prev = previous(r)
      return [
        { key: 'open', label: t('common.actions.open'), icon: 'eye', onClick: () => open() },
        {
          key: 'logs',
          label: t('runs.tabs.logs'),
          icon: 'document-info',
          onClick: () => open('logs'),
        },
        {
          key: 'metrics',
          label: t('runs.tabs.metrics'),
          icon: 'chart-line',
          onClick: () => open('metrics'),
        },
        {
          key: 'comparePrev',
          label: t('runs.actions.comparePrev'),
          icon: 'columns',
          disabled: !prev,
          disabledReason: t('runs.actions.comparePrevNone'),
          onClick: () =>
            prev &&
            void navigate({
              to: '/t/$slug/compare',
              params: { slug },
              search: { runs: [prev.id, r.id] } as never,
            }),
        },
        {
          key: 'copyLink',
          group: true,
          label: t('runs.actions.copyLink'),
          icon: 'link',
          onClick: () => copyToClipboard(`${window.location.origin}/t/${slug}/runs/${r.id}`, t),
        },
        {
          key: 'copyId',
          label: t('runs.actions.copyId'),
          icon: 'clipboard-alt',
          onClick: () => copyToClipboard(r.id, t),
        },
      ]
    }
    return [
      col.status<Run>({
        id: 'status',
        title: t('runs.columns.status'),
        status: (r) => r.status,
        sortKey: 'status',
      }),
      col.identity<Run>({
        id: 'name',
        header: t('runs.columns.name'),
        sortKey: 'name',
        render: (r) => {
          const running = r.status === 'running' || r.status === 'cancelling'
          const labels = Object.entries(r.labels ?? {}).map(([k, v]) => (v ? `${k}=${v}` : k))
          const second = running
            ? [
                t(`common.phase.${r.phase}`),
                r.summary?.segment ? t('runs.segment', { name: r.summary.segment }) : undefined,
                t('runs.progress', { pct: r.summary?.progress_pct ?? 0 }),
              ]
                .filter(Boolean)
                .join(' · ')
            : r.status_reason || providerView(r).primary
          return {
            title: r.name,
            link: { to: '/t/$slug/runs/$id', params: { slug, id: r.id } },
            lead:
              r.trigger !== 'manual' ? (
                <TriggerMark trigger={r.trigger} icon={TRIGGER_ICON[r.trigger]} />
              ) : undefined,
            subtitle: labels.length ? (
              <>
                <span className={styles.labels}>
                  <TagsCell items={labels} max={2} />
                </span>
                <span title={second}>{second}</span>
              </>
            ) : (
              second
            ),
            progress: running ? (r.summary?.progress_pct ?? 0) : undefined,
            badges:
              r.is_favorite || r.stand_kept ? (
                <>
                  {r.is_favorite && <FavoriteMark />}
                  {r.stand_kept && <Badge text="keep" color="purple" icon="lock" />}
                </>
              ) : undefined,
          }
        },
      }),
      col.custom<Run>({
        id: 'metrics',
        header: t('runs.columns.metrics'),
        width: 290,
        sortOptions: [
          { key: 'qps', label: 'QPS' },
          { key: 'p99', label: 'p99' },
          { key: 'p50', label: 'p50' },
          { key: 'errors', label: t('runs.sort.errors') },
        ],
        cell: (r) => <RunMetricsCell run={r} />,
      }),
      col.custom<Run>({
        id: 'time',
        header: t('runs.columns.time'),
        width: 180,
        sortOptions: [
          { key: 'started_at', label: t('runs.sort.started') },
          { key: 'finished_at', label: t('runs.sort.finished') },
          { key: 'duration', label: t('runs.sort.duration') },
        ],
        cell: (r) => <RunTimeCell run={r} />,
      }),
      col.stack<Run>({
        id: 'provider',
        header: t('runs.columns.provider'),
        width: 200,
        defaultHidden: true,
        sortKey: 'provider',
        render: (r) => providerView(r),
      }),
      col.stack<Run>({
        id: 'author',
        header: t('runs.columns.author'),
        width: 160,
        defaultHidden: true,
        sortKey: 'author',
        value: (r) => r.author.display_name,
        render: (r) => ({ primary: <UserLabel user={r.author} /> }),
      }),
      col.actions<Run>({ title: (r) => r.name, actions }),
    ]
  }, [t, slug, styles, runs, navigate])
  const prefs = useTablePrefs('test-runs', columns)

  return (
    <Stack direction="column" gap={2}>
      <Panel
        title={t('library.tests.trendTitle')}
        description={t('library.tests.trendHint', { count: trend?.points?.length ?? 0 })}
      >
        <div className={styles.trend}>
          <TpsTrend trend={trend} width={640} height={96} showLast />
        </div>
      </Panel>
      <div>
        <DataTableToolbar count={runs.length} settings={<TableSettings prefs={prefs} />} />
        <DataTable<Run>
          columns={prefs.visible}
          density={prefs.density}
          data={runs}
          getRowId={(r) => r.id}
          loading={history.isPending}
          error={history.isError ? history.error : undefined}
          onRetry={() => void history.refetch()}
          sort={{ field: sort, order }}
          onSortChange={(s) =>
            onSortChange(
              (s?.field as TestDetailSearch['rsort']) ?? 'created_at',
              s?.order ?? 'desc'
            )
          }
          rowHref={(r) => `/t/${slug}/runs/${r.id}`}
          empty={{ message: t('library.tests.noRuns') }}
          aria-label={t('library.tabs.runs')}
        />
      </div>
    </Stack>
  )
}

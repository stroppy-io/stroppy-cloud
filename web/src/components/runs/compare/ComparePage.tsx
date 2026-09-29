import { runMutations, runQueries } from '@api/queries/runs'
import type { Comparison, Run } from '@api/types'
import { AppLink } from '@app/AppLink'
import { ErrorState } from '@app/ErrorState'
import { Page } from '@app/Page'
import { PageHeader, SectionTitle } from '@app/PageHeader'
import { RelativeTime } from '@components/RelativeTime'
import { ShareModal } from '@components/runs/ShareModal'
import { StatusBadge } from '@components/StatusBadge'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Badge,
  Button,
  CollapsableSection,
  Combobox,
  EmptyState,
  Icon,
  IconButton,
  Input,
  LoadingPlaceholder,
  Stack,
  Text,
  Tooltip,
  useStyles2,
} from '@grafana/ui'
import { formatDuration, formatMetric } from '@helpers/format'
import { useTenant } from '@hooks/useTenant'
import { keepPreviousData, useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { Fragment, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { SpecChangesTable } from './SpecChangesTable'

export const compareSearchSchema = z.object({
  runs: z.array(z.string()).default([]).catch([]),
  baseline: z.string().optional().catch(undefined),
  deadband: z.number().min(0).max(50).optional().catch(undefined),
})
export type CompareSearch = z.infer<typeof compareSearchSchema>

const getStyles = (theme: GrafanaTheme2) => ({
  picker: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    marginBottom: theme.spacing(2),
  }),
  chips: css({ display: 'flex', gap: theme.spacing(0.5), flexWrap: 'wrap', alignItems: 'center' }),
  chip: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    border: `1px solid ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.pill,
    padding: theme.spacing(0, 0.5, 0, 1),
    fontSize: theme.typography.bodySmall.fontSize,
    background: theme.colors.background.secondary,
    maxWidth: 260,
  }),
  chipBase: css({
    borderColor: theme.colors.primary.border,
    background: theme.colors.primary.transparent,
  }),
  wrap: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    overflow: 'auto',
  }),
  table: css({
    width: '100%',
    borderCollapse: 'separate',
    borderSpacing: 0,
    fontSize: theme.typography.body.fontSize,
    'th, td': {
      padding: theme.spacing(1, 1.5),
      borderBottom: `1px solid ${theme.colors.border.weak}`,
      textAlign: 'right',
      whiteSpace: 'nowrap',
    },
    'th:first-child, td:first-child': {
      textAlign: 'left',
      position: 'sticky',
      left: 0,
      background: theme.colors.background.primary,
      zIndex: 1,
    },
    th: {
      background: theme.colors.background.secondary,
      color: theme.colors.text.secondary,
      fontWeight: theme.typography.fontWeightMedium,
      verticalAlign: 'top',
      position: 'sticky',
      top: 0,
      zIndex: 2,
    },
    'th:first-child': { zIndex: 3 },
  }),
  colHead: css({
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'flex-end',
    gap: 2,
    minWidth: 180,
    textAlign: 'right',
    whiteSpace: 'normal',
  }),
  group: css({
    background: theme.colors.background.canvas,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    textTransform: 'uppercase',
    letterSpacing: 0.4,
    textAlign: 'left !important' as 'left',
  }),
  metric: css({ fontWeight: theme.typography.fontWeightMedium }),
  unit: css({ color: theme.colors.text.secondary, fontSize: theme.typography.bodySmall.fontSize }),
  value: css({
    fontVariantNumeric: 'tabular-nums',
    fontFamily: theme.typography.fontFamilyMonospace,
  }),
  diff: css({ fontSize: theme.typography.bodySmall.fontSize, marginLeft: theme.spacing(0.75) }),
  better: css({ color: theme.colors.success.text }),
  worse: css({ color: theme.colors.error.text }),
  same: css({ color: theme.colors.text.secondary }),
  missing: css({ color: theme.colors.text.disabled }),
  verdicts: css({
    display: 'flex',
    gap: theme.spacing(1),
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  deadband: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.5) }),
})

function RunPicker({
  slug,
  exclude,
  onPick,
}: {
  slug: string
  exclude: string[]
  onPick: (id: string) => void
}) {
  const { t } = useTranslation()
  const list = useInfiniteQuery({
    ...runQueries.list(slug, {
      status: ['completed', 'failed'],
      sort: 'started_at',
      order: 'desc',
      limit: 50,
    }),
    placeholderData: keepPreviousData,
  })
  const options = (list.data?.pages[0]?.data ?? [])
    .filter((r: Run) => !exclude.includes(r.id))
    .map((r: Run) => ({
      label: r.name,
      value: r.id,
      description: `${r.summary?.db_kind ?? ''} ${r.summary?.db_version ?? ''} · ${r.summary?.workload_name ?? ''} · ${formatMetric(r.summary?.headline?.tps, 'tps')}`,
    }))
  return (
    <Combobox
      width={40}
      placeholder={t('runs.compare.addRun')}
      options={options}
      value={null}
      loading={list.isFetching}
      onChange={(o) => o && onPick(o.value)}
    />
  )
}

export function ComparePage({
  search,
  onSearchChange,
}: {
  search: CompareSearch
  onSearchChange: (next: Partial<CompareSearch>) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const ids = search.runs
  const body = useMemo(
    () => ({
      run_ids: ids,
      baseline_run_id:
        search.baseline && ids.includes(search.baseline) ? search.baseline : undefined,
      deadband_pct: search.deadband,
    }),
    [ids, search.baseline, search.deadband]
  )
  const cmp = useQuery({ ...runQueries.compare(slug, body), placeholderData: keepPreviousData })
  const [share, setShare] = useState(false)
  const data = cmp.data as Comparison | undefined
  const baseline = data?.baseline_run_id ?? body.baseline_run_id ?? ids[0]
  const groups = useMemo(() => {
    const m = new Map<string, Comparison['metrics']>()
    for (const row of data?.metrics ?? []) {
      const g = row.group ?? 'Other'
      m.set(g, [...(m.get(g) ?? []), row])
    }
    return [...m.entries()]
  }, [data])
  const remove = (id: string) =>
    onSearchChange({
      runs: ids.filter((x) => x !== id),
      baseline: search.baseline === id ? undefined : search.baseline,
    })

  return (
    <Page width="wide">
      <PageHeader
        title={t('runs.compare.title')}
        icon="columns"
        breadcrumbs={[
          { label: t('runs.title'), to: '/t/$slug/runs', params: { slug } },
          { label: t('runs.compare.title') },
        ]}
        subtitle={t('runs.compare.subtitle')}
        actions={
          <>
            <span className={styles.deadband}>
              <Tooltip content={t('runs.compare.deadbandHint')}>
                <Text color="secondary" variant="bodySmall">
                  {t('runs.compare.deadband')}
                </Text>
              </Tooltip>
              <Input
                type="number"
                width={8}
                min={0}
                max={50}
                step={0.5}
                suffix="%"
                value={search.deadband ?? 2}
                onChange={(e) => {
                  const n = Number(e.currentTarget.value)
                  onSearchChange({ deadband: Number.isNaN(n) || n === 2 ? undefined : n })
                }}
              />
            </span>
            <Button
              variant="secondary"
              icon="share-alt"
              disabled={!data || !can('share')}
              onClick={() => setShare(true)}
            >
              {t('common.actions.share')}
            </Button>
          </>
        }
      />
      <div className={styles.picker}>
        <div className={styles.chips}>
          {ids.map((id) => {
            const col = data?.columns.find((c) => c.run_id === id)
            const isBase = id === baseline
            return (
              <span key={id} className={cx(styles.chip, isBase && styles.chipBase)}>
                {col?.status && <StatusBadge status={col.status} iconOnly />}
                <AppLink to="/t/$slug/runs/$id" params={{ slug, id }} plain>
                  <Text truncate>{col?.name ?? id}</Text>
                </AppLink>
                {isBase ? (
                  <Badge text={t('runs.compare.baseline')} color="blue" />
                ) : (
                  <IconButton
                    name="bookmark"
                    size="sm"
                    tooltip={t('runs.compare.setBaseline')}
                    onClick={() => onSearchChange({ baseline: id })}
                  />
                )}
                <IconButton
                  name="times"
                  size="sm"
                  tooltip={t('common.actions.remove')}
                  onClick={() => remove(id)}
                />
              </span>
            )
          })}
        </div>
        {ids.length < 16 && (
          <RunPicker
            slug={slug}
            exclude={ids}
            onPick={(id) => onSearchChange({ runs: [...ids, id] })}
          />
        )}
        <Text color="secondary" variant="bodySmall">
          {t('runs.compare.count', { count: ids.length })}
        </Text>
      </div>

      {ids.length < 2 ? (
        <EmptyState
          variant="call-to-action"
          image={<Icon name="columns" size="xxxl" />}
          message={t('runs.compare.pickTitle')}
        >
          {t('runs.compare.pickHint')}
        </EmptyState>
      ) : cmp.isPending ? (
        <LoadingPlaceholder text={t('common.misc.loading')} />
      ) : cmp.isError ? (
        <ErrorState error={cmp.error} onRetry={() => void cmp.refetch()} />
      ) : data ? (
        <Stack direction="column" gap={3}>
          <div className={styles.wrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>{t('runs.compare.metric')}</th>
                  {data.columns.map((c) => {
                    const isBase = c.run_id === baseline
                    return (
                      <th key={c.run_id}>
                        <div className={styles.colHead}>
                          <Stack gap={0.5} alignItems="center">
                            {c.status && <StatusBadge status={c.status} iconOnly />}
                            <AppLink to="/t/$slug/runs/$id" params={{ slug, id: c.run_id }}>
                              {c.name ?? c.run_id}
                            </AppLink>
                            {isBase && <Badge text={t('runs.compare.baseline')} color="blue" />}
                          </Stack>
                          <Text color="secondary" variant="bodySmall">
                            {c.summary?.db_kind} {c.summary?.db_version} ·{' '}
                            {c.summary?.workload_name}
                          </Text>
                          <Text color="secondary" variant="bodySmall">
                            {c.summary?.provider_profile?.name}
                            {c.summary?.league ? ` · ${c.summary.league}` : ''}
                          </Text>
                          <Text color="secondary" variant="bodySmall">
                            <RelativeTime value={c.started_at} /> · {formatDuration(c.duration)}
                          </Text>
                          {!isBase && c.verdict && (
                            <span className={styles.verdicts}>
                              <span className={styles.better}>
                                <Icon name="arrow-up" size="sm" /> {c.verdict.better ?? 0}
                              </span>
                              <span className={styles.worse}>
                                <Icon name="arrow-down" size="sm" /> {c.verdict.worse ?? 0}
                              </span>
                              <span className={styles.same}>= {c.verdict.same ?? 0}</span>
                              {c.verdict.missing ? (
                                <span className={styles.missing}>? {c.verdict.missing}</span>
                              ) : null}
                            </span>
                          )}
                        </div>
                      </th>
                    )
                  })}
                </tr>
              </thead>
              <tbody>
                {groups.map(([group, rows]) => (
                  <Fragment key={group}>
                    <tr>
                      <td className={styles.group} colSpan={data.columns.length + 1}>
                        {group}
                      </td>
                    </tr>
                    {rows.map((row) => (
                      <tr key={row.key}>
                        <td>
                          <span className={styles.metric}>{row.title ?? row.key}</span>{' '}
                          <span className={styles.unit}>
                            {row.unit}
                            {row.higher_is_better !== undefined && (
                              <Tooltip
                                content={
                                  row.higher_is_better
                                    ? t('runs.compare.higherBetter')
                                    : t('runs.compare.lowerBetter')
                                }
                              >
                                <Icon
                                  name={row.higher_is_better ? 'arrow-up' : 'arrow-down'}
                                  size="xs"
                                />
                              </Tooltip>
                            )}
                          </span>
                        </td>
                        {data.columns.map((c) => {
                          const cell = row.cells.find((x) => x.run_id === c.run_id)
                          if (!cell?.present)
                            return (
                              <td key={c.run_id} className={styles.missing}>
                                —
                              </td>
                            )
                          const v = cell.verdict
                          return (
                            <td key={c.run_id}>
                              <span className={styles.value}>
                                {formatMetric(cell.value, row.unit)}
                              </span>
                              {v && v !== 'baseline' && cell.diff_pct !== undefined && (
                                <span
                                  className={cx(
                                    styles.diff,
                                    v === 'better' && styles.better,
                                    v === 'worse' && styles.worse,
                                    v === 'same' && styles.same
                                  )}
                                >
                                  {cell.diff_pct > 0 ? '+' : ''}
                                  {cell.diff_pct.toFixed(1)}%
                                </span>
                              )}
                            </td>
                          )
                        })}
                      </tr>
                    ))}
                  </Fragment>
                ))}
                {data.metrics.length === 0 && (
                  <tr>
                    <td colSpan={data.columns.length + 1} style={{ textAlign: 'center' }}>
                      <Text color="secondary">{t('runs.compare.noMetrics')}</Text>
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <Stack direction="column" gap={1}>
            <SectionTitle>{t('runs.compare.specDiff')}</SectionTitle>
            {Object.entries(data.spec_diff).length === 0 ? (
              <Text color="secondary">{t('runs.compare.noSpecDiff')}</Text>
            ) : (
              Object.entries(data.spec_diff).map(([runId, diff]) => {
                const col = data.columns.find((c) => c.run_id === runId)
                const baseCol = data.columns.find((c) => c.run_id === baseline)
                return (
                  <CollapsableSection
                    key={runId}
                    isOpen
                    label={`${col?.name ?? runId} — ${t('runs.compare.changes', { count: diff.changes.length })}`}
                  >
                    <SpecChangesTable
                      changes={diff.changes}
                      aLabel={baseCol?.name ?? t('runs.compare.baseline')}
                      bLabel={col?.name ?? runId}
                    />
                  </CollapsableSection>
                )
              })
            )}
          </Stack>
        </Stack>
      ) : null}
      <ShareModal
        isOpen={share}
        onClose={() => setShare(false)}
        defaultTitle={t('runs.compare.shareTitle', { count: ids.length })}
        onCreate={(input) =>
          runMutations.shareComparison(slug, {
            run_ids: ids,
            baseline_run_id: baseline,
            ttl: input.ttl,
            scope: input.scope,
            title: input.title || undefined,
          })
        }
      />
    </Page>
  )
}

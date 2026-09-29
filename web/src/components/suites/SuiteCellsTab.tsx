import { providerQueries } from '@api/queries/settings'
import { suiteMutations, suiteQueries } from '@api/queries/suites'
import type { Schemas } from '@api/types'
import { AppLink } from '@app/AppLink'
import { toast } from '@app/Toaster'
import { col, WIDTH } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { StatusBadge } from '@components/StatusBadge'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Badge, Button, Icon, Stack, Text, Tooltip, useStyles2 } from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { formatRoleSizes, sortRoles } from '@helpers/sizes'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { CellOverridesDrawer } from './CellOverridesDrawer'

type Suite = Schemas['Suite']
type SuiteCell = Schemas['SuiteCell']

const getStyles = (theme: GrafanaTheme2) => ({
  bar: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    margin: theme.spacing(2, 0, 1.5),
  }),
  right: css({ marginLeft: 'auto', display: 'flex', gap: theme.spacing(1) }),
  disabledRow: css({ td: { opacity: 0.55 } }),
  footer: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5) }),
  ok: css({ color: theme.colors.success.text }),
  bad: css({ color: theme.colors.error.text, display: 'inline-flex' }),
  warn: css({ color: theme.colors.warning.text, display: 'inline-flex' }),
  issues: css({ margin: 0, paddingLeft: theme.spacing(2) }),
})

// Status of a cell: will it run and does it validate. Disabled cells read as skipped; an enabled
// cell shows its validation verdict with the issues in the tooltip.
function CellStatus({
  enabled,
  validation,
}: {
  enabled: boolean
  validation: SuiteCell['validation']
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  if (!enabled)
    return <StatusBadge status="skipped" label={t('suites.cells.status.off')} iconOnly />
  if (!validation)
    return <StatusBadge status="verifying" label={t('suites.cells.status.checking')} iconOnly />
  const issues = validation.issues ?? []
  const errors = issues.filter((i) => i.severity === 'ERROR').length
  const warnings = issues.filter((i) => i.severity === 'WARNING').length
  if (!errors && !warnings)
    return <StatusBadge status="ready" label={t('suites.cells.status.ready')} iconOnly />
  const label = errors
    ? t('suites.cells.errors', { count: errors })
    : t('suites.cells.warnings', { count: warnings })
  return (
    <Tooltip
      content={
        <div>
          <strong>{label}</strong>
          <ul className={styles.issues}>
            {issues.map((i, idx) => (
              <li key={`${i.path}-${idx}`}>
                <strong>{i.path}</strong>: {i.message ?? i.code}
                {i.suggested !== undefined ? ` → ${String(i.suggested)}` : ''}
              </li>
            ))}
          </ul>
        </div>
      }
    >
      <span role="img" aria-label={label} className={errors ? styles.bad : styles.warn}>
        <Icon name={errors ? 'exclamation-circle' : 'exclamation-triangle'} />
      </span>
    </Tooltip>
  )
}

function stripValidation(cells: SuiteCell[]): SuiteCell[] {
  return cells.map(({ validation: _v, ...c }) => c)
}

export function SuiteCellsTab({
  suite,
  readOnly,
  onLaunch,
}: {
  suite: Suite
  readOnly?: boolean
  onLaunch: () => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can } = useTenant()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [cells, setCells] = useState<SuiteCell[]>(() => stripValidation(suite.cells ?? []))
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset local edits when the server copy changes
  useEffect(() => setCells(stripValidation(suite.cells ?? [])), [suite.id, suite.updated_at])
  const dirty = JSON.stringify(cells) !== JSON.stringify(stripValidation(suite.cells ?? []))
  const [editing, setEditing] = useState<SuiteCell | undefined>()
  const [regenNote, setRegenNote] = useState<number | undefined>()
  const providers = useQuery(providerQueries.list(slug))
  const providerName = useCallback(
    (id: string | undefined) =>
      id ? (providers.data?.data.find((p) => p.id === id)?.name ?? id) : undefined,
    [providers.data]
  )

  const body = useMemo<Schemas['SuiteWrite']>(
    () => ({
      name: suite.name,
      tests: suite.tests,
      axes: suite.axes,
      cells,
      concurrency: suite.concurrency,
      defaults: suite.defaults,
    }),
    [suite, cells]
  )
  const preview = useQuery(suiteQueries.preview(slug, body))
  const validated = useMemo(() => {
    const byId = new Map(
      [...(suite.cells ?? []), ...(preview.data?.cells ?? [])].map((c) => [c.id, c.validation])
    )
    return cells.map((c) => ({ ...c, validation: byId.get(c.id) }))
  }, [cells, preview.data, suite.cells])
  const roles = useMemo(() => {
    const set = new Set<string>(['db', 'runner'])
    for (const c of cells) for (const r of Object.keys(c.axis?.sizes ?? {})) set.add(r)
    for (const s of suite.axes?.sizes ?? []) for (const r of Object.keys(s)) set.add(r)
    return sortRoles([...set])
  }, [cells, suite.axes])

  const onError = (e: unknown) => toast.error(e)
  const save = useMutation({
    mutationFn: () =>
      suiteMutations.patch(slug, suite.id, { cells, concurrency: suite.concurrency }),
    onSuccess: () => {
      toast.success(t('suites.cells.saved'))
      setRegenNote(undefined)
      void qc.invalidateQueries({ queryKey: ['t', slug, 'suites'] })
    },
    onError,
  })
  const regenerate = useMutation({
    mutationFn: () => suiteMutations.preview(slug, { ...body, cells }),
    onSuccess: (p) => {
      setCells(stripValidation(p.cells))
      setRegenNote(p.cells.length)
    },
    onError,
  })

  const updateCell = useCallback(
    (next: SuiteCell) => setCells((cs) => cs.map((c) => (c.id === next.id ? next : c))),
    []
  )
  const enabledCount = cells.filter((c) => c.enabled).length
  const totals = preview.data?.totals
  const waves = Math.ceil(enabledCount / Math.max(1, suite.concurrency))

  const copy = (text: string) =>
    void navigator.clipboard
      .writeText(text)
      .then(() => toast.success(t('common.actions.copied')))
      .catch((e) => toast.error(e))

  const rowActions = (c: SuiteCell): RowAction[] => {
    const locked = readOnly ? t('suites.cells.readOnly') : undefined
    return [
      {
        key: 'toggle',
        label: c.enabled ? t('suites.cells.actions.disable') : t('suites.cells.actions.enable'),
        icon: c.enabled ? 'ban' : 'check-circle',
        disabled: !!readOnly,
        disabledReason: locked,
        onClick: () => updateCell({ ...c, enabled: !c.enabled }),
      },
      {
        key: 'overrides',
        label: t('suites.actions.overrides'),
        icon: 'sliders-v-alt',
        description: Object.keys(c.overrides ?? {}).length
          ? t('suites.cells.overridesCount', { count: Object.keys(c.overrides ?? {}).length })
          : undefined,
        disabled: !!readOnly,
        disabledReason: locked,
        onClick: () => setEditing(c),
      },
      {
        key: 'test',
        group: true,
        label: t('runs.actions.openTest'),
        description: c.test.name,
        icon: 'vial',
        onClick: () =>
          void navigate({ to: '/t/$slug/library/tests/$id', params: { slug, id: c.test.id } }),
      },
      {
        key: 'copyId',
        label: t('suites.cells.actions.copyId'),
        icon: 'copy',
        onClick: () => copy(c.id),
      },
    ]
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure; cells/validation drive the recompute
  const columns = useMemo<DataTableColumn<SuiteCell>[]>(
    () => [
      {
        // Validation verdict with the issue list in the tooltip — beyond `col.status`.
        ...col.custom<SuiteCell>({
          id: 'status',
          title: t('suites.cells.columns.status'),
          width: WIDTH.icon,
          align: 'center',
          tight: true,
          cell: (c) => (
            <CellStatus
              enabled={c.enabled}
              validation={(validated.find((v) => v.id === c.id) ?? c).validation}
            />
          ),
        }),
        sticky: 'left',
        hideable: false,
      },
      col.identity<SuiteCell>({
        id: 'name',
        header: t('suites.cells.columns.name'),
        render: (c) => {
          const n = Object.keys(c.overrides ?? {}).length
          return {
            title: c.name ?? c.id,
            subtitle: (
              <span title={c.test.name ?? c.test.id}>
                <AppLink to="/t/$slug/library/tests/$id" params={{ slug, id: c.test.id }} plain>
                  {c.test.name ?? c.test.id}
                </AppLink>
              </span>
            ),
            badges: (
              <>
                {!c.enabled && <Badge text={t('suites.cells.off')} color="darkgrey" />}
                {c.generated === false && <Badge text={t('suites.cells.manual')} color="purple" />}
                {n > 0 && (
                  <Badge
                    text={t('suites.cells.overridesCount', { count: n })}
                    color="blue"
                    icon="sliders-v-alt"
                  />
                )}
              </>
            ),
          }
        },
      }),
      col.stack<SuiteCell>({
        id: 'axis',
        header: t('suites.cells.columns.axis'),
        minWidth: 240,
        render: (c) => {
          const a = c.axis
          const provider = providerName(a?.provider_profile_id)
          const sizes = a?.sizes ? formatRoleSizes(a.sizes) : undefined
          const secondary = [
            a?.database_version ? `${t('suites.cells.axis.version')} ${a.database_version}` : '',
            sizes ?? '',
            a?.workload_variant ? `${t('suites.cells.axis.variant')} ${a.workload_variant}` : '',
          ]
            .filter(Boolean)
            .join(' · ')
          const lines = [
            [t('suites.cells.axis.provider'), provider],
            [t('suites.cells.axis.sizes'), sizes],
            [t('suites.cells.axis.version'), a?.database_version],
            [t('suites.cells.axis.variant'), a?.workload_variant],
          ]
            .filter(([, v]) => v)
            .map(([k, v]) => `${k}: ${v}`)
          return {
            primary: provider ?? t('suites.cells.axis.testProvider'),
            secondary: secondary || (lines.length ? undefined : t('suites.cells.axis.none')),
            title: lines.join('\n') || t('suites.cells.axis.none'),
          }
        },
      }),
      col.actions<SuiteCell>({ title: (c) => c.name ?? c.id, actions: rowActions }),
    ],
    [t, slug, readOnly, validated, providerName, updateCell]
  )
  return (
    <>
      <div className={styles.bar}>
        <Text color="secondary">{t('suites.cells.intro')}</Text>
        <div className={styles.right}>
          {!readOnly && (
            <>
              <Button
                size="sm"
                variant="secondary"
                fill="text"
                onClick={() => setCells((cs) => cs.map((c) => ({ ...c, enabled: true })))}
              >
                {t('suites.actions.enableAll')}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                fill="text"
                onClick={() => setCells((cs) => cs.map((c) => ({ ...c, enabled: false })))}
              >
                {t('suites.actions.disableAll')}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                icon="sync"
                disabled={regenerate.isPending}
                onClick={() => regenerate.mutate()}
              >
                {t('suites.actions.regenerate')}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                fill="outline"
                disabled={!dirty}
                onClick={() => setCells(stripValidation(suite.cells ?? []))}
              >
                {t('suites.actions.discard')}
              </Button>
              <Button
                size="sm"
                icon="save"
                disabled={!dirty || save.isPending}
                onClick={() => save.mutate()}
              >
                {t('suites.actions.saveCells')}
              </Button>
            </>
          )}
        </div>
      </div>
      {regenNote !== undefined && dirty && (
        <Alert severity="info" title={t('suites.cells.regenerated', { count: regenNote })} />
      )}
      <DataTable<SuiteCell>
        columns={columns}
        data={cells}
        getRowId={(c) => c.id}
        rowClassName={(c) => (c.enabled ? undefined : styles.disabledRow)}
        empty={{ message: t('suites.cells.empty') }}
        footer={
          <div className={styles.footer}>
            <Stack justifyContent="space-between" alignItems="center" wrap="wrap">
              <span>
                <strong>{t('suites.cells.totals')}</strong> (
                {t('suites.cellsOf', { enabled: enabledCount, total: cells.length })})
                {totals
                  ? ` — ${t('suites.cells.totalsLine', {
                      machines: totals.machines ?? 0,
                      cpu: totals.cpu ?? 0,
                      ram: totals.memory_gb ?? 0,
                      disk: totals.disk_gb ?? 0,
                    })}`
                  : ''}
              </span>
              <span>
                {t('suites.cells.waves', {
                  waves,
                  concurrency: suite.concurrency,
                  duration: totals?.estimated_duration
                    ? formatDuration(totals.estimated_duration)
                    : '—',
                })}
              </span>
            </Stack>
            <div>
              <strong>{t('suites.cells.quota')}:</strong>{' '}
              {preview.data?.quota_check?.length ? (
                preview.data.quota_check.map((q) => (
                  <span
                    key={q.provider_profile_id}
                    className={cx(q.fits ? styles.ok : styles.bad)}
                    style={{ marginRight: 12 }}
                  >
                    <Icon name={q.fits ? 'check' : 'exclamation-triangle'} size="sm" />{' '}
                    {q.fits
                      ? t('suites.cells.quotaOk', { provider: providerName(q.provider_profile_id) })
                      : t('suites.cells.quotaFail', {
                          provider: providerName(q.provider_profile_id),
                          issues: q.issues?.join('; '),
                        })}
                  </span>
                ))
              ) : (
                <span>{t('suites.cells.quotaNone')}</span>
              )}
            </div>
            {!dirty && enabledCount > 0 && can('run') && (
              <div>
                <Button size="sm" icon="play" onClick={onLaunch}>
                  {t('suites.actions.launch')}
                </Button>
              </div>
            )}
          </div>
        }
      />
      {editing && (
        <CellOverridesDrawer
          cell={editing}
          roles={roles}
          onApply={updateCell}
          onClose={() => setEditing(undefined)}
        />
      )}
    </>
  )
}

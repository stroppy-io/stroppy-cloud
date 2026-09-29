import { keys } from '@api/queries/keys'
import { runMutations } from '@api/queries/runs'
import type { Run, Schemas } from '@api/types'
import { AppLink } from '@app/AppLink'
import { toast } from '@app/Toaster'
import { CopyText } from '@components/CopyText'
import { KeyValueList } from '@components/KeyValueList'
import { TagsEditor } from '@components/TagsEditor'
import { UserLabel } from '@components/UserAvatar'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Badge, Divider, Stack, Switch, Text, Tooltip, useStyles2 } from '@grafana/ui'
import { formatDuration } from '@helpers/format'
import { formatDateTime } from '@helpers/time'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { NotesEditor } from './NotesEditor'

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  sizes: css({
    display: 'grid',
    gridTemplateColumns: 'max-content 1fr',
    gap: theme.spacing(0.5, 1.5),
    fontSize: theme.typography.bodySmall.fontSize,
    alignItems: 'center',
  }),
  role: css({ color: theme.colors.text.secondary }),
  switchRow: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
  }),
  hint: css({ color: theme.colors.text.secondary, fontSize: theme.typography.bodySmall.fontSize }),
})

// Right column of the overview: snapshot, provider & sizes, trigger lineage, rating, labels, notes.
export function RunSidebar({ run }: { run: Run }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const qc = useQueryClient()
  const canEdit = can('run')
  const patch = useMutation({
    mutationFn: (body: Schemas['RunPatch']) => runMutations.patch(slug, run.id, body),
    onSuccess: () => {
      toast.success(t('runs.toasts.saved'))
      void qc.invalidateQueries({ queryKey: keys.t(slug) })
    },
    onError: (e) => toast.error(e),
  })
  const s = run.summary
  const snap = run.snapshot
  const sizes = Object.entries(snap.sizes ?? {})
  const ref = run.trigger_ref

  return (
    <div className={styles.root}>
      <KeyValueList
        title={t('runs.overview.snapshot.title')}
        items={[
          {
            label: t('runs.overview.snapshot.database'),
            value: (
              <span>
                {snap.database_name ?? `${snap.database.kind}`}{' '}
                <Text color="secondary">
                  {snap.database.kind} {snap.database.version}
                </Text>
              </span>
            ),
          },
          {
            label: t('runs.overview.snapshot.topology'),
            value: s?.topology_label
              ? `${s.topology_label}${s.node_count ? ` · ${s.node_count}n` : ''}`
              : '—',
          },
          {
            label: t('runs.overview.snapshot.workload'),
            value: snap.workload_name ?? s?.workload_name ?? '—',
          },
          {
            label: t('runs.overview.snapshot.segments'),
            value: t('runs.overview.snapshot.segmentsCount', {
              count: snap.workload.segments.length,
            }),
          },
          { label: t('runs.overview.snapshot.stroppy'), value: snap.workload.stroppy_version },
          { label: t('runs.overview.snapshot.protocol'), value: snap.workload.protocol },
          {
            label: t('runs.overview.snapshot.keep'),
            value:
              snap.keep && snap.keep !== '0s' ? formatDuration(snap.keep) : t('common.misc.no'),
          },
          { label: t('common.fields.id'), value: <CopyText value={run.id} /> },
        ]}
      />
      <Divider spacing={0} />
      <KeyValueList
        title={t('runs.overview.provider.title')}
        items={[
          {
            label: t('common.fields.provider'),
            value: (
              <span>
                {snap.provider_profile.name ??
                  s?.provider_profile?.name ??
                  snap.provider_profile.id}{' '}
                {s?.provider_kind && <Text color="secondary">· {s.provider_kind}</Text>}
              </span>
            ),
          },
          { label: t('runs.overview.provider.league'), value: s?.league ?? '—' },
          {
            label: t('common.fields.sizes'),
            value: sizes.length ? (
              <div className={styles.sizes}>
                {sizes.map(([role, v]) => (
                  <Tooltip
                    key={role}
                    content={
                      v.disk
                        ? `${v.disk.gb ?? ''} GB ${v.disk.type ?? ''}`.trim()
                        : t('runs.overview.provider.presetDisk')
                    }
                  >
                    <span style={{ display: 'contents' }}>
                      <span className={styles.role}>{role}</span>
                      <span>
                        <Badge text={v.size} color="darkgrey" />
                        {v.disk?.gb ? (
                          <Text color="secondary" variant="bodySmall">
                            {' '}
                            {v.disk.gb} GB
                          </Text>
                        ) : null}
                      </span>
                    </span>
                  </Tooltip>
                ))}
              </div>
            ) : (
              '—'
            ),
          },
          {
            label: t('runs.overview.provider.machines'),
            value: snap.machines?.length
              ? `${snap.machines.length} · ${snap.machines.reduce((a, m) => a + (m.cpu ?? 0), 0)} vCPU · ${snap.machines.reduce((a, m) => a + (m.memory_gb ?? 0), 0)} GB`
              : '—',
          },
        ]}
      />
      <Divider spacing={0} />
      <KeyValueList
        title={t('runs.overview.trigger.title')}
        items={[
          { label: t('runs.filters.trigger'), value: t(`common.trigger.${run.trigger}`) },
          {
            label: t('runs.overview.trigger.test'),
            value: run.test_ref.name ? (
              <AppLink to="/t/$slug/library/tests/$id" params={{ slug, id: run.test_ref.id }}>
                {run.test_ref.name}
              </AppLink>
            ) : (
              <Text color="secondary">{t('runs.overview.trigger.testDeleted')}</Text>
            ),
          },
          ...(ref?.suite_run_id
            ? [
                {
                  label: t('runs.overview.trigger.suiteRun'),
                  value: (
                    <AppLink to="/t/$slug/suite-runs/$id" params={{ slug, id: ref.suite_run_id }}>
                      {ref.cell_id
                        ? `${t('runs.overview.trigger.cell')} ${ref.cell_id}`
                        : ref.suite_run_id}
                    </AppLink>
                  ),
                },
              ]
            : []),
          ...(ref?.schedule_id
            ? [
                {
                  label: t('runs.overview.trigger.schedule'),
                  value: (
                    <AppLink to="/t/$slug/schedules/$id" params={{ slug, id: ref.schedule_id }}>
                      {ref.schedule_id}
                    </AppLink>
                  ),
                },
              ]
            : []),
          ...(ref?.parent_run_id
            ? [
                {
                  label: t('runs.overview.trigger.parent'),
                  value: (
                    <AppLink to="/t/$slug/runs/$id" params={{ slug, id: ref.parent_run_id }}>
                      {t('runs.overview.trigger.parentRun')}
                    </AppLink>
                  ),
                },
              ]
            : []),
          { label: t('common.fields.author'), value: <UserLabel user={run.author} /> },
          { label: t('common.fields.created'), value: formatDateTime(run.created_at) },
          ...(run.finished_at
            ? [{ label: t('common.fields.finished'), value: formatDateTime(run.finished_at) }]
            : []),
          ...(run.graphene?.run_ref
            ? [{ label: 'graphene', value: <CopyText value={run.graphene.run_ref} /> }]
            : []),
        ]}
      />
      <Divider spacing={0} />
      <Stack direction="column" gap={1}>
        <Text element="h3" variant="bodySmall" weight="medium" color="secondary">
          {t('runs.overview.rating.title').toUpperCase()}
        </Text>
        <div className={styles.switchRow}>
          <span>
            {t('runs.overview.rating.tenant')}
            <div className={styles.hint}>{t('runs.overview.rating.tenantHint')}</div>
          </span>
          <Switch
            value={!!run.rating?.tenant}
            disabled={!canEdit || patch.isPending}
            onChange={(e) =>
              patch.mutate({ rating: { ...run.rating, tenant: e.currentTarget.checked } })
            }
          />
        </div>
        <div className={styles.switchRow}>
          <span>
            {t('runs.overview.rating.global')}
            <div className={styles.hint}>{t('runs.overview.rating.globalHint')}</div>
          </span>
          <Switch
            value={!!run.rating?.global}
            disabled={!canEdit || patch.isPending || run.status !== 'completed'}
            onChange={(e) =>
              patch.mutate({ rating: { ...run.rating, global: e.currentTarget.checked } })
            }
          />
        </div>
      </Stack>
      <Divider spacing={0} />
      <Stack direction="column" gap={1}>
        <Text element="h3" variant="bodySmall" weight="medium" color="secondary">
          {t('common.fields.labels').toUpperCase()}
        </Text>
        <TagsEditor
          value={run.labels}
          disabled={!canEdit}
          placeholder={canEdit ? 'key=value' : undefined}
          onChange={(labels) => patch.mutate({ labels })}
        />
      </Stack>
      <Divider spacing={0} />
      <Stack direction="column" gap={0.5}>
        <Text element="h3" variant="bodySmall" weight="medium" color="secondary">
          {t('common.fields.notes').toUpperCase()}
        </Text>
        <NotesEditor
          value={run.notes}
          canEdit={canEdit}
          onSave={(notes) => patch.mutateAsync({ notes })}
        />
      </Stack>
    </div>
  )
}

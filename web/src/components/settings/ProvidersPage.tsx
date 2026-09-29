import { catalogQueries } from '@api/queries/catalog'
import { settingsMutations, settingsQueries } from '@api/queries/settings'
import type { ProviderProfile } from '@api/types'
import { SectionTitle } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { ConfirmAction } from '@components/ConfirmAction'
import { RelativeTime } from '@components/RelativeTime'
import { StatusBadge } from '@components/StatusBadge'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Badge, Button, Icon, Stack, Text, Tooltip, useStyles2 } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ProviderCreateDialog } from './ProviderCreateDialog'
import { ProviderEditDrawer } from './ProviderEditDrawer'
import { ReadOnlyNotice } from './ReadOnlyNotice'

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
    gap: theme.spacing(2),
  }),
  card: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(2),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1.5),
    minWidth: 0,
  }),
  failed: css({ borderColor: theme.colors.error.border }),
  head: css({ display: 'flex', alignItems: 'flex-start', gap: theme.spacing(1.5), minWidth: 0 }),
  logo: css({
    width: 40,
    height: 40,
    borderRadius: theme.shape.radius.default,
    display: 'grid',
    placeItems: 'center',
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    flexShrink: 0,
    fontWeight: theme.typography.fontWeightBold,
    fontSize: theme.typography.bodySmall.fontSize,
    letterSpacing: 0.5,
  }),
  name: css({
    fontWeight: theme.typography.fontWeightMedium,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
  meta: css({
    display: 'grid',
    gridTemplateColumns: 'max-content 1fr',
    gap: theme.spacing(0.5, 1.5),
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    dd: { margin: 0, color: theme.colors.text.primary, overflowWrap: 'anywhere' },
  }),
  reason: css({
    color: theme.colors.error.text,
    fontSize: theme.typography.bodySmall.fontSize,
    overflowWrap: 'anywhere',
  }),
  actions: css({
    display: 'flex',
    gap: theme.spacing(0.5),
    flexWrap: 'wrap',
    marginTop: 'auto',
    paddingTop: theme.spacing(1),
    borderTop: `1px solid ${theme.colors.border.weak}`,
  }),
})

export const KIND_LABEL: Record<string, string> = { yandex: 'YC', aws: 'AWS' }

export function ProvidersPage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const providers = useSuspenseQuery(settingsQueries.providers(slug)).data.data
  const catalog = useQuery(catalogQueries.providers()).data?.data ?? []
  const canManage = can('manage-providers')
  const [createOpen, setCreateOpen] = useState(false)
  const [editing, setEditing] = useState<ProviderProfile>()
  const invalidate = () =>
    qc.invalidateQueries({ queryKey: [...settingsQueries.providers(slug).queryKey.slice(0, 3)] })
  const fail = (e: unknown) => toast.error(e)

  const verify = useMutation({
    mutationFn: (id: string) => settingsMutations.verifyProvider(slug, id),
    onSuccess: async () => {
      await invalidate()
      toast.info(t('settings.providers.verifyStarted'))
    },
    onError: fail,
  })
  const remove = useMutation({
    mutationFn: (id: string) => settingsMutations.deleteProvider(slug, id),
    onSuccess: async () => {
      await invalidate()
      toast.success(t('settings.providers.deleted'))
    },
    onError: fail,
  })

  const kindTitle = (kind: string) => catalog.find((c) => c.kind === kind)?.title ?? kind

  return (
    <Stack direction="column" gap={2}>
      {!canManage && <ReadOnlyNotice />}
      <SectionTitle
        right={
          canManage ? (
            <Button icon="plus" onClick={() => setCreateOpen(true)}>
              {t('settings.providers.add')}
            </Button>
          ) : undefined
        }
      >
        {t('settings.providers.title')}
      </SectionTitle>
      <Text color="secondary">{t('settings.providers.hint')}</Text>
      {providers.length === 0 ? (
        <Stack direction="column" alignItems="center" gap={1}>
          <Icon name="cloud" size="xxl" />
          <Text weight="medium">{t('settings.providers.empty')}</Text>
          <Text color="secondary">{t('settings.providers.emptyHint')}</Text>
          {canManage && (
            <Button icon="plus" onClick={() => setCreateOpen(true)}>
              {t('settings.providers.add')}
            </Button>
          )}
        </Stack>
      ) : (
        <div className={styles.grid}>
          {providers.map((p) => {
            const settings = (p.settings ?? {}) as Record<string, unknown>
            const location = String(settings.zone ?? settings.region ?? '—')
            const scope = String(settings.folder_id ?? settings.vpc ?? settings.cloud_id ?? '')
            const stale =
              !!p.quotas_observed_at &&
              Date.now() - new Date(p.quotas_observed_at).getTime() > 60_000
            const busy = p.status === 'verifying' || p.status === 'deleting'
            return (
              <article
                key={p.id}
                className={cx(styles.card, p.status === 'failed' && styles.failed)}
                aria-label={p.name}
              >
                <div className={styles.head}>
                  <div className={styles.logo} aria-hidden>
                    {KIND_LABEL[p.kind] ?? p.kind.toUpperCase()}
                  </div>
                  <div style={{ minWidth: 0, flex: 1 }}>
                    <div className={styles.name} title={p.name}>
                      {p.name}
                    </div>
                    <Text color="secondary" variant="bodySmall">
                      {kindTitle(p.kind)}
                    </Text>
                  </div>
                  <Tooltip
                    content={
                      p.status === 'verifying'
                        ? t('settings.providers.verifyingHint')
                        : p.status === 'failed'
                          ? (p.status_reason ?? t('settings.providers.failedHint'))
                          : p.status === 'ready'
                            ? t('settings.providers.readyHint')
                            : p.status
                    }
                  >
                    <span>
                      <StatusBadge status={p.status} />
                    </span>
                  </Tooltip>
                </div>
                {p.status === 'failed' && p.status_reason && (
                  <div className={styles.reason}>
                    <Icon name="exclamation-triangle" size="sm" /> {p.status_reason}
                  </div>
                )}
                <dl className={styles.meta}>
                  <dt>{t('settings.providers.location')}</dt>
                  <dd>
                    {location}
                    {scope ? (
                      <Text color="secondary" variant="bodySmall">
                        {' '}
                        · {scope}
                      </Text>
                    ) : null}
                  </dd>
                  <dt>{t('settings.providers.verifiedAt')}</dt>
                  <dd>
                    {p.verified_at ? (
                      <RelativeTime value={p.verified_at} />
                    ) : (
                      t('common.time.never')
                    )}
                  </dd>
                  <dt>{t('settings.providers.quotasObserved')}</dt>
                  <dd>
                    {p.quotas_observed_at ? (
                      <Stack gap={0.5} alignItems="center">
                        <RelativeTime value={p.quotas_observed_at} />
                        {stale && (
                          <Badge
                            text={t('common.status.stale')}
                            color="orange"
                            tooltip={t('settings.providers.staleHint')}
                          />
                        )}
                      </Stack>
                    ) : (
                      t('common.time.never')
                    )}
                  </dd>
                  <dt>{t('common.fields.created')}</dt>
                  <dd>
                    <RelativeTime value={p.created_at} />
                    {p.created_by?.display_name ? ` · ${p.created_by.display_name}` : ''}
                  </dd>
                </dl>
                <div className={styles.actions}>
                  <Button
                    size="sm"
                    variant="secondary"
                    icon="graph-bar"
                    disabled={p.status !== 'ready'}
                    onClick={() =>
                      void navigate({
                        to: '/t/$slug/settings/quotas',
                        params: { slug },
                        hash: p.id,
                      })
                    }
                  >
                    {t('settings.providers.openQuotas')}
                  </Button>
                  {canManage && (
                    <>
                      <Button
                        size="sm"
                        variant="secondary"
                        icon="sync"
                        disabled={busy || verify.isPending}
                        onClick={() => verify.mutate(p.id)}
                      >
                        {t('settings.providers.verify')}
                      </Button>
                      <Button
                        size="sm"
                        variant="secondary"
                        icon="pen"
                        disabled={busy}
                        onClick={() => setEditing(p)}
                      >
                        {t('common.actions.edit')}
                      </Button>
                      <ConfirmAction
                        title={t('settings.providers.deleteTitle', { name: p.name })}
                        body={t('settings.providers.deleteBody')}
                        onConfirm={() => remove.mutateAsync(p.id)}
                      >
                        {(open) => (
                          <Button
                            size="sm"
                            variant="destructive"
                            fill="outline"
                            icon="trash-alt"
                            disabled={busy}
                            onClick={open}
                            aria-label={t('common.actions.delete')}
                          />
                        )}
                      </ConfirmAction>
                    </>
                  )}
                </div>
              </article>
            )
          })}
        </div>
      )}
      {createOpen && (
        <ProviderCreateDialog
          onClose={() => setCreateOpen(false)}
          onCreated={async () => {
            await invalidate()
            setCreateOpen(false)
          }}
        />
      )}
      {editing && (
        <ProviderEditDrawer
          provider={editing}
          onClose={() => setEditing(undefined)}
          onSaved={async () => {
            await invalidate()
            setEditing(undefined)
          }}
        />
      )}
    </Stack>
  )
}

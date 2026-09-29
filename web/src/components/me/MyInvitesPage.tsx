import { meMutations, meQueries } from '@api/queries/me'
import type { Invite } from '@api/types'
import { SectionTitle } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { RelativeTime } from '@components/RelativeTime'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Badge, Button, Icon, Stack, Text, useStyles2 } from '@grafana/ui'
import { useMutation, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

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
  }),
  message: css({
    borderLeft: `3px solid ${theme.colors.border.medium}`,
    paddingLeft: theme.spacing(1.5),
    color: theme.colors.text.secondary,
    fontStyle: 'italic',
  }),
  meta: css({
    display: 'grid',
    gridTemplateColumns: 'max-content 1fr',
    gap: theme.spacing(0.5, 1.5),
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    dd: { margin: 0, color: theme.colors.text.primary },
  }),
  muted: css({ opacity: 0.6 }),
})

export function MyInvitesPage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const qc = useQueryClient()
  const navigate = useNavigate()
  const invites = useSuspenseQuery(meQueries.invites()).data.data
  const fail = (e: unknown) => toast.error(e)
  const invalidate = () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: meQueries.invites().queryKey }),
      qc.invalidateQueries({ queryKey: meQueries.me().queryKey }),
    ])
  const accept = useMutation({
    mutationFn: (inv: Invite) => meMutations.acceptInvite(inv.id),
    onSuccess: async (m, inv) => {
      await invalidate()
      toast.success(t('me.invites.accepted', { name: inv.tenant.name }))
      void navigate({ to: '/t/$slug', params: { slug: m.tenant.slug } })
    },
    onError: fail,
  })
  const decline = useMutation({
    mutationFn: (inv: Invite) => meMutations.declineInvite(inv.id),
    onSuccess: async () => {
      await invalidate()
      toast.success(t('me.invites.declined'))
    },
    onError: fail,
  })
  const pending = invites.filter((i) => i.status === 'pending')
  const past = invites.filter((i) => i.status !== 'pending')

  const card = (inv: Invite) => {
    const expired = new Date(inv.expires_at).getTime() < Date.now()
    const isPending = inv.status === 'pending' && !expired
    return (
      <article key={inv.id} className={styles.card} aria-label={inv.tenant.name}>
        <Stack justifyContent="space-between" alignItems="flex-start">
          <Stack gap={1} alignItems="center">
            <Icon name="building" />
            <div>
              <Text weight="medium">{inv.tenant.name}</Text>
              <br />
              <Text color="secondary" variant="bodySmall">
                {inv.tenant.slug}
              </Text>
            </div>
          </Stack>
          {isPending ? (
            <Badge text={t(`common.role.${inv.role}`)} color="blue" icon="user" />
          ) : (
            <StatusBadge status={expired && inv.status === 'pending' ? 'expired' : inv.status} />
          )}
        </Stack>
        {inv.message && <div className={styles.message}>“{inv.message}”</div>}
        <dl className={styles.meta}>
          <dt>{t('me.invites.invitedBy')}</dt>
          <dd>{inv.invited_by?.display_name ?? '—'}</dd>
          <dt>{t('common.fields.role')}</dt>
          <dd>{t(`common.role.${inv.role}`)}</dd>
          <dt>{expired ? t('me.invites.expiredAt') : t('me.invites.expires')}</dt>
          <dd>
            <RelativeTime value={inv.expires_at} />
          </dd>
        </dl>
        {isPending && (
          <Stack gap={1}>
            <Button
              icon="check"
              disabled={accept.isPending || decline.isPending}
              onClick={() => accept.mutate(inv)}
            >
              {t('me.invites.accept')}
            </Button>
            <Button
              variant="secondary"
              icon="times"
              disabled={accept.isPending || decline.isPending}
              onClick={() => decline.mutate(inv)}
            >
              {t('me.invites.decline')}
            </Button>
          </Stack>
        )}
      </article>
    )
  }

  return (
    <Stack direction="column" gap={3}>
      <Stack direction="column" gap={1}>
        <SectionTitle>{t('me.invites.pending', { count: pending.length })}</SectionTitle>
        {pending.length === 0 ? (
          <Stack direction="column" alignItems="center" gap={1}>
            <Icon name="envelope" size="xxl" />
            <Text weight="medium">{t('me.invites.empty')}</Text>
            <Text color="secondary">{t('me.invites.emptyHint')}</Text>
          </Stack>
        ) : (
          <div className={styles.grid}>{pending.map(card)}</div>
        )}
      </Stack>
      {past.length > 0 && (
        <Stack direction="column" gap={1}>
          <SectionTitle>{t('me.invites.past')}</SectionTitle>
          <div className={`${styles.grid} ${styles.muted}`}>{past.map(card)}</div>
        </Stack>
      )}
    </Stack>
  )
}

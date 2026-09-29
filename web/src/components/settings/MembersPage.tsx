import { tenantMutations, tenantQueries } from '@api/queries/tenants'
import type { Invite, Member, TenantRole } from '@api/types'
import { SectionTitle } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { Dash } from '@components/DataTable/cells'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { RelativeTime } from '@components/RelativeTime'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Badge,
  Button,
  Combobox,
  ConfirmModal,
  Field,
  Input,
  Modal,
  Stack,
  Text,
  TextArea,
  useStyles2,
} from '@grafana/ui'
import { applyServerErrors, firstError } from '@helpers/form'
import { formatDateTime, relativeTime } from '@helpers/time'
import { useCopy } from '@hooks/useCopy'
import { useTenant } from '@hooks/useTenant'
import { revalidateLogic, useForm } from '@tanstack/react-form'
import { useMutation, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { ReadOnlyNotice } from './ReadOnlyNotice'

const ROLES = ['admin', 'member', 'viewer'] as const satisfies readonly TenantRole[]

const getStyles = (theme: GrafanaTheme2) => ({
  avatar: css({
    width: theme.spacing(3.5),
    height: theme.spacing(3.5),
    borderRadius: '50%',
    display: 'inline-grid',
    placeItems: 'center',
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.medium}`,
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    flexShrink: 0,
  }),
})

export function initials(name: string | undefined): string {
  const parts = (name ?? '').trim().split(/\s+/).filter(Boolean)
  if (!parts.length) return '?'
  return parts
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? '')
    .join('')
}

export function MembersPage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can, me } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const members = useSuspenseQuery(tenantQueries.members(slug)).data.data
  const invites = useSuspenseQuery(tenantQueries.invites(slug)).data.data
  const canManage = can('manage-members')
  const [inviteOpen, setInviteOpen] = useState(false)
  const [dialog, setDialog] = useState<
    { kind: 'remove'; member: Member } | { kind: 'revoke'; invite: Invite }
  >()
  const copy = useCopy()
  const invalidate = () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: tenantQueries.members(slug).queryKey }),
      qc.invalidateQueries({ queryKey: tenantQueries.invites(slug).queryKey }),
      qc.invalidateQueries({ queryKey: tenantQueries.detail(slug).queryKey }),
    ])
  const fail = (e: unknown) => toast.error(e)

  const setRole = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: TenantRole }) =>
      tenantMutations.setRole(slug, userId, role),
    onSuccess: async (_, v) => {
      await invalidate()
      toast.success(t('settings.members.roleChanged', { role: t(`common.role.${v.role}`) }))
    },
    onError: fail,
  })
  const remove = useMutation({
    mutationFn: (userId: string) => tenantMutations.removeMember(slug, userId),
    onSuccess: async (_, userId) => {
      setDialog(undefined)
      if (userId === me.id) {
        await qc.invalidateQueries({ queryKey: ['me'] })
        toast.success(t('settings.members.left'))
        void navigate({ to: '/' })
        return
      }
      await invalidate()
      toast.success(t('settings.members.removed'))
    },
    onError: fail,
  })
  const revoke = useMutation({
    mutationFn: (id: string) => tenantMutations.revokeInvite(slug, id),
    onSuccess: async () => {
      setDialog(undefined)
      await invalidate()
      toast.success(t('settings.members.inviteRevoked'))
    },
    onError: fail,
  })

  const noPerm = t('settings.table.noPermission')
  const memberActions = (m: Member): RowAction[] => {
    const isMe = m.user.id === me.id
    const owner = m.role === 'owner'
    return [
      ...ROLES.map(
        (r): RowAction => ({
          key: `role:${r}`,
          label: t('settings.members.setRole', { role: t(`common.role.${r}`) }),
          description: t(`settings.members.roleDesc.${r}`),
          icon: 'user',
          disabled: owner || !canManage || m.role === r || setRole.isPending,
          disabledReason: owner
            ? t('settings.members.ownerImmutable')
            : !canManage
              ? noPerm
              : t('settings.members.currentRole'),
          onClick: () => setRole.mutate({ userId: m.user.id, role: r }),
        })
      ),
      {
        key: 'copyEmail',
        group: true,
        label: t('settings.table.copyEmail'),
        icon: 'envelope',
        disabled: !m.user.email,
        disabledReason: t('settings.table.noEmail'),
        onClick: () => m.user.email && copy(m.user.email),
      },
      {
        key: 'copyId',
        label: t('settings.table.copyId'),
        icon: 'copy',
        onClick: () => copy(m.user.id),
      },
      {
        key: 'remove',
        group: true,
        label: isMe ? t('settings.members.leave') : t('common.actions.remove'),
        icon: isMe ? 'signout' : 'trash-alt',
        destructive: true,
        disabled: owner || (!isMe && !canManage),
        disabledReason: owner ? t('settings.members.ownerCantLeave') : noPerm,
        onClick: () => setDialog({ kind: 'remove', member: m }),
      },
    ]
  }
  const inviteActions = (i: Invite): RowAction[] => [
    {
      key: 'copyEmail',
      label: t('settings.table.copyEmail'),
      icon: 'envelope',
      onClick: () => copy(i.email),
    },
    {
      key: 'revoke',
      group: true,
      label: t('settings.members.revoke'),
      icon: 'ban',
      destructive: true,
      disabled: !canManage || i.status !== 'pending',
      disabledReason: !canManage ? noPerm : t('settings.members.notPending'),
      onClick: () => setDialog({ kind: 'revoke', invite: i }),
    },
  ]

  // biome-ignore lint/correctness/useExhaustiveDependencies: memberActions is a per-render closure over mutation state
  const memberColumns = useMemo<DataTableColumn<Member>[]>(
    () => [
      col.identity<Member>({
        id: 'user',
        header: t('settings.members.columns.user'),
        render: (m) => ({
          title: m.user.display_name ?? m.user.email ?? m.user.id,
          subtitle: m.user.email,
          lead: (
            <span className={styles.avatar} aria-hidden>
              {initials(m.user.display_name ?? m.user.email)}
            </span>
          ),
          badges:
            m.user.id === me.id ? <Badge text={t('common.misc.you')} color="blue" /> : undefined,
        }),
      }),
      col.text<Member>({
        id: 'role',
        header: t('common.fields.role'),
        width: 150,
        value: (m) => t(`common.role.${m.role}`),
      }),
      col.stack<Member>({
        id: 'activity',
        header: t('settings.table.activity'),
        width: 200,
        value: (m) => m.last_seen_at ?? m.joined_at,
        render: (m) => ({
          primary: m.last_seen_at ? <RelativeTime value={m.last_seen_at} /> : <Dash />,
          secondary: t('settings.members.joinedAgo', { time: relativeTime(m.joined_at) }),
          title: [
            m.last_seen_at
              ? t('settings.table.lastSeenAt', { time: formatDateTime(m.last_seen_at) })
              : undefined,
            t('settings.members.joinedAt', { time: formatDateTime(m.joined_at) }),
          ]
            .filter(Boolean)
            .join('\n'),
        }),
      }),
      col.actions<Member>({
        title: (m) => m.user.display_name ?? m.user.email ?? m.user.id,
        actions: memberActions,
      }),
    ],
    [t, styles, me.id, canManage, setRole.isPending]
  )

  // biome-ignore lint/correctness/useExhaustiveDependencies: inviteActions is a per-render closure
  const inviteColumns = useMemo<DataTableColumn<Invite>[]>(
    () => [
      col.status<Invite>({
        id: 'status',
        title: t('common.fields.status'),
        status: (i) => i.status,
      }),
      col.identity<Invite>({
        id: 'email',
        header: t('common.fields.email'),
        render: (i) => ({
          title: i.email,
          subtitle: [
            i.invited_by?.display_name
              ? t('settings.members.invitedByName', { name: i.invited_by.display_name })
              : undefined,
            i.message,
          ]
            .filter(Boolean)
            .join(' · '),
        }),
      }),
      col.text<Invite>({
        id: 'role',
        header: t('common.fields.role'),
        width: 150,
        value: (i) => t(`common.role.${i.role}`),
      }),
      col.stack<Invite>({
        id: 'expires',
        header: t('common.fields.expires'),
        width: 200,
        value: (i) => i.expires_at,
        render: (i) => ({
          primary: <RelativeTime value={i.expires_at} />,
          secondary: t('settings.table.createdAgo', { time: relativeTime(i.created_at) }),
          title: [
            t('settings.table.expiresAt', { time: formatDateTime(i.expires_at) }),
            t('settings.table.createdAt', { time: formatDateTime(i.created_at) }),
          ].join('\n'),
        }),
      }),
      col.actions<Invite>({ title: (i) => i.email, actions: inviteActions }),
    ],
    [t, canManage]
  )

  const pending = invites.filter((i) => i.status === 'pending').length
  const removing = dialog?.kind === 'remove' ? dialog.member : undefined
  const removingMe = removing?.user.id === me.id

  return (
    <Stack direction="column" gap={3}>
      {!canManage && <ReadOnlyNotice />}
      <Stack direction="column" gap={1}>
        <SectionTitle
          right={
            canManage ? (
              <Button icon="plus" onClick={() => setInviteOpen(true)}>
                {t('settings.members.invite')}
              </Button>
            ) : undefined
          }
        >
          {t('settings.members.title', { count: members.length })}
        </SectionTitle>
        <DataTable<Member>
          columns={memberColumns}
          data={members}
          getRowId={(m) => m.user.id}
          clientSort
          aria-label={t('settings.members.title', { count: members.length })}
        />
      </Stack>
      <Stack direction="column" gap={1}>
        <SectionTitle>
          {t('settings.members.invites')}{' '}
          {pending > 0 && (
            <Badge text={t('settings.members.pending', { count: pending })} color="blue" />
          )}
        </SectionTitle>
        <DataTable<Invite>
          columns={inviteColumns}
          data={invites}
          getRowId={(i) => i.id}
          clientSort
          aria-label={t('settings.members.invites')}
          empty={{
            message: t('settings.members.noInvitesHint'),
            button: canManage ? (
              <Button icon="plus" onClick={() => setInviteOpen(true)}>
                {t('settings.members.invite')}
              </Button>
            ) : undefined,
          }}
        />
      </Stack>
      <InviteDialog
        isOpen={inviteOpen}
        onClose={() => setInviteOpen(false)}
        onCreated={async () => {
          await invalidate()
          setInviteOpen(false)
        }}
      />
      <ConfirmModal
        isOpen={!!removing}
        title={
          removingMe
            ? t('settings.members.leaveTitle')
            : t('settings.members.removeTitle', { name: removing?.user.display_name ?? '' })
        }
        body={removingMe ? t('settings.members.leaveBody') : t('settings.members.removeBody')}
        confirmText={removingMe ? t('settings.members.leave') : t('common.actions.remove')}
        confirmButtonVariant="destructive"
        dismissText={t('common.actions.cancel')}
        disabled={remove.isPending}
        onConfirm={() => removing && remove.mutate(removing.user.id)}
        onDismiss={() => setDialog(undefined)}
      />
      <ConfirmModal
        isOpen={dialog?.kind === 'revoke'}
        title={t('settings.members.revokeTitle', {
          email: dialog?.kind === 'revoke' ? dialog.invite.email : '',
        })}
        body={t('settings.members.revokeBody')}
        confirmText={t('settings.members.revoke')}
        confirmButtonVariant="destructive"
        dismissText={t('common.actions.cancel')}
        disabled={revoke.isPending}
        onConfirm={() => {
          if (dialog?.kind === 'revoke') revoke.mutate(dialog.invite.id)
        }}
        onDismiss={() => setDialog(undefined)}
      />
    </Stack>
  )
}

function InviteDialog({
  isOpen,
  onClose,
  onCreated,
}: {
  isOpen: boolean
  onClose: () => void
  onCreated: () => Promise<void>
}) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const schema = useMemo(
    () =>
      z.object({
        // Invite.email (format: email)
        email: z.email(t('common.validation.email')),
        // TenantRole minus owner (server rejects owner)
        role: z.enum(['admin', 'member', 'viewer']),
        message: z
          .string()
          .trim()
          .max(500, t('common.validation.max', { max: 500 })),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: { email: '', role: 'member' as 'admin' | 'member' | 'viewer', message: '' },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    validators: { onDynamic: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        await tenantMutations.invite(slug, {
          email: value.email.trim(),
          role: value.role,
          message: value.message.trim() || undefined,
        })
        toast.success(t('settings.members.invited', { email: value.email }))
        await onCreated()
        formApi.reset()
      } catch (e) {
        applyServerErrors(formApi, e, t('common.errors.generic'))
      }
    },
  })
  if (!isOpen) return null
  return (
    <Modal
      isOpen
      title={t('settings.members.inviteTitle')}
      onDismiss={() => {
        form.reset()
        onClose()
      }}
    >
      <form
        onSubmit={(e) => {
          e.preventDefault()
          void form.handleSubmit()
        }}
      >
        <Stack direction="column" gap={1}>
          <Text color="secondary">{t('settings.members.inviteHint')}</Text>
          <form.Field name="email">
            {(field) => (
              <Field
                label={t('common.fields.email')}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
                required
              >
                <Input
                  id={field.name}
                  type="email"
                  autoFocus
                  placeholder="name@company.com"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="role">
            {(field) => (
              <Field
                label={t('common.fields.role')}
                description={t('settings.members.roleHint')}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
              >
                <Combobox
                  id={field.name}
                  options={ROLES.map((r) => ({
                    label: t(`common.role.${r}`),
                    value: r,
                    description: t(`settings.members.roleDesc.${r}`),
                  }))}
                  value={field.state.value}
                  onChange={(o) => field.handleChange(o.value)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="message">
            {(field) => (
              <Field
                label={t('settings.members.message')}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
              >
                <TextArea
                  id={field.name}
                  rows={3}
                  placeholder={t('settings.members.messagePlaceholder')}
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )}
          </form.Field>
          <form.Subscribe selector={(s) => [s.isSubmitting, s.errorMap.onServer] as const}>
            {([isSubmitting, serverError]) => (
              <>
                <ServerErrorAlert error={serverError} />
                <Modal.ButtonRow>
                  <Button variant="secondary" type="button" onClick={onClose}>
                    {t('common.actions.cancel')}
                  </Button>
                  <Button type="submit" disabled={isSubmitting} icon="envelope">
                    {t('settings.members.sendInvite')}
                  </Button>
                </Modal.ButtonRow>
              </>
            )}
          </form.Subscribe>
        </Stack>
      </form>
    </Modal>
  )
}

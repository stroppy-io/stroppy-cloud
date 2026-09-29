import { tenantMutations, tenantQueries } from '@api/queries/tenants'
import type { ApiToken } from '@api/types'
import { PageFill } from '@app/Page'
import { SectionTitle } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { col } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { RelativeTime } from '@components/RelativeTime'
import { SecretRevealModal } from '@components/SecretRevealModal'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { css } from '@emotion/css'
import { dateTime, type GrafanaTheme2 } from '@grafana/data'
import {
  Button,
  Combobox,
  ConfirmModal,
  DateTimePicker,
  Field,
  Input,
  Modal,
  Stack,
  Text,
  useStyles2,
} from '@grafana/ui'
import { applyServerErrors, firstError } from '@helpers/form'
import { formatDateTime, relativeTime } from '@helpers/time'
import { useCopy } from '@hooks/useCopy'
import { useTenant } from '@hooks/useTenant'
import { revalidateLogic, useForm } from '@tanstack/react-form'
import { useMutation, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { ReadOnlyNotice } from './ReadOnlyNotice'

const getStyles = (theme: GrafanaTheme2) => ({ root: css({ gap: theme.spacing(1.5) }) })

function tokenExpired(x: ApiToken): boolean {
  return !!x.expires_at && new Date(x.expires_at).getTime() < Date.now()
}

export function TokensPage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can } = useTenant()
  const qc = useQueryClient()
  const copy = useCopy()
  const tokens = useSuspenseQuery(tenantQueries.tokens(slug)).data.data
  const canManage = can('manage-tokens')
  const [createOpen, setCreateOpen] = useState(false)
  const [secret, setSecret] = useState<{ name: string; secret: string }>()
  const [revoking, setRevoking] = useState<ApiToken>()
  const invalidate = () => qc.invalidateQueries({ queryKey: tenantQueries.tokens(slug).queryKey })

  const revoke = useMutation({
    mutationFn: (id: string) => tenantMutations.revokeToken(slug, id),
    onSuccess: async () => {
      setRevoking(undefined)
      await invalidate()
      toast.success(t('settings.tokens.revoked'))
    },
    onError: (e) => toast.error(e),
  })

  const rowActions = (x: ApiToken): RowAction[] => [
    {
      key: 'copyPrefix',
      label: t('settings.table.copyPrefix'),
      icon: 'copy',
      onClick: () => copy(x.prefix),
    },
    {
      key: 'copyId',
      label: t('settings.table.copyId'),
      icon: 'copy',
      onClick: () => copy(x.id),
    },
    {
      key: 'revoke',
      group: true,
      label: t('settings.tokens.revoke'),
      icon: 'ban',
      destructive: true,
      disabled: !canManage,
      disabledReason: t('settings.table.noPermission'),
      onClick: () => setRevoking(x),
    },
  ]

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure
  const columns = useMemo<DataTableColumn<ApiToken>[]>(
    () => [
      col.status<ApiToken>({
        id: 'status',
        title: t('common.fields.status'),
        status: (x) => (tokenExpired(x) ? 'expired' : 'active'),
      }),
      col.identity<ApiToken>({
        id: 'name',
        header: t('common.fields.name'),
        render: (x) => ({ title: x.name, subtitle: `${x.prefix}…` }),
      }),
      col.text<ApiToken>({
        id: 'role',
        header: t('common.fields.role'),
        width: 150,
        value: (x) => t(`common.role.${x.role}`),
      }),
      col.stack<ApiToken>({
        id: 'used',
        header: t('common.fields.lastUsed'),
        width: 220,
        value: (x) => x.last_used_at ?? '',
        render: (x) => ({
          primary: x.last_used_at ? (
            <RelativeTime value={x.last_used_at} />
          ) : (
            t('common.time.never')
          ),
          secondary: !x.expires_at
            ? t('settings.table.noExpiry')
            : tokenExpired(x)
              ? t('settings.table.expiredAgo', { time: relativeTime(x.expires_at) })
              : t('settings.table.expiresIn', { time: relativeTime(x.expires_at) }),
          title: [
            x.last_used_at
              ? t('settings.table.lastUsedAt', { time: formatDateTime(x.last_used_at) })
              : undefined,
            x.expires_at
              ? t('settings.table.expiresAt', { time: formatDateTime(x.expires_at) })
              : t('settings.table.noExpiry'),
            t('settings.table.createdAt', { time: formatDateTime(x.created_at) }),
          ]
            .filter(Boolean)
            .join('\n'),
        }),
      }),
      col.actions<ApiToken>({ title: (x) => x.name, actions: rowActions }),
    ],
    [t, canManage]
  )

  return (
    <PageFill className={styles.root}>
      {!canManage && <ReadOnlyNotice />}
      <SectionTitle
        right={
          canManage ? (
            <Button icon="plus" onClick={() => setCreateOpen(true)}>
              {t('settings.tokens.create')}
            </Button>
          ) : undefined
        }
      >
        {t('settings.tokens.title')}
      </SectionTitle>
      <Text color="secondary">{t('settings.tokens.hint')}</Text>
      <DataTable<ApiToken>
        fill
        columns={columns}
        data={tokens}
        getRowId={(x) => x.id}
        clientSort
        aria-label={t('settings.tokens.title')}
        empty={{
          message: t('settings.tokens.emptyHint'),
          button: canManage ? (
            <Button icon="plus" onClick={() => setCreateOpen(true)}>
              {t('settings.tokens.create')}
            </Button>
          ) : undefined,
        }}
      />
      <CreateTokenDialog
        isOpen={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={async (created) => {
          await invalidate()
          setCreateOpen(false)
          setSecret({ name: created.name, secret: created.secret })
        }}
      />
      <SecretRevealModal
        isOpen={!!secret}
        title={t('settings.tokens.secretTitle', { name: secret?.name ?? '' })}
        description={t('settings.tokens.secretHint')}
        secret={secret?.secret}
        onClose={() => setSecret(undefined)}
      />
      <ConfirmModal
        isOpen={!!revoking}
        title={t('settings.tokens.revokeTitle', { name: revoking?.name ?? '' })}
        body={t('settings.tokens.revokeBody')}
        confirmText={t('settings.tokens.revoke')}
        confirmButtonVariant="destructive"
        dismissText={t('common.actions.cancel')}
        disabled={revoke.isPending}
        onConfirm={() => revoking && revoke.mutate(revoking.id)}
        onDismiss={() => setRevoking(undefined)}
      />
    </PageFill>
  )
}

function CreateTokenDialog({
  isOpen,
  onClose,
  onCreated,
}: {
  isOpen: boolean
  onClose: () => void
  onCreated: (created: ApiToken & { secret: string }) => Promise<void>
}) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const schema = useMemo(
    () =>
      z.object({
        // ServiceTokenCreate.name
        name: z
          .string()
          .trim()
          .min(2, t('common.validation.min', { min: 2 }))
          .max(64),
        // ServiceTokenCreate.role ∈ member|viewer
        role: z.enum(['member', 'viewer']),
        // ISO date-time or '' (never)
        expires_at: z
          .string()
          .refine(
            (v) => v === '' || new Date(v).getTime() > Date.now(),
            t('settings.tokens.expiresFuture')
          ),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: { name: '', role: 'member' as 'member' | 'viewer', expires_at: '' },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    validators: { onDynamic: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        const created = await tenantMutations.createToken(slug, {
          name: value.name.trim(),
          role: value.role,
          expires_at: value.expires_at || null,
        })
        await onCreated(created as ApiToken & { secret: string })
        formApi.reset()
      } catch (e) {
        applyServerErrors(formApi, e, t('common.errors.generic'))
      }
    },
  })
  if (!isOpen) return null
  return (
    <Modal isOpen title={t('settings.tokens.createTitle')} onDismiss={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          void form.handleSubmit()
        }}
      >
        <Stack direction="column" gap={1}>
          <form.Field name="name">
            {(field) => (
              <Field
                label={t('common.fields.name')}
                description={t('settings.tokens.nameHint')}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
                required
              >
                <Input
                  id={field.name}
                  autoFocus
                  placeholder="ci-nightly"
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
                description={t('settings.tokens.roleHint')}
                htmlFor={field.name}
              >
                <Combobox
                  id={field.name}
                  options={[
                    {
                      label: t('common.role.member'),
                      value: 'member' as const,
                      description: t('settings.members.roleDesc.member'),
                    },
                    {
                      label: t('common.role.viewer'),
                      value: 'viewer' as const,
                      description: t('settings.members.roleDesc.viewer'),
                    },
                  ]}
                  value={field.state.value}
                  onChange={(o) => field.handleChange(o.value)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="expires_at">
            {(field) => (
              <Field
                label={t('common.fields.expires')}
                description={t('settings.tokens.expiresHint')}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
              >
                <DateTimePicker
                  date={field.state.value ? dateTime(field.state.value) : undefined}
                  minDate={new Date()}
                  clearable
                  onChange={(d) => {
                    field.handleChange(d ? d.toISOString() : '')
                    field.handleBlur()
                  }}
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
                  <Button type="submit" disabled={isSubmitting} icon="key-skeleton-alt">
                    {t('common.actions.create')}
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

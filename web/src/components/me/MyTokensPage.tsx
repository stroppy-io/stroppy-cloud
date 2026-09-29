import { meMutations, meQueries } from '@api/queries/me'
import type { ApiToken, TenantRole } from '@api/types'
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
import { useMe } from '@hooks/useMe'
import { revalidateLogic, useForm } from '@tanstack/react-form'
import { useMutation, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

const RANK: Record<TenantRole, number> = { viewer: 0, member: 1, admin: 2, owner: 3 }
const ALL_ROLES: TenantRole[] = ['owner', 'admin', 'member', 'viewer']

const getStyles = (theme: GrafanaTheme2) => ({ root: css({ gap: theme.spacing(1.5) }) })

function tokenExpired(x: ApiToken): boolean {
  return !!x.expires_at && new Date(x.expires_at).getTime() < Date.now()
}

export function MyTokensPage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const qc = useQueryClient()
  const me = useMe()
  const copy = useCopy()
  const tokens = useSuspenseQuery(meQueries.tokens()).data.data
  const [createOpen, setCreateOpen] = useState(false)
  const [secret, setSecret] = useState<{ name: string; secret: string }>()
  const [revoking, setRevoking] = useState<ApiToken>()
  const invalidate = () => qc.invalidateQueries({ queryKey: meQueries.tokens().queryKey })
  const revoke = useMutation({
    mutationFn: (id: string) => meMutations.revokeToken(id),
    onSuccess: async () => {
      setRevoking(undefined)
      await invalidate()
      toast.success(t('me.tokens.revoked'))
    },
    onError: (e) => toast.error(e),
  })
  const slugs = useMemo(
    () => new Map(me.tenants.map((m) => [m.tenant.id, m.tenant.slug])),
    [me.tenants]
  )

  const rowActions = (x: ApiToken): RowAction[] => [
    {
      key: 'copyPrefix',
      label: t('me.tokens.copyPrefix'),
      icon: 'copy',
      onClick: () => copy(x.prefix),
    },
    {
      key: 'copyId',
      label: t('me.tokens.copyId'),
      icon: 'copy',
      onClick: () => copy(x.id),
    },
    {
      key: 'revoke',
      group: true,
      label: t('me.tokens.revoke'),
      icon: 'ban',
      destructive: true,
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
      col.link<ApiToken>({
        id: 'tenant',
        header: t('common.misc.tenant'),
        icon: 'building',
        width: 200,
        render: (x) => {
          const slug = x.tenant ? slugs.get(x.tenant.id) : undefined
          return {
            text: x.tenant?.name ?? slug,
            link: slug ? { to: '/t/$slug', params: { slug } } : undefined,
          }
        },
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
            ? t('me.tokens.noExpiry')
            : tokenExpired(x)
              ? t('me.tokens.expiredAgo', { time: relativeTime(x.expires_at) })
              : t('me.tokens.expiresIn', { time: relativeTime(x.expires_at) }),
          title: [
            x.last_used_at
              ? t('me.tokens.lastUsedAt', { time: formatDateTime(x.last_used_at) })
              : undefined,
            x.expires_at
              ? t('me.tokens.expiresAt', { time: formatDateTime(x.expires_at) })
              : t('me.tokens.noExpiry'),
            t('me.tokens.createdAt', { time: formatDateTime(x.created_at) }),
          ]
            .filter(Boolean)
            .join('\n'),
        }),
      }),
      col.actions<ApiToken>({ title: (x) => x.name, actions: rowActions }),
    ],
    [t, slugs]
  )
  return (
    <PageFill className={styles.root}>
      <SectionTitle
        right={
          <Button icon="plus" onClick={() => setCreateOpen(true)} disabled={!me.tenants.length}>
            {t('me.tokens.create')}
          </Button>
        }
      >
        {t('me.tokens.title')}
      </SectionTitle>
      <Text color="secondary">{t('me.tokens.hint')}</Text>
      <DataTable<ApiToken>
        fill
        columns={columns}
        data={tokens}
        getRowId={(x) => x.id}
        clientSort
        aria-label={t('me.tokens.title')}
        empty={{
          message: t('me.tokens.emptyHint'),
          button: me.tenants.length ? (
            <Button icon="plus" onClick={() => setCreateOpen(true)}>
              {t('me.tokens.create')}
            </Button>
          ) : undefined,
        }}
      />
      {createOpen && (
        <CreateDialog
          onClose={() => setCreateOpen(false)}
          onCreated={async (c) => {
            await invalidate()
            setCreateOpen(false)
            setSecret({ name: c.name, secret: c.secret })
          }}
        />
      )}
      <SecretRevealModal
        isOpen={!!secret}
        title={t('me.tokens.secretTitle', { name: secret?.name ?? '' })}
        description={t('me.tokens.secretHint')}
        secret={secret?.secret}
        onClose={() => setSecret(undefined)}
      />
      <ConfirmModal
        isOpen={!!revoking}
        title={t('me.tokens.revokeTitle', { name: revoking?.name ?? '' })}
        body={t('me.tokens.revokeBody')}
        confirmText={t('me.tokens.revoke')}
        confirmButtonVariant="destructive"
        dismissText={t('common.actions.cancel')}
        disabled={revoke.isPending}
        onConfirm={() => revoking && revoke.mutate(revoking.id)}
        onDismiss={() => setRevoking(undefined)}
      />
    </PageFill>
  )
}

function CreateDialog({
  onClose,
  onCreated,
}: {
  onClose: () => void
  onCreated: (created: ApiToken & { secret: string }) => Promise<void>
}) {
  const { t } = useTranslation()
  const me = useMe()
  const first = me.tenants[0]
  const schema = useMemo(
    () =>
      z.object({
        // ApiTokenCreate.name
        name: z
          .string()
          .trim()
          .min(2, t('common.validation.min', { min: 2 }))
          .max(64),
        tenant_id: z.string().min(1, t('common.validation.required')),
        role: z.enum(['owner', 'admin', 'member', 'viewer']),
        expires_at: z
          .string()
          .refine(
            (v) => v === '' || new Date(v).getTime() > Date.now(),
            t('me.tokens.expiresFuture')
          ),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: {
      name: '',
      tenant_id: first?.tenant.id ?? '',
      role: (first?.role ?? 'viewer') as TenantRole,
      expires_at: '',
    },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    validators: { onDynamic: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        const created = await meMutations.createToken({
          name: value.name.trim(),
          tenant_id: value.tenant_id,
          role: value.role,
          expires_at: value.expires_at || null,
        })
        await onCreated(created as ApiToken & { secret: string })
      } catch (e) {
        applyServerErrors(formApi, e, t('common.errors.generic'))
      }
    },
  })
  return (
    <Modal isOpen title={t('me.tokens.createTitle')} onDismiss={onClose}>
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
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
                required
              >
                <Input
                  id={field.name}
                  autoFocus
                  placeholder="laptop cli"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="tenant_id">
            {(field) => (
              <Field
                label={t('common.misc.tenant')}
                description={t('me.tokens.tenantHint')}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
              >
                <Combobox
                  id={field.name}
                  options={me.tenants.map((m) => ({
                    label: m.tenant.name,
                    value: m.tenant.id,
                    description: `${m.tenant.slug} · ${t(`common.role.${m.role}`)}`,
                  }))}
                  value={field.state.value}
                  onChange={(o) => {
                    field.handleChange(o.value)
                    const myRole = me.tenants.find((m) => m.tenant.id === o.value)?.role ?? 'viewer'
                    const cur = form.getFieldValue('role')
                    if (RANK[cur] > RANK[myRole]) form.setFieldValue('role', myRole)
                  }}
                />
              </Field>
            )}
          </form.Field>
          <form.Subscribe selector={(s) => s.values.tenant_id}>
            {(tenantId) => {
              const myRole = me.tenants.find((m) => m.tenant.id === tenantId)?.role ?? 'viewer'
              const allowed = ALL_ROLES.filter((r) => RANK[r] <= RANK[myRole])
              return (
                <form.Field name="role">
                  {(field) => (
                    <Field
                      label={t('common.fields.role')}
                      description={t('me.tokens.roleHint', { role: t(`common.role.${myRole}`) })}
                      htmlFor={field.name}
                    >
                      <Combobox
                        id={field.name}
                        options={allowed.map((r) => ({ label: t(`common.role.${r}`), value: r }))}
                        value={field.state.value}
                        onChange={(o) => field.handleChange(o.value)}
                      />
                    </Field>
                  )}
                </form.Field>
              )
            }}
          </form.Subscribe>
          <form.Field name="expires_at">
            {(field) => (
              <Field
                label={t('common.fields.expires')}
                description={t('me.tokens.expiresHint')}
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

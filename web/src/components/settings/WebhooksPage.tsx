import {
  settingsMutations,
  settingsQueries,
  WEBHOOK_EVENTS,
  type WebhookCreate,
} from '@api/queries/settings'
import type { Webhook, WebhookDelivery } from '@api/types'
import { PageFill } from '@app/Page'
import { SectionTitle } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { col, WIDTH } from '@components/DataTable/columns'
import { DataTable, type DataTableColumn } from '@components/DataTable/DataTable'
import type { RowAction } from '@components/DataTable/RowActionsMenu'
import { RelativeTime } from '@components/RelativeTime'
import { SecretRevealModal } from '@components/SecretRevealModal'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Badge,
  Button,
  ConfirmModal,
  Drawer,
  Field,
  Input,
  MultiCombobox,
  Stack,
  Switch,
  Text,
  TextArea,
  useStyles2,
} from '@grafana/ui'
import { applyServerErrors, firstError } from '@helpers/form'
import { formatDateTime, relativeTime } from '@helpers/time'
import { useCopy } from '@hooks/useCopy'
import { useTenant } from '@hooks/useTenant'
import { revalidateLogic, useForm } from '@tanstack/react-form'
import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
  useSuspenseQuery,
} from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { ReadOnlyNotice } from './ReadOnlyNotice'

const getStyles = (theme: GrafanaTheme2) => ({ root: css({ gap: theme.spacing(1.5) }) })

type Secret = { url: string; secret: string; rotated: boolean }
type Dialog = { kind: 'rotate' | 'delete'; webhook: Webhook }

// Webhook state for the status column: off, on, or on with the last delivery failed.
function webhookState(w: Webhook): { status: string; key: 'off' | 'on' | 'failing' } {
  if (!w.enabled) return { status: 'cancelled', key: 'off' }
  if (w.last_delivery?.status === 'failed') return { status: 'failed', key: 'failing' }
  return { status: 'active', key: 'on' }
}

export function WebhooksPage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can } = useTenant()
  const qc = useQueryClient()
  const copy = useCopy()
  const webhooks = useSuspenseQuery(settingsQueries.webhooks(slug)).data.data
  const canManage = can('manage-settings')
  const [editor, setEditor] = useState<{ mode: 'create' } | { mode: 'edit'; webhook: Webhook }>()
  const [deliveriesFor, setDeliveriesFor] = useState<Webhook>()
  const [secret, setSecret] = useState<Secret>()
  const [dialog, setDialog] = useState<Dialog>()
  const invalidate = () =>
    qc.invalidateQueries({ queryKey: settingsQueries.webhooks(slug).queryKey })
  const fail = (e: unknown) => toast.error(e)

  const toggle = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      settingsMutations.patchWebhook(slug, id, { enabled }),
    onSuccess: async (_, v) => {
      await invalidate()
      toast.success(v.enabled ? t('settings.webhooks.enabled') : t('settings.webhooks.disabled'))
    },
    onError: fail,
  })
  const rotate = useMutation({
    mutationFn: (id: string) => settingsMutations.rotateWebhookSecret(slug, id),
    onSuccess: (data) => {
      setDialog(undefined)
      setSecret({ url: data.url, secret: data.secret, rotated: true })
    },
    onError: fail,
  })
  const remove = useMutation({
    mutationFn: (id: string) => settingsMutations.deleteWebhook(slug, id),
    onSuccess: async () => {
      setDialog(undefined)
      await invalidate()
      toast.success(t('settings.webhooks.deleted'))
    },
    onError: fail,
  })

  const rowActions = (w: Webhook): RowAction[] => {
    const noPerm = t('settings.table.noPermission')
    return [
      {
        key: 'deliveries',
        label: t('settings.webhooks.deliveries'),
        icon: 'list-ul',
        onClick: () => setDeliveriesFor(w),
      },
      {
        key: 'edit',
        label: t('common.actions.edit'),
        icon: 'pen',
        disabled: !canManage,
        disabledReason: noPerm,
        onClick: () => setEditor({ mode: 'edit', webhook: w }),
      },
      {
        key: 'toggle',
        label: w.enabled ? t('settings.webhooks.disable') : t('settings.webhooks.enable'),
        icon: w.enabled ? 'pause' : 'play',
        disabled: !canManage || toggle.isPending,
        disabledReason: noPerm,
        onClick: () => toggle.mutate({ id: w.id, enabled: !w.enabled }),
      },
      {
        key: 'rotate',
        label: t('settings.webhooks.rotate'),
        icon: 'key-skeleton-alt',
        disabled: !canManage,
        disabledReason: noPerm,
        onClick: () => setDialog({ kind: 'rotate', webhook: w }),
      },
      {
        key: 'copyUrl',
        group: true,
        label: t('settings.table.copyUrl'),
        icon: 'link',
        onClick: () => copy(w.url),
      },
      {
        key: 'copyId',
        label: t('settings.table.copyId'),
        icon: 'copy',
        onClick: () => copy(w.id),
      },
      {
        key: 'delete',
        group: true,
        label: t('common.actions.delete'),
        icon: 'trash-alt',
        destructive: true,
        disabled: !canManage,
        disabledReason: noPerm,
        onClick: () => setDialog({ kind: 'delete', webhook: w }),
      },
    ]
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: rowActions is a per-render closure over mutation state
  const columns = useMemo<DataTableColumn<Webhook>[]>(
    () => [
      // Status with a webhook-specific label («Выключен» is not a run status); the kit's
      // `col.status` takes no label, hence `custom` in the status column's shape.
      col.custom<Webhook>({
        id: 'status',
        title: t('common.fields.status'),
        width: WIDTH.icon,
        align: 'center',
        tight: true,
        value: (w) => webhookState(w).key,
        cell: (w) => {
          const s = webhookState(w)
          return (
            <StatusBadge status={s.status} iconOnly label={t(`settings.webhooks.state.${s.key}`)} />
          )
        },
      }),
      col.identity<Webhook>({
        id: 'url',
        header: t('common.fields.url'),
        render: (w) => ({ title: w.url, subtitle: w.description }),
      }),
      col.tags<Webhook>({
        id: 'events',
        header: t('common.fields.events'),
        width: 240,
        items: (w) => w.events,
      }),
      col.stack<Webhook>({
        id: 'last',
        header: t('settings.webhooks.lastDelivery'),
        width: 210,
        value: (w) => w.last_delivery?.at ?? '',
        render: (w) => {
          const d = w.last_delivery
          const status = d?.status ? t(`common.status.${d.status}`) : undefined
          return {
            primary: d?.at ? <RelativeTime value={d.at} /> : t('common.time.never'),
            secondary: [
              status,
              t('settings.table.createdAgo', { time: relativeTime(w.created_at) }),
            ]
              .filter(Boolean)
              .join(' · '),
            title: [
              d?.at ? `${t('settings.webhooks.lastDelivery')}: ${formatDateTime(d.at)}` : undefined,
              status,
              t('settings.table.createdAt', { time: formatDateTime(w.created_at) }),
            ]
              .filter(Boolean)
              .join('\n'),
          }
        },
      }),
      col.actions<Webhook>({ title: (w) => w.url, actions: rowActions }),
    ],
    [t, canManage, toggle.isPending]
  )

  return (
    <PageFill className={styles.root}>
      {!canManage && <ReadOnlyNotice />}
      <SectionTitle
        right={
          canManage ? (
            <Button icon="plus" onClick={() => setEditor({ mode: 'create' })}>
              {t('settings.webhooks.add')}
            </Button>
          ) : undefined
        }
      >
        {t('settings.webhooks.title')}
      </SectionTitle>
      <Text color="secondary">{t('settings.webhooks.hint')}</Text>
      <DataTable<Webhook>
        fill
        columns={columns}
        data={webhooks}
        getRowId={(w) => w.id}
        clientSort
        onRowClick={setDeliveriesFor}
        aria-label={t('settings.webhooks.title')}
        empty={{
          message: t('settings.webhooks.emptyHint'),
          button: canManage ? (
            <Button icon="plus" onClick={() => setEditor({ mode: 'create' })}>
              {t('settings.webhooks.add')}
            </Button>
          ) : undefined,
        }}
      />
      {editor && (
        <WebhookDrawer
          webhook={editor.mode === 'edit' ? editor.webhook : undefined}
          onClose={() => setEditor(undefined)}
          onSaved={async (created) => {
            await invalidate()
            setEditor(undefined)
            if (created) setSecret({ url: created.url, secret: created.secret, rotated: false })
          }}
        />
      )}
      {deliveriesFor && (
        <DeliveriesDrawer webhook={deliveriesFor} onClose={() => setDeliveriesFor(undefined)} />
      )}
      <SecretRevealModal
        isOpen={!!secret}
        title={
          secret?.rotated
            ? t('settings.webhooks.secretRotatedTitle')
            : t('settings.webhooks.secretTitle')
        }
        description={t('settings.webhooks.secretHint', { url: secret?.url ?? '' })}
        secret={secret?.secret}
        onClose={() => setSecret(undefined)}
      />
      <ConfirmModal
        isOpen={dialog?.kind === 'rotate'}
        title={t('settings.webhooks.rotateTitle')}
        body={t('settings.webhooks.rotateBody')}
        confirmText={t('settings.webhooks.rotate')}
        confirmButtonVariant="primary"
        dismissText={t('common.actions.cancel')}
        disabled={rotate.isPending}
        onConfirm={() => {
          if (dialog) rotate.mutate(dialog.webhook.id)
        }}
        onDismiss={() => setDialog(undefined)}
      />
      <ConfirmModal
        isOpen={dialog?.kind === 'delete'}
        title={t('settings.webhooks.deleteTitle')}
        body={t('settings.webhooks.deleteBody', { url: dialog?.webhook.url ?? '' })}
        confirmText={t('common.confirm.yesDelete')}
        confirmButtonVariant="destructive"
        dismissText={t('common.actions.cancel')}
        disabled={remove.isPending}
        onConfirm={() => {
          if (dialog) remove.mutate(dialog.webhook.id)
        }}
        onDismiss={() => setDialog(undefined)}
      />
    </PageFill>
  )
}

function WebhookDrawer({
  webhook,
  onClose,
  onSaved,
}: {
  webhook?: Webhook
  onClose: () => void
  onSaved: (created?: { url: string; secret: string }) => Promise<void>
}) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const schema = useMemo(
    () =>
      z.object({
        // WebhookCreate.url (format: uri) — https only
        url: z
          .string()
          .trim()
          .regex(/^https:\/\/[^\s/$.?#].[^\s]*$/i, t('common.validation.url')),
        description: z
          .string()
          .trim()
          .max(200, t('common.validation.max', { max: 200 })),
        events: z.array(z.enum(WEBHOOK_EVENTS)).min(1, t('settings.webhooks.eventsRequired')),
        enabled: z.boolean(),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: {
      url: webhook?.url ?? '',
      description: webhook?.description ?? '',
      events: (webhook?.events ?? ['run.finished', 'run.failed']) as Webhook['events'],
      enabled: webhook?.enabled ?? true,
    },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    validators: { onDynamic: schema },
    onSubmit: async ({ value, formApi }) => {
      const body: WebhookCreate = {
        url: value.url.trim(),
        description: value.description.trim() || undefined,
        events: value.events,
        enabled: value.enabled,
      }
      try {
        if (webhook) {
          await settingsMutations.patchWebhook(slug, webhook.id, body)
          toast.success(t('settings.webhooks.saved'))
          await onSaved()
        } else {
          const created = await settingsMutations.createWebhook(slug, body)
          toast.success(t('settings.webhooks.created'))
          await onSaved({ url: created.url, secret: created.secret })
        }
      } catch (e) {
        applyServerErrors(formApi, e, t('common.errors.generic'))
      }
    },
  })
  return (
    <Drawer
      title={webhook ? t('settings.webhooks.editTitle') : t('settings.webhooks.createTitle')}
      subtitle={webhook?.url}
      size="md"
      onClose={onClose}
    >
      <form
        onSubmit={(e) => {
          e.preventDefault()
          void form.handleSubmit()
        }}
      >
        <Stack direction="column" gap={1}>
          <form.Field name="url">
            {(field) => (
              <Field
                label={t('common.fields.url')}
                description={t('settings.webhooks.urlHint')}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
                required
              >
                <Input
                  id={field.name}
                  autoFocus={!webhook}
                  placeholder="https://ci.example.com/hooks/stroppy"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="description">
            {(field) => (
              <Field
                label={t('common.fields.description')}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
              >
                <TextArea
                  id={field.name}
                  rows={2}
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="events">
            {(field) => (
              <Field
                label={t('common.fields.events')}
                description={t('settings.webhooks.eventsHint')}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
                required
              >
                <MultiCombobox
                  id={field.name}
                  options={WEBHOOK_EVENTS.map((e) => ({
                    label: e,
                    value: e,
                    description: t(`settings.webhooks.events.${e}`),
                  }))}
                  value={field.state.value}
                  enableAllOption
                  onChange={(opts) => {
                    field.handleChange(opts.map((o) => o.value))
                    field.handleBlur()
                  }}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="enabled">
            {(field) => (
              <Field label={t('common.fields.enabled')} htmlFor={field.name}>
                <Switch
                  id={field.name}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.currentTarget.checked)}
                />
              </Field>
            )}
          </form.Field>
          {!webhook && (
            <Text color="secondary" variant="bodySmall">
              {t('settings.webhooks.secretNote')}
            </Text>
          )}
          <form.Subscribe selector={(s) => [s.isSubmitting, s.errorMap.onServer] as const}>
            {([isSubmitting, serverError]) => (
              <Stack direction="column" gap={1}>
                <ServerErrorAlert error={serverError} />
                <Stack gap={1}>
                  <Button type="submit" disabled={isSubmitting} icon={webhook ? 'save' : 'plus'}>
                    {webhook ? t('common.actions.save') : t('common.actions.create')}
                  </Button>
                  <Button type="button" variant="secondary" onClick={onClose}>
                    {t('common.actions.cancel')}
                  </Button>
                </Stack>
              </Stack>
            )}
          </form.Subscribe>
        </Stack>
      </form>
    </Drawer>
  )
}

function DeliveriesDrawer({ webhook, onClose }: { webhook: Webhook; onClose: () => void }) {
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const qc = useQueryClient()
  const copy = useCopy()
  const list = useInfiniteQuery(settingsQueries.deliveries(slug, webhook.id))
  const rows = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  const replay = useMutation({
    mutationFn: (deliveryId: string) =>
      settingsMutations.replayDelivery(slug, webhook.id, deliveryId),
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: settingsQueries.deliveries(slug, webhook.id).queryKey }),
        qc.invalidateQueries({ queryKey: settingsQueries.webhooks(slug).queryKey }),
      ])
      toast.success(t('settings.webhooks.replayed'))
    },
    onError: (e) => toast.error(e),
  })
  const canReplay = can('manage-settings')
  const deliveryActions = (d: WebhookDelivery): RowAction[] => [
    {
      key: 'replay',
      label: t('settings.webhooks.replay'),
      icon: 'repeat',
      disabled: !canReplay || replay.isPending,
      disabledReason: t('settings.table.noPermission'),
      onClick: () => replay.mutate(d.id),
    },
    {
      key: 'copyId',
      group: true,
      label: t('settings.table.copyId'),
      icon: 'copy',
      onClick: () => copy(d.id),
    },
  ]
  // biome-ignore lint/correctness/useExhaustiveDependencies: deliveryActions is a per-render closure over mutation state
  const columns = useMemo<DataTableColumn<WebhookDelivery>[]>(
    () => [
      col.status<WebhookDelivery>({
        id: 'status',
        title: t('common.fields.status'),
        status: (d) => d.status,
      }),
      col.identity<WebhookDelivery>({
        id: 'event',
        header: t('settings.webhooks.event'),
        minWidth: 220,
        render: (d) => ({ title: d.event, subtitle: d.error }),
      }),
      col.number<WebhookDelivery>({
        id: 'attempts',
        header: t('settings.webhooks.attempts'),
        width: 120,
        value: (d) => d.attempts,
        format: (v) => String(v),
      }),
      col.number<WebhookDelivery>({
        id: 'response',
        header: t('settings.webhooks.response'),
        width: 112,
        value: (d) => d.response_status,
        format: (v) => String(v),
      }),
      col.time<WebhookDelivery>({
        id: 'created',
        header: t('common.fields.created'),
        value: (d) => d.created_at,
      }),
      col.actions<WebhookDelivery>({ title: (d) => d.event, actions: deliveryActions }),
    ],
    [t, canReplay, replay.isPending]
  )
  return (
    <Drawer
      title={t('settings.webhooks.deliveriesTitle')}
      subtitle={webhook.url}
      size="lg"
      onClose={onClose}
    >
      <Stack direction="column" gap={1}>
        <Stack gap={1} alignItems="center">
          <Badge
            text={t('settings.webhooks.eventsCount', { count: webhook.events.length })}
            color="darkgrey"
          />
          <Text color="secondary" variant="bodySmall">
            {t('settings.webhooks.deliveriesHint')}
          </Text>
        </Stack>
        <DataTable<WebhookDelivery>
          columns={columns}
          data={rows}
          getRowId={(d) => d.id}
          loading={list.isPending}
          error={list.isError ? list.error : undefined}
          onRetry={() => void list.refetch()}
          empty={{ message: t('settings.webhooks.noDeliveries') }}
          footer={
            list.hasNextPage ? (
              <Stack justifyContent="space-between" alignItems="center">
                <span>{t('common.misc.showing', { count: rows.length })}</span>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => void list.fetchNextPage()}
                  disabled={list.isFetchingNextPage}
                >
                  {t('common.actions.loadMore')}
                </Button>
              </Stack>
            ) : undefined
          }
        />
      </Stack>
    </Drawer>
  )
}

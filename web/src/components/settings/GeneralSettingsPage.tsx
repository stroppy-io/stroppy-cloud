import { settingsMutations, settingsQueries } from '@api/queries/settings'
import { tenantMutations, tenantQueries } from '@api/queries/tenants'
import { toast } from '@app/Toaster'
import { ConfirmAction } from '@components/ConfirmAction'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Button,
  Combobox,
  Field,
  FieldSet,
  Input,
  Modal,
  Stack,
  Switch,
  TagsInput,
  Text,
  TextArea,
  useStyles2,
} from '@grafana/ui'
import { applyServerErrors, firstError, isGoDuration, serverErrors } from '@helpers/form'
import { useTenant } from '@hooks/useTenant'
import { revalidateLogic, useForm } from '@tanstack/react-form'
import { useMutation, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { ReadOnlyNotice } from './ReadOnlyNotice'

const getStyles = (theme: GrafanaTheme2) => ({
  form: css({ maxWidth: 720 }),
  danger: css({
    marginTop: theme.spacing(4),
    border: `1px solid ${theme.colors.error.border}`,
    borderRadius: theme.shape.radius.default,
    padding: theme.spacing(2),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
  }),
  dangerRow: css({
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
  }),
})

export function GeneralSettingsPage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can, me } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const tenant = useSuspenseQuery(tenantQueries.detail(slug)).data
  const settings = useSuspenseQuery(settingsQueries.settings(slug)).data
  const limits = useSuspenseQuery(settingsQueries.limits(slug)).data
  const members = useSuspenseQuery(tenantQueries.members(slug)).data.data
  const canEdit = can('manage-settings')
  const [transferOpen, setTransferOpen] = useState(false)

  const schema = useMemo(
    () =>
      z.object({
        // Tenant.name — OpenAPI TenantPatch.name
        name: z
          .string()
          .trim()
          .min(2, t('common.validation.min', { min: 2 }))
          .max(64),
        description: z
          .string()
          .trim()
          .max(500, t('common.validation.max', { max: 500 })),
        public_name: z
          .string()
          .trim()
          .max(64, t('common.validation.max', { max: 64 })),
        // TenantSettings.run_retention_days ≤ TenantLimits.run_retention_max_days
        run_retention_days: z
          .number({ error: t('common.validation.positive') })
          .int(t('common.validation.positive'))
          .min(1, t('common.validation.positive'))
          .max(
            limits.run_retention_max_days,
            t('settings.general.retentionMax', { max: limits.run_retention_max_days })
          ),
        rating_tenant: z.boolean(),
        rating_global: z.boolean(),
        default_keep: z.string().trim().refine(isGoDuration, t('common.validation.duration')),
        notification_emails: z.array(z.email(t('common.validation.email'))),
      }),
    [t, limits.run_retention_max_days]
  )

  const form = useForm({
    defaultValues: {
      name: tenant.name,
      description: tenant.description ?? '',
      public_name: tenant.public_name ?? '',
      run_retention_days: settings.run_retention_days,
      rating_tenant: settings.default_rating.tenant ?? false,
      rating_global: settings.default_rating.global ?? false,
      default_keep: settings.default_keep,
      notification_emails: settings.notification_emails ?? [],
    },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    validators: { onDynamic: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        await Promise.all([
          tenantMutations.patch(slug, {
            name: value.name.trim(),
            description: value.description.trim(),
            public_name: value.public_name.trim() || null,
          }),
          settingsMutations.patchSettings(slug, {
            run_retention_days: value.run_retention_days,
            default_rating: { tenant: value.rating_tenant, global: value.rating_global },
            default_keep: value.default_keep.trim(),
            notification_emails: value.notification_emails,
          }),
        ])
        await Promise.all([
          qc.invalidateQueries({ queryKey: tenantQueries.detail(slug).queryKey }),
          qc.invalidateQueries({ queryKey: settingsQueries.settings(slug).queryKey }),
          qc.invalidateQueries({ queryKey: ['me'] }),
        ])
        formApi.reset(value)
        toast.success(t('settings.general.saved'))
      } catch (e) {
        applyServerErrors(formApi, e, t('common.errors.generic'))
      }
    },
  })

  const remove = useMutation({
    mutationFn: () => tenantMutations.remove(slug),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ['me'] })
      toast.success(t('settings.general.deleted', { name: tenant.name }))
      void navigate({ to: '/' })
    },
    onError: (e) => toast.error(e),
  })

  return (
    <Stack direction="column" gap={2}>
      {!canEdit && <ReadOnlyNotice />}
      <form
        className={styles.form}
        onSubmit={(e) => {
          e.preventDefault()
          void form.handleSubmit()
        }}
      >
        <FieldSet label={t('settings.general.identity')}>
          <form.Field name="name">
            {(field) => (
              <Field
                label={t('common.fields.name')}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
              >
                <Input
                  id={field.name}
                  value={field.state.value}
                  disabled={!canEdit}
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
                  rows={3}
                  value={field.state.value}
                  disabled={!canEdit}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="public_name">
            {(field) => (
              <Field
                label={t('settings.general.publicName')}
                description={t('settings.general.publicNameHint')}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
              >
                <Input
                  id={field.name}
                  value={field.state.value}
                  placeholder={t('settings.general.publicNamePlaceholder')}
                  disabled={!canEdit}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )}
          </form.Field>
          <Field label={t('settings.general.slug')} description={t('settings.general.slugHint')}>
            <Input value={tenant.slug} disabled readOnly />
          </Field>
        </FieldSet>

        <FieldSet label={t('settings.general.runDefaults')}>
          <form.Field name="run_retention_days">
            {(field) => (
              <Field
                label={t('settings.general.retention')}
                description={t('settings.general.retentionHint', {
                  max: limits.run_retention_max_days,
                })}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
              >
                <Input
                  id={field.name}
                  type="number"
                  min={1}
                  max={limits.run_retention_max_days}
                  width={16}
                  suffix={t('settings.general.days')}
                  value={field.state.value}
                  disabled={!canEdit}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.valueAsNumber)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="default_keep">
            {(field) => (
              <Field
                label={t('settings.general.defaultKeep')}
                description={t('settings.general.defaultKeepHint', { max: limits.max_keep })}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
              >
                <Input
                  id={field.name}
                  width={16}
                  placeholder="0s"
                  value={field.state.value}
                  disabled={!canEdit}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="rating_tenant">
            {(field) => (
              <Field
                label={t('settings.general.ratingTenant')}
                description={t('settings.general.ratingTenantHint')}
                htmlFor={field.name}
              >
                <Switch
                  id={field.name}
                  value={field.state.value}
                  disabled={!canEdit}
                  onChange={(e) => field.handleChange(e.currentTarget.checked)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="rating_global">
            {(field) => (
              <Field
                label={t('settings.general.ratingGlobal')}
                description={t('settings.general.ratingGlobalHint')}
                htmlFor={field.name}
              >
                <Switch
                  id={field.name}
                  value={field.state.value}
                  disabled={!canEdit}
                  onChange={(e) => field.handleChange(e.currentTarget.checked)}
                />
              </Field>
            )}
          </form.Field>
        </FieldSet>

        <FieldSet label={t('settings.general.notifications')}>
          <form.Field name="notification_emails">
            {(field) => (
              <Field
                label={t('settings.general.emails')}
                description={t('settings.general.emailsHint')}
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
              >
                <TagsInput
                  id={field.name}
                  tags={field.state.value}
                  disabled={!canEdit}
                  placeholder={t('settings.general.emailsPlaceholder')}
                  onChange={(tags) => {
                    field.handleChange(tags.map((x) => x.trim()).filter(Boolean))
                    field.handleBlur()
                  }}
                />
              </Field>
            )}
          </form.Field>
        </FieldSet>

        <form.Subscribe selector={(s) => [s.isSubmitting, s.isDirty, s.errorMap.onServer] as const}>
          {([isSubmitting, isDirty, serverError]) => (
            <Stack direction="column" gap={1}>
              <ServerErrorAlert error={serverError} />
              {canEdit && (
                <Stack gap={1}>
                  <Button type="submit" disabled={isSubmitting} icon="save">
                    {isSubmitting ? t('common.misc.saving') : t('common.actions.save')}
                  </Button>
                  <Button
                    type="button"
                    variant="secondary"
                    disabled={!isDirty || isSubmitting}
                    onClick={() => form.reset()}
                  >
                    {t('common.actions.reset')}
                  </Button>
                </Stack>
              )}
            </Stack>
          )}
        </form.Subscribe>
      </form>

      {(can('transfer') || can('delete-tenant')) && (
        <section className={styles.danger} aria-label={t('settings.general.danger')}>
          <Text element="h2" variant="h4" color="error">
            {t('settings.general.danger')}
          </Text>
          {can('transfer') && (
            <div className={styles.dangerRow}>
              <div>
                <Text weight="medium">{t('settings.general.transfer')}</Text>
                <br />
                <Text color="secondary" variant="bodySmall">
                  {t('settings.general.transferHint')}
                </Text>
              </div>
              <Button variant="secondary" icon="exchange-alt" onClick={() => setTransferOpen(true)}>
                {t('settings.general.transferCta')}
              </Button>
            </div>
          )}
          {can('delete-tenant') && (
            <div className={styles.dangerRow}>
              <div>
                <Text weight="medium">{t('settings.general.delete')}</Text>
                <br />
                <Text color="secondary" variant="bodySmall">
                  {t('settings.general.deleteHint')}
                </Text>
              </div>
              <ConfirmAction
                title={t('settings.general.deleteTitle', { name: tenant.name })}
                body={
                  <Stack direction="column" gap={1}>
                    <span>{t('settings.general.deleteBody')}</span>
                    <span>{t('settings.general.deleteConfirm', { slug })}</span>
                  </Stack>
                }
                confirmationText={slug}
                confirmText={t('settings.general.deleteCta')}
                onConfirm={() => remove.mutateAsync()}
              >
                {(open) => (
                  <Button variant="destructive" icon="trash-alt" onClick={open}>
                    {t('settings.general.deleteCta')}
                  </Button>
                )}
              </ConfirmAction>
            </div>
          )}
        </section>
      )}

      <TransferModal
        isOpen={transferOpen}
        onClose={() => setTransferOpen(false)}
        members={members
          .filter((m) => m.role !== 'owner' && m.user.id !== me.id)
          .map((m) => ({
            label: `${m.user.display_name ?? m.user.email ?? m.user.id} · ${t(`common.role.${m.role}`)}`,
            value: m.user.id,
            description: m.user.email,
          }))}
      />
    </Stack>
  )
}

function TransferModal({
  isOpen,
  onClose,
  members,
}: {
  isOpen: boolean
  onClose: () => void
  members: { label: string; value: string; description?: string }[]
}) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const qc = useQueryClient()
  const [userId, setUserId] = useState<string | null>(null)
  const [error, setError] = useState<string>()
  const transfer = useMutation({
    mutationFn: (id: string) => tenantMutations.transfer(slug, id),
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['t', slug] }),
        qc.invalidateQueries({ queryKey: ['me'] }),
      ])
      toast.success(t('settings.general.transferred'))
      onClose()
    },
    onError: (e) => setError(serverErrors(e, t('common.errors.generic')).form),
  })
  return (
    <Modal isOpen={isOpen} title={t('settings.general.transfer')} onDismiss={onClose}>
      <Stack direction="column" gap={2}>
        <Alert severity="warning" title={t('settings.general.transferWarnTitle')}>
          {t('settings.general.transferWarnBody')}
        </Alert>
        <Field
          label={t('settings.general.transferTo')}
          invalid={!!error}
          error={error}
          description={members.length ? undefined : t('settings.general.transferNoMembers')}
        >
          <Combobox
            options={members}
            value={userId}
            isClearable
            placeholder={t('settings.general.transferPick')}
            onChange={(o) => {
              setUserId(o?.value ?? null)
              setError(undefined)
            }}
          />
        </Field>
        <Modal.ButtonRow>
          <Button variant="secondary" onClick={onClose}>
            {t('common.actions.cancel')}
          </Button>
          <Button
            variant="destructive"
            disabled={!userId || transfer.isPending}
            onClick={() => userId && transfer.mutate(userId)}
          >
            {t('settings.general.transferCta')}
          </Button>
        </Modal.ButtonRow>
      </Stack>
    </Modal>
  )
}

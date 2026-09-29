import { type AdminTenant, adminMutations, adminQueries } from '@api/queries/admin'
import { toast } from '@app/Toaster'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { Alert, Button, Combobox, Drawer, Field, Input, Stack, Text } from '@grafana/ui'
import { applyServerErrors, firstError, isGoDuration } from '@helpers/form'
import { revalidateLogic, useForm } from '@tanstack/react-form'
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

const SIZES = ['XS', 'S', 'M', 'L', 'XL'] as const

export function TenantLimitsDrawer({
  tenant: tn,
  onClose,
  onSaved,
}: {
  tenant: AdminTenant
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const { t } = useTranslation()
  const system = useQuery(adminQueries.settings()).data
  const d = system?.default_limits
  const cur = tn.limits
  const schema = useMemo(
    () =>
      z.object({
        max_concurrent_runs: z.number().int().min(1, t('common.validation.positive')),
        max_machines_per_run: z.number().int().min(1, t('common.validation.positive')),
        max_size: z.enum(SIZES),
        max_keep: z.string().trim().refine(isGoDuration, t('common.validation.duration')),
        run_retention_max_days: z.number().int().min(1, t('common.validation.positive')),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: {
      max_concurrent_runs: cur?.max_concurrent_runs ?? d?.max_concurrent_runs ?? 2,
      max_machines_per_run: cur?.max_machines_per_run ?? d?.max_machines_per_run ?? 8,
      max_size: (cur?.max_size ?? d?.max_size ?? 'M') as (typeof SIZES)[number],
      max_keep: cur?.max_keep ?? d?.max_keep ?? '2h',
      run_retention_max_days: cur?.run_retention_max_days ?? d?.run_retention_max_days ?? 90,
    },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    validators: { onDynamic: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        await adminMutations.setLimits(tn.slug, { ...value, max_keep: value.max_keep.trim() })
        toast.success(t('admin.tenants.limitsSaved', { name: tn.name }))
        await onSaved()
      } catch (e) {
        applyServerErrors(formApi, e, t('common.errors.generic'))
      }
    },
  })
  const resetToDefaults = async () => {
    try {
      await adminMutations.setLimits(tn.slug, {
        max_concurrent_runs: null,
        max_machines_per_run: null,
        max_size: null,
        max_keep: null,
        run_retention_max_days: null,
      })
      toast.success(t('admin.tenants.limitsReset', { name: tn.name }))
      await onSaved()
    } catch (e) {
      applyServerErrors(form, e, t('common.errors.generic'))
    }
  }
  const num = (
    name: 'max_concurrent_runs' | 'max_machines_per_run' | 'run_retention_max_days',
    label: string,
    def?: number
  ) => (
    <form.Field name={name}>
      {(field) => (
        <Field
          label={label}
          description={
            def !== undefined ? t('admin.tenants.platformDefault', { value: def }) : undefined
          }
          invalid={!!firstError(field.state.meta)}
          error={firstError(field.state.meta)}
          htmlFor={field.name}
        >
          <Input
            id={field.name}
            type="number"
            min={1}
            width={16}
            value={field.state.value}
            onBlur={field.handleBlur}
            onChange={(e) => field.handleChange(e.currentTarget.valueAsNumber)}
          />
        </Field>
      )}
    </form.Field>
  )
  return (
    <Drawer
      title={t('admin.tenants.editLimits')}
      subtitle={`${tn.name} · ${tn.slug}`}
      size="sm"
      onClose={onClose}
    >
      <form
        onSubmit={(e) => {
          e.preventDefault()
          void form.handleSubmit()
        }}
      >
        <Stack direction="column" gap={1}>
          <Alert severity="info" title={t('admin.tenants.limitsAboutTitle')}>
            {t('admin.tenants.limitsAboutBody')}
          </Alert>
          {cur?.source === 'tenant_override' && (
            <Text color="secondary" variant="bodySmall">
              {t('settings.limits.overrideNote')}
            </Text>
          )}
          {num(
            'max_concurrent_runs',
            t('settings.limits.maxConcurrentRuns'),
            d?.max_concurrent_runs
          )}
          {num(
            'max_machines_per_run',
            t('settings.limits.maxMachinesPerRun'),
            d?.max_machines_per_run
          )}
          <form.Field name="max_size">
            {(field) => (
              <Field
                label={t('settings.limits.maxSize')}
                description={
                  d ? t('admin.tenants.platformDefault', { value: d.max_size }) : undefined
                }
                htmlFor={field.name}
              >
                <Combobox
                  id={field.name}
                  width={16}
                  options={SIZES.map((s) => ({ label: s, value: s }))}
                  value={field.state.value}
                  onChange={(o) => field.handleChange(o.value)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="max_keep">
            {(field) => (
              <Field
                label={t('settings.limits.maxKeep')}
                description={
                  d ? t('admin.tenants.platformDefault', { value: d.max_keep }) : undefined
                }
                invalid={!!firstError(field.state.meta)}
                error={firstError(field.state.meta)}
                htmlFor={field.name}
              >
                <Input
                  id={field.name}
                  width={16}
                  placeholder="2h"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )}
          </form.Field>
          {num(
            'run_retention_max_days',
            t('settings.limits.retentionMax'),
            d?.run_retention_max_days
          )}
          <form.Subscribe selector={(s) => [s.isSubmitting, s.errorMap.onServer] as const}>
            {([isSubmitting, serverError]) => (
              <Stack direction="column" gap={1}>
                <ServerErrorAlert error={serverError} />
                <Stack gap={1} wrap="wrap">
                  <Button type="submit" icon="save" disabled={isSubmitting}>
                    {t('admin.tenants.saveOverride')}
                  </Button>
                  <Button
                    type="button"
                    variant="secondary"
                    disabled={isSubmitting}
                    onClick={() => void resetToDefaults()}
                  >
                    {t('admin.tenants.resetToDefaults')}
                  </Button>
                  <Button type="button" variant="secondary" fill="text" onClick={onClose}>
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

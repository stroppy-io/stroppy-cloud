import { isApiError } from '@api/errors'
import { testQueries } from '@api/queries/library'
import { providerQueries } from '@api/queries/settings'
import { suiteQueries } from '@api/queries/suites'
import type { Schedule, Schemas } from '@api/types'
import { RoleSizesEditor } from '@components/RoleSizesEditor'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Button,
  Checkbox,
  Combobox,
  Field,
  FieldSet,
  Input,
  RadioButtonGroup,
  Stack,
  Switch,
  Text,
  useStyles2,
} from '@grafana/ui'
import { isValidCron, isValidTimeZone } from '@helpers/cron'
import { firstError } from '@helpers/form'
import { listTimeZones, localTimeZone, zoneOffsetLabel } from '@helpers/timezones'
import { useTenant } from '@hooks/useTenant'
import { useForm, useStore } from '@tanstack/react-form'
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { CronPreview } from './CronPreview'

export interface ScheduleFormValues {
  name: string
  targetKind: 'test' | 'suite'
  targetId: string
  cron: string
  timezone: string
  enabled: boolean
  providerId: string
  sizesOn: boolean
  sizes: Schemas['RoleSizes']
  keep: string
  ratingOn: boolean
  ratingTenant: boolean
  ratingGlobal: boolean
}

export function scheduleFormDefaults(
  s?: Schedule,
  preset?: { kind?: 'test' | 'suite'; target?: string }
): ScheduleFormValues {
  return {
    name: s?.name ?? '',
    targetKind: s?.target.kind ?? preset?.kind ?? 'test',
    targetId: s?.target.id ?? preset?.target ?? '',
    cron: s?.cron ?? '0 2 * * *',
    timezone: s?.timezone ?? localTimeZone(),
    enabled: s?.enabled ?? true,
    providerId: s?.overrides?.provider_profile_id ?? '',
    sizesOn: !!s?.overrides?.sizes,
    sizes: s?.overrides?.sizes ?? { db: { size: 'M' }, runner: { size: 'S' } },
    keep: s?.overrides?.keep ?? '',
    ratingOn: !!s?.overrides?.rating,
    ratingTenant: s?.overrides?.rating?.tenant ?? true,
    ratingGlobal: s?.overrides?.rating?.global ?? false,
  }
}

export function toScheduleWrite(v: ScheduleFormValues): Schemas['ScheduleWrite'] {
  const overrides: Schemas['LaunchOverrides'] = {}
  if (v.providerId) overrides.provider_profile_id = v.providerId
  if (v.sizesOn) overrides.sizes = v.sizes
  if (v.keep.trim()) overrides.keep = v.keep.trim()
  if (v.ratingOn) overrides.rating = { tenant: v.ratingTenant, global: v.ratingGlobal }
  return {
    name: v.name.trim().replace(/\s+/g, ' '),
    target: { kind: v.targetKind, id: v.targetId },
    cron: v.cron.trim().replace(/\s+/g, ' '),
    timezone: v.timezone,
    enabled: v.enabled,
    overrides: Object.keys(overrides).length ? overrides : undefined,
  }
}

const PRESETS: { key: string; cron: string }[] = [
  { key: 'hourly', cron: '0 * * * *' },
  { key: 'every6h', cron: '0 */6 * * *' },
  { key: 'daily', cron: '0 2 * * *' },
  { key: 'weekdays', cron: '0 9 * * 1-5' },
  { key: 'weekly', cron: '0 3 * * 6' },
  { key: 'monthly', cron: '0 3 1 * *' },
]

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) 360px',
    gap: theme.spacing(3),
    alignItems: 'start',
    [theme.breakpoints.down('md')]: { gridTemplateColumns: '1fr' },
  }),
  presets: css({
    display: 'flex',
    gap: theme.spacing(0.5),
    flexWrap: 'wrap',
    marginBottom: theme.spacing(1),
  }),
  mono: css({ fontFamily: theme.typography.fontFamilyMonospace }),
  sticky: css({ position: 'sticky', top: theme.spacing(2) }),
})

// Create/edit form. Format checks are local (cron grammar, tz list); existence of the target and
// quota-level checks are the server's (`validation.errors[]` mapped back onto fields).
export function ScheduleForm({
  initial,
  submitLabel,
  onSubmit,
  onCancel,
}: {
  initial: ScheduleFormValues
  submitLabel: string
  onSubmit: (write: Schemas['ScheduleWrite']) => Promise<unknown>
  onCancel?: () => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug } = useTenant()
  const tests = useQuery(testQueries.options(slug))
  const suites = useQuery(suiteQueries.all(slug))
  const providers = useQuery(providerQueries.list(slug))
  const [serverError, setServerError] = useState<string | undefined>()
  const zones = useMemo(
    () => listTimeZones().map((tz) => ({ label: tz, value: tz, description: zoneOffsetLabel(tz) })),
    []
  )

  const schema = useMemo(
    () =>
      z.object({
        name: z
          .string()
          .trim()
          .min(1, t('schedules.form.errors.name'))
          .max(120, t('schedules.form.errors.name')), // ScheduleWrite.name
        targetKind: z.enum(['test', 'suite']), // ScheduleTarget.kind
        targetId: z.string().min(1, t('schedules.form.errors.target')), // ScheduleTarget.id
        cron: z.string().refine((v) => isValidCron(v), t('schedules.form.errors.cron')), // ScheduleWrite.cron
        timezone: z.string().refine((v) => isValidTimeZone(v), t('schedules.form.errors.timezone')),
        enabled: z.boolean(),
        providerId: z.string(),
        sizesOn: z.boolean(),
        sizes: z.record(
          z.string(),
          z.object({ size: z.enum(['XS', 'S', 'M', 'L', 'XL']) }).passthrough()
        ),
        keep: z
          .string()
          .trim()
          .regex(/^(\d+(h|m|s))?$/, t('schedules.form.errors.keep')),
        ratingOn: z.boolean(),
        ratingTenant: z.boolean(),
        ratingGlobal: z.boolean(),
      }),
    [t]
  )

  const form = useForm({
    defaultValues: initial,
    validators: { onBlur: schema, onSubmit: schema },
    onSubmit: async ({ value, formApi }) => {
      setServerError(undefined)
      try {
        await onSubmit(toScheduleWrite(value))
      } catch (e) {
        if (isApiError(e) && e.validation?.errors?.length) {
          const fields: Record<string, string> = {}
          for (const issue of e.validation.errors) {
            const path =
              issue.path === 'target.id' || issue.path === 'target'
                ? 'targetId'
                : issue.path === 'overrides.keep'
                  ? 'keep'
                  : issue.path
            fields[path] = issue.message ?? issue.code
          }
          formApi.setErrorMap({ onServer: { fields, form: e.detail } as never })
        } else setServerError(isApiError(e) ? (e.detail ?? e.title) : String(e))
      }
    },
  })
  const cron = useStore(form.store, (s) => s.values.cron)
  const timezone = useStore(form.store, (s) => s.values.timezone)
  const targetKind = useStore(form.store, (s) => s.values.targetKind)
  const sizesOn = useStore(form.store, (s) => s.values.sizesOn)
  const ratingOn = useStore(form.store, (s) => s.values.ratingOn)

  const targetOptions = useMemo(
    () =>
      targetKind === 'test'
        ? (tests.data?.data ?? []).map((x) => ({
            label: x.name,
            value: x.id,
            description: [x.summary?.db_kind, x.summary?.db_version, t(`common.status.${x.status}`)]
              .filter(Boolean)
              .join(' · '),
          }))
        : (suites.data?.data ?? []).map((s) => ({
            label: s.name,
            value: s.id,
            description: t('suites.cellsOf', {
              enabled: s.summary?.enabled_cell_count ?? 0,
              total: s.summary?.cell_count ?? 0,
            }),
          })),
    [targetKind, tests.data, suites.data, t]
  )

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        void form.handleSubmit()
      }}
      noValidate
    >
      <div className={styles.grid}>
        <div>
          <form.Field name="name">
            {(field) => (
              <Field
                label={t('schedules.form.name')}
                htmlFor={field.name}
                required
                invalid={field.state.meta.errors.length > 0}
                error={firstError(field.state.meta)}
              >
                <Input
                  id={field.name}
                  value={field.state.value}
                  placeholder={t('schedules.form.namePlaceholder')}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                  autoFocus
                />
              </Field>
            )}
          </form.Field>

          <form.Field name="targetKind">
            {(field) => (
              <Field label={t('schedules.form.targetKind')}>
                <RadioButtonGroup<'test' | 'suite'>
                  options={[
                    { label: t('schedules.target.test'), value: 'test', icon: 'vial' },
                    { label: t('schedules.target.suite'), value: 'suite', icon: 'layer-group' },
                  ]}
                  value={field.state.value}
                  onChange={(v) => {
                    field.handleChange(v)
                    form.setFieldValue('targetId', '')
                  }}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="targetId">
            {(field) => (
              <Field
                label={t('schedules.form.target')}
                htmlFor={field.name}
                required
                invalid={field.state.meta.errors.length > 0}
                error={firstError(field.state.meta)}
              >
                <Combobox
                  id={field.name}
                  options={targetOptions}
                  value={field.state.value || null}
                  isClearable
                  loading={targetKind === 'test' ? tests.isPending : suites.isPending}
                  placeholder={
                    targetKind === 'test'
                      ? t('schedules.form.targetPlaceholderTest')
                      : t('schedules.form.targetPlaceholderSuite')
                  }
                  onBlur={field.handleBlur}
                  onChange={(o) => field.handleChange(o?.value ?? '')}
                />
              </Field>
            )}
          </form.Field>

          <form.Field name="cron">
            {(field) => (
              <Field
                label={t('schedules.form.cron')}
                description={t('schedules.form.cronHint')}
                htmlFor={field.name}
                required
                invalid={field.state.meta.errors.length > 0}
                error={firstError(field.state.meta)}
              >
                {/* biome-ignore lint/complexity/noUselessFragments: Field clones `invalid` onto its direct child; a Fragment keeps it off the DOM */}
                <>
                  <div className={styles.presets}>
                    {PRESETS.map((p) => (
                      <Button
                        key={p.key}
                        size="sm"
                        variant={field.state.value.trim() === p.cron ? 'primary' : 'secondary'}
                        fill="outline"
                        type="button"
                        onClick={() => field.handleChange(p.cron)}
                      >
                        {t(`schedules.form.presets.${p.key}`)}
                      </Button>
                    ))}
                  </div>
                  <Input
                    id={field.name}
                    className={styles.mono}
                    width={30}
                    value={field.state.value}
                    placeholder="0 2 * * *"
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.currentTarget.value)}
                  />
                </>
              </Field>
            )}
          </form.Field>

          <form.Field name="timezone">
            {(field) => (
              <Field
                label={t('schedules.form.timezone')}
                htmlFor={field.name}
                invalid={field.state.meta.errors.length > 0}
                error={firstError(field.state.meta)}
              >
                {/* biome-ignore lint/complexity/noUselessFragments: Field clones `invalid` onto its direct child; a Fragment keeps it off the DOM */}
                <>
                  <Combobox
                    id={field.name}
                    options={zones}
                    value={field.state.value}
                    width={30}
                    onBlur={field.handleBlur}
                    onChange={(o) => field.handleChange(o.value)}
                  />
                  {field.state.value !== localTimeZone() && (
                    <Button
                      size="sm"
                      variant="secondary"
                      fill="text"
                      type="button"
                      onClick={() => field.handleChange(localTimeZone())}
                    >
                      {t('schedules.form.timezoneLocal')}: {localTimeZone()}
                    </Button>
                  )}
                </>
              </Field>
            )}
          </form.Field>

          <form.Field name="enabled">
            {(field) => (
              <Field
                label={t('schedules.form.enabled')}
                description={t('schedules.form.enabledHint')}
                htmlFor={field.name}
              >
                <Switch
                  id={field.name}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.currentTarget.checked)}
                />
              </Field>
            )}
          </form.Field>

          <FieldSet label={t('schedules.form.overrides')}>
            <Text color="secondary" variant="bodySmall">
              {t('schedules.form.overridesHint')}
            </Text>
            <form.Field name="providerId">
              {(field) => (
                <Field label={t('schedules.form.provider')} htmlFor={field.name}>
                  <Combobox
                    id={field.name}
                    isClearable
                    width={30}
                    loading={providers.isPending}
                    placeholder={t('schedules.form.providerPlaceholder')}
                    options={(providers.data?.data ?? []).map((p) => ({
                      label: p.name,
                      value: p.id,
                      description: `${p.kind} · ${t(`common.status.${p.status}`)}`,
                    }))}
                    value={field.state.value || null}
                    onChange={(o) => field.handleChange(o?.value ?? '')}
                  />
                </Field>
              )}
            </form.Field>
            <form.Field name="sizesOn">
              {(field) => (
                <Field htmlFor={field.name} label={t('schedules.form.sizesToggle')}>
                  <Switch
                    id={field.name}
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.currentTarget.checked)}
                  />
                </Field>
              )}
            </form.Field>
            {sizesOn && (
              <form.Field name="sizes">
                {(field) => (
                  <Field label={t('common.fields.sizes')}>
                    <RoleSizesEditor
                      value={field.state.value}
                      roles={['db', 'runner']}
                      onChange={field.handleChange}
                    />
                  </Field>
                )}
              </form.Field>
            )}
            <form.Field name="keep">
              {(field) => (
                <Field
                  label={t('schedules.form.keep')}
                  htmlFor={field.name}
                  invalid={field.state.meta.errors.length > 0}
                  error={firstError(field.state.meta)}
                >
                  <Input
                    id={field.name}
                    width={20}
                    placeholder={t('schedules.form.keepPlaceholder')}
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.currentTarget.value)}
                  />
                </Field>
              )}
            </form.Field>
            <form.Field name="ratingOn">
              {(field) => (
                <Field htmlFor={field.name} label={t('schedules.form.rating')}>
                  <Switch
                    id={field.name}
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.currentTarget.checked)}
                  />
                </Field>
              )}
            </form.Field>
            {ratingOn && (
              <Stack gap={2}>
                <form.Field name="ratingTenant">
                  {(field) => (
                    <Checkbox
                      id={field.name}
                      label={t('schedules.form.ratingTenant')}
                      value={field.state.value}
                      onChange={(e) => field.handleChange(e.currentTarget.checked)}
                    />
                  )}
                </form.Field>
                <form.Field name="ratingGlobal">
                  {(field) => (
                    <Checkbox
                      id={field.name}
                      label={t('schedules.form.ratingGlobal')}
                      value={field.state.value}
                      onChange={(e) => field.handleChange(e.currentTarget.checked)}
                    />
                  )}
                </form.Field>
              </Stack>
            )}
          </FieldSet>

          {serverError && <Alert severity="error" title={serverError} />}
          <form.Subscribe selector={(s) => [s.isSubmitting, s.errorMap.onServer] as const}>
            {([isSubmitting, onServer]) => (
              <Stack direction="column" gap={1}>
                {typeof onServer === 'string' && <Alert severity="error" title={onServer} />}
                <Stack gap={1} justifyContent="flex-end">
                  {onCancel && (
                    <Button type="button" variant="secondary" fill="outline" onClick={onCancel}>
                      {t('common.actions.cancel')}
                    </Button>
                  )}
                  <Button
                    type="submit"
                    disabled={isSubmitting}
                    icon={isSubmitting ? 'spinner' : 'save'}
                  >
                    {submitLabel}
                  </Button>
                </Stack>
              </Stack>
            )}
          </form.Subscribe>
        </div>
        <aside className={styles.sticky}>
          <CronPreview cron={cron} timezone={isValidTimeZone(timezone) ? timezone : 'UTC'} />
        </aside>
      </div>
    </form>
  )
}

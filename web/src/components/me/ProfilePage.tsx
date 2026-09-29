import { meMutations, meQueries } from '@api/queries/me'
import type { Me } from '@api/types'
import { toast } from '@app/Toaster'
import { useThemeMode } from '@app/theme-context'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Button,
  Combobox,
  Field,
  FieldSet,
  Input,
  RadioButtonGroup,
  Stack,
  Switch,
  Text,
  TimeZonePicker,
  useStyles2,
} from '@grafana/ui'
import { applyServerErrors, firstError } from '@helpers/form'
import { useMe } from '@hooks/useMe'
import { revalidateLogic, useForm } from '@tanstack/react-form'
import { useQueryClient } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

const getStyles = (theme: GrafanaTheme2) => ({
  form: css({ maxWidth: 720 }),
  preview: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    marginBottom: theme.spacing(2),
  }),
  avatar: css({
    width: 56,
    height: 56,
    borderRadius: '50%',
    display: 'grid',
    placeItems: 'center',
    overflow: 'hidden',
    border: `1px solid ${theme.colors.border.medium}`,
    background: theme.colors.background.secondary,
    fontSize: theme.typography.h3.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    img: { width: '100%', height: '100%', objectFit: 'cover' },
  }),
})

type Theme = NonNullable<Me['preferences']['theme']>

function identicon(seed: string): string {
  // Deterministic 5×5 symmetric pattern; enough for a visual identity without an upload.
  let h = 0
  for (const ch of seed) h = (h * 31 + ch.charCodeAt(0)) >>> 0
  const cells: string[] = []
  for (let y = 0; y < 5; y++)
    for (let x = 0; x < 3; x++) {
      h = (h * 1664525 + 1013904223) >>> 0
      if (h & 0x8000) {
        cells.push(`<rect x="${x}" y="${y}" width="1" height="1"/>`)
        if (x < 2) cells.push(`<rect x="${4 - x}" y="${y}" width="1" height="1"/>`)
      }
    }
  const hue = h % 360
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="-0.5 -0.5 6 6" shape-rendering="crispEdges"><g fill="hsl(${hue} 55% 55%)">${cells.join('')}</g></svg>`
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`
}

export function ProfilePage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const me = useMe()
  const qc = useQueryClient()
  const { setMode } = useThemeMode()
  const schema = useMemo(
    () =>
      z.object({
        // MePatch.display_name
        display_name: z.string().trim().min(1, t('common.validation.required')).max(64),
        avatar_kind: z.enum(['identicon', 'url']),
        avatar_url: z.string().trim(),
        theme: z.enum(['system', 'light', 'dark']),
        timezone: z.string(),
        default_tenant: z.string(),
        run_finished: z.boolean(),
        run_failed: z.boolean(),
        suite_finished: z.boolean(),
      }),
    [t]
  )
  const isUrl = !!me.avatar && /^https?:\/\//.test(me.avatar)
  const form = useForm({
    defaultValues: {
      display_name: me.display_name,
      avatar_kind: (isUrl ? 'url' : 'identicon') as 'identicon' | 'url',
      avatar_url: isUrl ? (me.avatar as string) : '',
      theme: (me.preferences.theme ?? 'system') as Theme,
      timezone: me.preferences.timezone ?? '',
      default_tenant: me.preferences.default_tenant ?? '',
      run_finished: me.notifications.run_finished ?? true,
      run_failed: me.notifications.run_failed ?? true,
      suite_finished: me.notifications.suite_finished ?? false,
    },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    validators: {
      onDynamic: schema.refine(
        (v) => v.avatar_kind !== 'url' || /^https:\/\/\S+$/.test(v.avatar_url),
        { message: t('common.validation.url'), path: ['avatar_url'] }
      ),
    },
    onSubmit: async ({ value, formApi }) => {
      try {
        const next = await meMutations.patch({
          display_name: value.display_name.trim(),
          avatar: value.avatar_kind === 'url' ? value.avatar_url.trim() : 'identicon',
          preferences: {
            theme: value.theme,
            timezone: value.timezone || undefined,
            default_tenant: value.default_tenant || null,
          },
          notifications: {
            run_finished: value.run_finished,
            run_failed: value.run_failed,
            suite_finished: value.suite_finished,
          },
        })
        qc.setQueryData(meQueries.me().queryKey, next)
        formApi.reset(value)
        toast.success(t('me.profile.saved'))
      } catch (e) {
        applyServerErrors(formApi, e, t('common.errors.generic'))
      }
    },
  })

  const applyTheme = (theme: Theme) => {
    if (theme === 'system')
      setMode(window.matchMedia?.('(prefers-color-scheme: light)').matches ? 'light' : 'dark')
    else setMode(theme)
  }

  return (
    <form
      className={styles.form}
      onSubmit={(e) => {
        e.preventDefault()
        void form.handleSubmit()
      }}
    >
      <FieldSet label={t('me.profile.identity')}>
        <form.Subscribe
          selector={(s) =>
            [s.values.avatar_kind, s.values.avatar_url, s.values.display_name] as const
          }
        >
          {([kind, url, name]) => (
            <div className={styles.preview}>
              <div className={styles.avatar} aria-hidden>
                {kind === 'url' && /^https:\/\/\S+$/.test(url) ? (
                  <img src={url} alt="" />
                ) : (
                  <img src={identicon(me.id)} alt="" />
                )}
              </div>
              <div>
                <Text weight="medium">{name || me.display_name}</Text>
                <br />
                <Text color="secondary" variant="bodySmall">
                  {me.email}
                </Text>
              </div>
            </div>
          )}
        </form.Subscribe>
        <form.Field name="display_name">
          {(field) => (
            <Field
              label={t('me.profile.displayName')}
              invalid={!!firstError(field.state.meta)}
              error={firstError(field.state.meta)}
              htmlFor={field.name}
              required
            >
              <Input
                id={field.name}
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.currentTarget.value)}
              />
            </Field>
          )}
        </form.Field>
        <form.Field name="avatar_kind">
          {(field) => (
            <Field label={t('me.profile.avatar')} description={t('me.profile.avatarHint')}>
              <RadioButtonGroup
                options={[
                  { label: t('me.profile.avatarIdenticon'), value: 'identicon' as const },
                  { label: t('me.profile.avatarUrl'), value: 'url' as const },
                ]}
                value={field.state.value}
                onChange={(v) => field.handleChange(v)}
              />
            </Field>
          )}
        </form.Field>
        <form.Subscribe selector={(s) => s.values.avatar_kind}>
          {(kind) =>
            kind === 'url' ? (
              <form.Field name="avatar_url">
                {(field) => (
                  <Field
                    label={t('me.profile.avatarUrlLabel')}
                    invalid={!!firstError(field.state.meta)}
                    error={firstError(field.state.meta)}
                    htmlFor={field.name}
                  >
                    <Input
                      id={field.name}
                      placeholder="https://…/avatar.png"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.currentTarget.value)}
                    />
                  </Field>
                )}
              </form.Field>
            ) : null
          }
        </form.Subscribe>
      </FieldSet>

      <FieldSet label={t('me.profile.preferences')}>
        <form.Field name="theme">
          {(field) => (
            <Field label={t('nav.theme')} description={t('me.profile.themeHint')}>
              <RadioButtonGroup
                options={(['system', 'dark', 'light'] as const).map((v) => ({
                  label: t(`common.theme.${v}`),
                  value: v,
                }))}
                value={field.state.value}
                onChange={(v) => {
                  field.handleChange(v)
                  applyTheme(v)
                }}
              />
            </Field>
          )}
        </form.Field>
        <form.Field name="timezone">
          {(field) => (
            <Field
              label={t('common.fields.timezone')}
              description={t('me.profile.timezoneHint')}
              htmlFor={field.name}
            >
              <TimeZonePicker
                inputId={field.name}
                value={field.state.value || undefined}
                width={40}
                onChange={(tz) => field.handleChange(tz ?? '')}
                onBlur={field.handleBlur}
              />
            </Field>
          )}
        </form.Field>
        <form.Field name="default_tenant">
          {(field) => (
            <Field
              label={t('me.profile.defaultTenant')}
              description={t('me.profile.defaultTenantHint')}
              htmlFor={field.name}
            >
              <Combobox
                id={field.name}
                width={40}
                isClearable
                options={me.tenants.map((m) => ({
                  label: m.tenant.name,
                  value: m.tenant.slug,
                  description: `${m.tenant.slug} · ${t(`common.role.${m.role}`)}`,
                }))}
                value={field.state.value || null}
                onChange={(o) => field.handleChange(o?.value ?? '')}
              />
            </Field>
          )}
        </form.Field>
      </FieldSet>

      <FieldSet label={t('me.profile.notifications')}>
        {(['run_finished', 'run_failed', 'suite_finished'] as const).map((key) => (
          <form.Field key={key} name={key}>
            {(field) => (
              <Field
                label={t(`me.profile.notify.${key}`)}
                description={t(`me.profile.notifyHint.${key}`)}
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
        ))}
      </FieldSet>

      <form.Subscribe selector={(s) => [s.isSubmitting, s.isDirty, s.errorMap.onServer] as const}>
        {([isSubmitting, isDirty, serverError]) => (
          <Stack direction="column" gap={1}>
            <ServerErrorAlert error={serverError} />
            <Stack gap={1}>
              <Button type="submit" disabled={isSubmitting} icon="save">
                {isSubmitting ? t('common.misc.saving') : t('common.actions.save')}
              </Button>
              <Button
                type="button"
                variant="secondary"
                disabled={!isDirty || isSubmitting}
                onClick={() => {
                  form.reset()
                  applyTheme(me.preferences.theme ?? 'system')
                }}
              >
                {t('common.actions.reset')}
              </Button>
            </Stack>
          </Stack>
        )}
      </form.Subscribe>
    </form>
  )
}

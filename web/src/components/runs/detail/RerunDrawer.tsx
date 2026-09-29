import { fieldErrorsFrom, isApiError } from '@api/errors'
import type { LaunchOverrides, Run } from '@api/types'
import { TagsEditor } from '@components/TagsEditor'
import { Alert, Button, Drawer, Field, Input, Stack, Switch, Text, TextArea } from '@grafana/ui'
import { useForm } from '@tanstack/react-form'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

export type RerunBody = LaunchOverrides & { resume?: boolean }

// "Rerun with overrides" (clone): the snapshot is the default, the user tweaks name/keep/labels/notes.
export function RerunDrawer({
  run,
  mode,
  onClose,
  onSubmit,
}: {
  run: Run
  mode: 'clone' | 'resume'
  onClose: () => void
  onSubmit: (body: RerunBody) => Promise<unknown>
}) {
  const { t } = useTranslation()
  const schema = useMemo(
    () =>
      z.object({
        // LaunchOverrides.name / keep (Go duration, openapi LaunchOverrides)
        name: z.string().trim().min(1, t('common.validation.required')).max(120),
        keep: z
          .string()
          .trim()
          .regex(/^(\d+(h|m)|0s)?$/, t('common.validation.duration')),
        labels: z.record(z.string(), z.string()),
        notes: z.string(),
        resume: z.boolean(),
        rating_tenant: z.boolean(),
        rating_global: z.boolean(),
      }),
    [t]
  )
  const baseName = run.name.replace(/ #\d+$/, '')
  const form = useForm({
    defaultValues: {
      name: `${baseName} #${t('runs.rerun.newSuffix')}`,
      keep: run.snapshot.keep ?? '',
      labels: { ...(run.labels ?? {}) },
      notes: '',
      resume: mode === 'resume',
      rating_tenant: run.rating?.tenant ?? true,
      rating_global: run.rating?.global ?? false,
    },
    validators: { onSubmit: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        await onSubmit({
          name: value.name.trim(),
          keep: value.keep.trim() || undefined,
          labels: value.labels,
          notes: value.notes.trim() || undefined,
          rating: { tenant: value.rating_tenant, global: value.rating_global },
          resume: value.resume || undefined,
        })
      } catch (e) {
        if (isApiError(e)) {
          formApi.setErrorMap({
            onServer: {
              fields: fieldErrorsFrom(e.validation),
              form: e.validation ? undefined : e.detail || e.title,
            } as never,
          })
        } else throw e
      }
    },
  })
  return (
    <Drawer
      title={mode === 'resume' ? t('runs.actions.rerunResume') : t('runs.actions.clone')}
      subtitle={t('runs.rerun.subtitle', { name: run.name })}
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
          {mode === 'resume' && !run.stand_kept && (
            <Alert severity="warning" title={t('runs.rerun.resumeDegradedTitle')}>
              {t('runs.rerun.resumeDegraded')}
            </Alert>
          )}
          {mode === 'resume' && run.stand_kept && (
            <Alert severity="info" title={t('runs.rerun.resumeReadyTitle')}>
              {t('runs.rerun.resumeReady')}
            </Alert>
          )}
          <form.Field name="name">
            {(field) => (
              <Field
                label={t('common.fields.name')}
                htmlFor={field.name}
                invalid={field.state.meta.errors.length > 0}
                error={field.state.meta.errors[0]?.message}
              >
                <Input
                  id={field.name}
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                  autoFocus
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="keep">
            {(field) => (
              <Field
                label={t('runs.rerun.keep')}
                description={t('runs.rerun.keepHint')}
                htmlFor={field.name}
                invalid={field.state.meta.errors.length > 0}
                error={field.state.meta.errors[0]?.message}
              >
                <Input
                  id={field.name}
                  width={16}
                  placeholder="0s"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="labels">
            {(field) => (
              <Field label={t('common.fields.labels')} htmlFor={field.name}>
                <TagsEditor
                  value={field.state.value}
                  onChange={(v) => field.handleChange(v)}
                  placeholder="key=value"
                />
              </Field>
            )}
          </form.Field>
          <Stack gap={3}>
            <form.Field name="rating_tenant">
              {(field) => (
                <Field label={t('runs.overview.rating.tenant')} htmlFor={field.name}>
                  <Switch
                    id={field.name}
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.currentTarget.checked)}
                  />
                </Field>
              )}
            </form.Field>
            <form.Field name="rating_global">
              {(field) => (
                <Field label={t('runs.overview.rating.global')} htmlFor={field.name}>
                  <Switch
                    id={field.name}
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.currentTarget.checked)}
                  />
                </Field>
              )}
            </form.Field>
          </Stack>
          <form.Field name="notes">
            {(field) => (
              <Field label={t('common.fields.notes')} htmlFor={field.name}>
                <TextArea
                  id={field.name}
                  rows={3}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )}
          </form.Field>
          {mode === 'resume' && (
            <form.Field name="resume">
              {(field) => (
                <Field
                  label={t('runs.rerun.resumeFlag')}
                  description={t('runs.rerun.resumeFlagHint')}
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
          )}
          <form.Subscribe selector={(s) => [s.isSubmitting, s.errorMap] as const}>
            {([busy, errorMap]) => {
              const server = (errorMap as { onServer?: { form?: string } }).onServer?.form
              return (
                <Stack direction="column" gap={1}>
                  {server && <Text color="error">{server}</Text>}
                  <Stack justifyContent="flex-end" gap={1}>
                    <Button variant="secondary" fill="outline" type="button" onClick={onClose}>
                      {t('common.actions.cancel')}
                    </Button>
                    <Button type="submit" icon="play" disabled={busy}>
                      {t('common.actions.launch')}
                    </Button>
                  </Stack>
                </Stack>
              )
            }}
          </form.Subscribe>
        </Stack>
      </form>
    </Drawer>
  )
}

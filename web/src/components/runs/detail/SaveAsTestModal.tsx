import { fieldErrorsFrom, isApiError } from '@api/errors'
import type { Run } from '@api/types'
import { Button, Checkbox, Field, Input, Modal, Stack, Text } from '@grafana/ui'
import { useForm } from '@tanstack/react-form'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

// Snapshot → Test. Optionally also stores the inline database/workload as library records.
export function SaveAsTestModal({
  run,
  isOpen,
  onClose,
  onSubmit,
}: {
  run: Run
  isOpen: boolean
  onClose: () => void
  onSubmit: (body: {
    name: string
    save_database_as?: string
    save_workload_as?: string
  }) => Promise<unknown>
}) {
  const { t } = useTranslation()
  const schema = useMemo(
    () =>
      z.object({
        name: z.string().trim().min(1, t('common.validation.required')).max(120),
        saveDb: z.boolean(),
        dbName: z.string().trim(),
        saveWl: z.boolean(),
        wlName: z.string().trim(),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: {
      name: run.test_ref.name ? `${run.test_ref.name} (${t('runs.saveAsTest.copy')})` : run.name,
      saveDb: false,
      dbName:
        run.snapshot.database_name ??
        `${run.snapshot.database.kind}-${run.snapshot.database.version}`,
      saveWl: false,
      wlName: run.snapshot.workload_name ?? run.summary?.workload_name ?? 'workload',
    },
    validators: { onSubmit: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        await onSubmit({
          name: value.name.trim(),
          save_database_as: value.saveDb ? value.dbName.trim() || undefined : undefined,
          save_workload_as: value.saveWl ? value.wlName.trim() || undefined : undefined,
        })
      } catch (e) {
        if (isApiError(e))
          formApi.setErrorMap({
            onServer: {
              fields: fieldErrorsFrom(e.validation),
              form: e.validation ? undefined : e.detail || e.title,
            } as never,
          })
        else throw e
      }
    },
  })
  return (
    <Modal title={t('runs.actions.saveAsTest')} isOpen={isOpen} onDismiss={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          void form.handleSubmit()
        }}
      >
        <Text color="secondary">{t('runs.saveAsTest.hint')}</Text>
        <div style={{ height: 8 }} />
        <form.Field name="name">
          {(field) => (
            <Field
              label={t('runs.saveAsTest.name')}
              htmlFor={field.name}
              invalid={field.state.meta.errors.length > 0}
              error={field.state.meta.errors[0]?.message}
            >
              <Input
                id={field.name}
                autoFocus
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.currentTarget.value)}
              />
            </Field>
          )}
        </form.Field>
        <form.Field name="saveDb">
          {(field) => (
            <Checkbox
              id={field.name}
              label={t('runs.saveAsTest.saveDb')}
              description={t('runs.saveAsTest.saveDbHint', {
                kind: run.snapshot.database.kind,
                version: run.snapshot.database.version,
              })}
              value={field.state.value}
              onChange={(e) => field.handleChange(e.currentTarget.checked)}
            />
          )}
        </form.Field>
        <form.Subscribe selector={(s) => s.values.saveDb}>
          {(on) =>
            on ? (
              <form.Field name="dbName">
                {(field) => (
                  <Field label={t('runs.saveAsTest.dbName')} htmlFor={field.name}>
                    <Input
                      id={field.name}
                      value={field.state.value}
                      onChange={(e) => field.handleChange(e.currentTarget.value)}
                    />
                  </Field>
                )}
              </form.Field>
            ) : null
          }
        </form.Subscribe>
        <div style={{ height: 8 }} />
        <form.Field name="saveWl">
          {(field) => (
            <Checkbox
              id={field.name}
              label={t('runs.saveAsTest.saveWl')}
              description={t('runs.saveAsTest.saveWlHint', {
                count: run.snapshot.workload.segments.length,
              })}
              value={field.state.value}
              onChange={(e) => field.handleChange(e.currentTarget.checked)}
            />
          )}
        </form.Field>
        <form.Subscribe selector={(s) => s.values.saveWl}>
          {(on) =>
            on ? (
              <form.Field name="wlName">
                {(field) => (
                  <Field label={t('runs.saveAsTest.wlName')} htmlFor={field.name}>
                    <Input
                      id={field.name}
                      value={field.state.value}
                      onChange={(e) => field.handleChange(e.currentTarget.value)}
                    />
                  </Field>
                )}
              </form.Field>
            ) : null
          }
        </form.Subscribe>
        <form.Subscribe selector={(s) => [s.isSubmitting, s.errorMap] as const}>
          {([busy, errorMap]) => {
            const server = (errorMap as { onServer?: { form?: string } }).onServer?.form
            return (
              <Stack direction="column" gap={1}>
                {server && <Text color="error">{server}</Text>}
                <Modal.ButtonRow>
                  <Button variant="secondary" fill="outline" type="button" onClick={onClose}>
                    {t('common.actions.cancel')}
                  </Button>
                  <Button type="submit" icon="save" disabled={busy}>
                    {t('common.actions.save')}
                  </Button>
                </Modal.ButtonRow>
              </Stack>
            )
          }}
        </form.Subscribe>
      </form>
    </Modal>
  )
}

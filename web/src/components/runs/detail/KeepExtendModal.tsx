import { isApiError } from '@api/errors'
import type { Run } from '@api/types'
import { Button, Field, Input, Modal, RadioButtonGroup, Stack, Text } from '@grafana/ui'
import { formatDateTime } from '@helpers/time'
import { useForm } from '@tanstack/react-form'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

// Extend the kept stand by a Go duration (30m, 2h, …).
export function KeepExtendModal({
  run,
  isOpen,
  onClose,
  onSubmit,
}: {
  run: Run
  isOpen: boolean
  onClose: () => void
  onSubmit: (duration: string) => Promise<unknown>
}) {
  const { t } = useTranslation()
  const schema = useMemo(
    // POST :keep-extend {duration}: Go duration in hours/minutes
    () =>
      z.object({
        duration: z
          .string()
          .trim()
          .regex(/^\d+(h|m)$/, t('common.validation.duration')),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: { duration: '2h' },
    validators: { onSubmit: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        await onSubmit(value.duration.trim())
      } catch (e) {
        if (isApiError(e))
          formApi.setErrorMap({
            onServer: { fields: { duration: e.detail || e.title } } as never,
          })
        else throw e
      }
    },
  })
  return (
    <Modal title={t('runs.keep.extendTitle')} isOpen={isOpen} onDismiss={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          void form.handleSubmit()
        }}
      >
        <Text color="secondary">
          {run.keep_until
            ? t('runs.keptUntil', { time: formatDateTime(run.keep_until) })
            : t('runs.keep.notKept')}
        </Text>
        <div style={{ height: 8 }} />
        <form.Field name="duration">
          {(field) => (
            <Field
              label={t('runs.keep.duration')}
              description={t('runs.keep.durationHint')}
              htmlFor={field.name}
              invalid={field.state.meta.errors.length > 0}
              error={field.state.meta.errors[0]?.message}
            >
              <Stack gap={1}>
                <Input
                  id={field.name}
                  width={12}
                  autoFocus
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
                <RadioButtonGroup
                  size="sm"
                  value={field.state.value}
                  options={['30m', '1h', '2h', '8h', '24h'].map((v) => ({ value: v, label: v }))}
                  onChange={(v) => field.handleChange(v)}
                />
              </Stack>
            </Field>
          )}
        </form.Field>
        <Modal.ButtonRow>
          <Button variant="secondary" fill="outline" type="button" onClick={onClose}>
            {t('common.actions.cancel')}
          </Button>
          <form.Subscribe selector={(s) => s.isSubmitting}>
            {(busy) => (
              <Button type="submit" icon="lock" disabled={busy}>
                {t('runs.keep.extend')}
              </Button>
            )}
          </form.Subscribe>
        </Modal.ButtonRow>
      </form>
    </Modal>
  )
}

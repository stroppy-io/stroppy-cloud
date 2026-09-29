import type { Share } from '@api/types'
import { toast } from '@app/Toaster'
import { CopyText } from '@components/CopyText'
import {
  Alert,
  Button,
  Field,
  Input,
  Modal,
  RadioButtonGroup,
  Stack,
  Text,
  TextLink,
} from '@grafana/ui'
import { formatDateTime } from '@helpers/time'
import { useForm } from '@tanstack/react-form'
import { useMutation } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

export interface ShareInput {
  scope: Share['scope']
  ttl: string
  title: string
}

// Create a public share link (run or comparison). Shows the link with copy once created.
export function ShareModal({
  isOpen,
  onClose,
  defaultTitle,
  onCreate,
}: {
  isOpen: boolean
  onClose: () => void
  defaultTitle: string
  onCreate: (input: ShareInput) => Promise<Share>
}) {
  const { t } = useTranslation()
  const [created, setCreated] = useState<Share | undefined>()
  // ShareCreate.ttl: Go duration, `0s` = never (openapi ShareCreate)
  const schema = useMemo(
    () =>
      z.object({
        scope: z.enum(['overview', 'metrics', 'configs']),
        ttl: z.string().regex(/^(\d+(h|m|d)|0s)$/, t('runs.share.ttlHint')),
        title: z
          .string()
          .trim()
          .max(120, t('common.validation.max', { max: 120 })),
      }),
    [t]
  )
  const m = useMutation({
    mutationFn: onCreate,
    onSuccess: (s) => {
      setCreated(s)
      toast.success(t('runs.share.created'))
    },
    onError: (e) => toast.error(e),
  })
  const form = useForm({
    defaultValues: { scope: 'overview', ttl: '7d', title: defaultTitle } as ShareInput,
    validators: { onSubmit: schema },
    onSubmit: ({ value }) => m.mutateAsync(value),
  })
  const close = () => {
    setCreated(undefined)
    form.reset()
    onClose()
  }
  const absolute = created
    ? created.url.startsWith('http')
      ? created.url
      : `${window.location.origin}${created.url}`
    : ''
  return (
    <Modal title={t('runs.share.title')} isOpen={isOpen} onDismiss={close}>
      {created ? (
        <Stack direction="column" gap={2}>
          <Alert severity="success" title={t('runs.share.ready')}>
            {created.expires_at
              ? t('runs.share.expiresAt', { time: formatDateTime(created.expires_at) })
              : t('runs.share.neverExpires')}
          </Alert>
          <CopyText value={absolute} />
          <Text color="secondary" variant="bodySmall">
            {t('runs.share.scopeLabel')}: {t(`runs.share.scope.${created.scope}`)} ·{' '}
            <TextLink href={absolute} external>
              {t('common.actions.open')}
            </TextLink>
          </Text>
          <Modal.ButtonRow>
            <Button onClick={close}>{t('common.actions.close')}</Button>
          </Modal.ButtonRow>
        </Stack>
      ) : (
        <form
          onSubmit={(e) => {
            e.preventDefault()
            void form.handleSubmit()
          }}
        >
          <form.Field name="scope">
            {(field) => (
              <Field
                label={t('runs.share.scopeLabel')}
                description={t('runs.share.scopeHint')}
                htmlFor={field.name}
              >
                <RadioButtonGroup
                  id={field.name}
                  value={field.state.value}
                  options={(['overview', 'metrics', 'configs'] as Share['scope'][]).map((s) => ({
                    value: s,
                    label: t(`runs.share.scope.${s}`),
                  }))}
                  onChange={(v) => field.handleChange(v)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="ttl">
            {(field) => (
              <Field
                label={t('runs.share.ttl')}
                description={t('runs.share.ttlHint')}
                htmlFor={field.name}
                invalid={field.state.meta.errors.length > 0}
                error={field.state.meta.errors[0]?.message}
              >
                <Stack gap={1}>
                  <Input
                    id={field.name}
                    width={14}
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.currentTarget.value.trim())}
                  />
                  <RadioButtonGroup
                    size="sm"
                    value={field.state.value}
                    options={['1d', '7d', '30d', '0s'].map((v) => ({
                      value: v,
                      label: v === '0s' ? t('runs.share.never') : v,
                    }))}
                    onChange={(v) => field.handleChange(v)}
                  />
                </Stack>
              </Field>
            )}
          </form.Field>
          <form.Field name="title">
            {(field) => (
              <Field
                label={t('runs.share.titleField')}
                htmlFor={field.name}
                invalid={field.state.meta.errors.length > 0}
                error={field.state.meta.errors[0]?.message}
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
          <Modal.ButtonRow>
            <Button variant="secondary" fill="outline" onClick={close} type="button">
              {t('common.actions.cancel')}
            </Button>
            <form.Subscribe selector={(s) => s.isSubmitting}>
              {(busy) => (
                <Button type="submit" icon="share-alt" disabled={busy}>
                  {t('runs.share.create')}
                </Button>
              )}
            </form.Subscribe>
          </Modal.ButtonRow>
        </form>
      )}
    </Modal>
  )
}

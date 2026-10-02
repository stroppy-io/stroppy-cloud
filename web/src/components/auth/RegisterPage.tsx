import { Page } from '@app/Page'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Button, Field, Input, Stack, Text, useStyles2 } from '@grafana/ui'
import { applyServerErrors, firstError } from '@helpers/form'
import { register, registrationOpen } from '@lib/auth'
import { useForm } from '@tanstack/react-form'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

const getStyles = (theme: GrafanaTheme2) => ({
  card: css({
    width: 380,
    margin: `${theme.spacing(10)} auto 0`,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(3),
  }),
})

/** Kratos registration: an account + a session; the e-mail waits for confirmation. */
export function RegisterPage({ next }: { next?: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const qc = useQueryClient()
  const navigate = useNavigate()
  // A closed platform (registration flow off in Kratos) is learned by probing.
  const [closed, setClosed] = useState(false)
  useEffect(() => {
    void registrationOpen().then(setClosed)
  }, [])

  const schema = useMemo(
    () =>
      z.object({
        email: z.string().trim().email(t('common.validation.email')),
        name: z
          .string()
          .trim()
          .max(80, t('common.validation.max', { max: 80 })),
        password: z.string().min(8, t('auth.register.passwordMin')),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: { email: '', name: '', password: '' },
    validators: { onDynamic: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        await register(value.email.trim(), value.name.trim(), value.password)
        qc.clear()
        void navigate({ to: next?.startsWith('/') ? next : '/' })
      } catch (e) {
        applyServerErrors(formApi, e, e instanceof Error ? e.message : t('common.errors.generic'))
      }
    },
  })

  return (
    <Page>
      <div className={styles.card}>
        <Stack direction="column" gap={2}>
          <Text variant="h4" weight="medium">
            {t('auth.register.title')}
          </Text>
          {closed ? (
            <Alert severity="warning" title={t('auth.register.closedTitle')}>
              {t('auth.register.closedBody')}
            </Alert>
          ) : (
            <form
              onSubmit={(e) => {
                e.preventDefault()
                void form.handleSubmit()
              }}
            >
              <Stack direction="column" gap={1}>
                <form.Field name="email">
                  {(field) => (
                    <Field
                      label={t('auth.register.email')}
                      invalid={!!firstError(field.state.meta)}
                      error={firstError(field.state.meta)}
                      htmlFor={field.name}
                      required
                    >
                      <Input
                        id={field.name}
                        autoFocus
                        type="email"
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) => field.handleChange(e.currentTarget.value)}
                      />
                    </Field>
                  )}
                </form.Field>
                <form.Field name="name">
                  {(field) => (
                    <Field
                      label={t('auth.register.name')}
                      invalid={!!firstError(field.state.meta)}
                      error={firstError(field.state.meta)}
                      htmlFor={field.name}
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
                <form.Field name="password">
                  {(field) => (
                    <Field
                      label={t('auth.register.password')}
                      description={t('auth.register.passwordHint')}
                      invalid={!!firstError(field.state.meta)}
                      error={firstError(field.state.meta)}
                      htmlFor={field.name}
                      required
                    >
                      <Input
                        id={field.name}
                        type="password"
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) => field.handleChange(e.currentTarget.value)}
                      />
                    </Field>
                  )}
                </form.Field>
                <Button type="submit" disabled={form.state.isSubmitting}>
                  {t('auth.register.submit')}
                </Button>
              </Stack>
            </form>
          )}
          {!closed && (
            <Alert severity="info" title={t('auth.register.verifyTitle')}>
              {t('auth.register.verifyBody')}
            </Alert>
          )}
          <Text variant="bodySmall" color="secondary">
            {t('auth.register.hasAccount')}{' '}
            <Button
              size="sm"
              variant="secondary"
              fill="text"
              type="button"
              onClick={() => void navigate({ to: '/login', search: { next } })}
            >
              {t('auth.register.login')}
            </Button>
          </Text>
        </Stack>
      </div>
    </Page>
  )
}

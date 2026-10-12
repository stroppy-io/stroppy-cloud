import { Page } from '@app/Page'
import { applyKratosErrors } from '@components/auth/kratos-errors'
import { LangSwitch } from '@components/auth/LoginPage'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Button, Field, Input, Stack, Text, useStyles2 } from '@grafana/ui'
import { firstError } from '@helpers/form'
import { register, registrationStatus } from '@lib/auth'
import { revalidateLogic, useForm } from '@tanstack/react-form'
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
  // The registration flow's state for this browser: closed platforms show a
  // notice; a signed-in visitor is taken to the app instead.
  const [closed, setClosed] = useState(false)
  useEffect(() => {
    void registrationStatus().then((s) => {
      if (s === 'signedin') {
        void navigate({ to: '/' })
        return
      }
      setClosed(s === 'closed')
    })
  }, [navigate])

  const schema = useMemo(
    () =>
      z.object({
        email: z.string().trim().email(t('common.validation.email')),
        // Other members see people by this name; Kratos requires it too.
        name: z
          .string()
          .trim()
          .min(1, t('auth.register.nameRequired'))
          .max(80, t('common.validation.max', { max: 80 })),
        password: z.string().min(8, t('auth.register.passwordMin')),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: { email: '', name: '', password: '' },
    validators: { onDynamic: schema },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    onSubmit: async ({ value, formApi }) => {
      try {
        const verification = await register(value.email.trim(), value.name.trim(), value.password)
        qc.clear()
        // Signed in already; the address is confirmed next (the code is mailed).
        if (verification) void navigate({ to: '/verify', search: { flow: verification } })
        else void navigate({ to: next?.startsWith('/') ? next : '/' })
      } catch (e) {
        applyKratosErrors(formApi, e, t, {
          'traits.email': 'email',
          'traits.name': 'name',
          password: 'password',
        })
      }
    },
  })

  return (
    <Page>
      <LangSwitch />
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
                <form.Subscribe selector={(s) => [s.isSubmitting, s.errorMap.onServer] as const}>
                  {([isSubmitting, serverError]) => (
                    <Stack direction="column" gap={1}>
                      <ServerErrorAlert error={serverError} />
                      <Button type="submit" disabled={isSubmitting}>
                        {t('auth.register.submit')}
                      </Button>
                    </Stack>
                  )}
                </form.Subscribe>
              </Stack>
            </form>
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

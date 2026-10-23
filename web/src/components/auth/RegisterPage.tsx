import { AuthCard } from '@components/auth/AuthCard'
import { applyKratosErrors } from '@components/auth/kratos-errors'
import { FlowNotes, OidcButtons, useAuthFlow } from '@components/auth/OidcButtons'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { Alert, Button, Field, Input, Stack, Text } from '@grafana/ui'
import { firstError, REWARD_EARLY } from '@helpers/form'
import { register, registrationStatus } from '@lib/auth'
import { useForm } from '@tanstack/react-form'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

/**
 * Kratos registration: a provider (GitHub) when Kratos offers one, or an
 * e-mail and password. A password account gets a session at once and
 * confirms the e-mail next.
 */
export function RegisterPage({ next, flow: flowId }: { next?: string; flow?: string }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const navigate = useNavigate()
  // The registration flow's state for this browser: closed platforms show a
  // notice; a signed-in visitor is taken to the app instead.
  const [closed, setClosed] = useState(false)
  const flow = useAuthFlow('registration', flowId)
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
    ...REWARD_EARLY,
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
    <AuthCard title={t('auth.register.title')} subtitle={t('auth.register.subtitle')}>
      {closed ? (
        <Alert severity="warning" title={t('auth.register.closedTitle')}>
          {t('auth.register.closedBody')}
        </Alert>
      ) : (
        <>
          <FlowNotes flow={flow} />
          <OidcButtons flow={flow} divider />
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
                    <Button size="lg" type="submit" disabled={isSubmitting}>
                      {t('auth.register.submit')}
                    </Button>
                    <Text variant="bodySmall" color="secondary">
                      {t('auth.register.nextStep')}
                    </Text>
                  </Stack>
                )}
              </form.Subscribe>
            </Stack>
          </form>
        </>
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
    </AuthCard>
  )
}

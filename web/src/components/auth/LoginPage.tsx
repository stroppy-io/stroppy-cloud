import { toast } from '@app/Toaster'
import { AuthCard } from '@components/auth/AuthCard'
import { applyKratosErrors } from '@components/auth/kratos-errors'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { Alert, Button, Field, Input, Stack, Text } from '@grafana/ui'
import { firstError, REWARD_EARLY } from '@helpers/form'
import { login, setToken } from '@lib/auth'
import { useForm } from '@tanstack/react-form'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

/** Kratos login (e-mail + password) or, in dev mode, a static token. */
export function LoginPage({ mode, next }: { mode: 'kratos' | 'dev'; next?: string }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const navigate = useNavigate()

  const done = () => {
    qc.clear() // the cache belongs to the previous session
    void navigate({ to: next?.startsWith('/') ? next : '/' })
  }

  const schema = useMemo(
    () =>
      z.object({
        email: z.string().trim().email(t('common.validation.email')),
        password: z.string().min(1, t('auth.login.passwordRequired')),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: { email: '', password: '' },
    validators: { onDynamic: schema },
    ...REWARD_EARLY,
    onSubmit: async ({ value, formApi }) => {
      try {
        await login(value.email.trim(), value.password)
        done()
      } catch (e) {
        // Kratos flow failures (bad credentials, throttling) are plain text.
        applyKratosErrors(formApi, e, t, { identifier: 'email', password: 'password' })
      }
    },
  })

  if (mode === 'dev') {
    return <DevTokenCard onDone={done} />
  }

  return (
    <AuthCard title={t('auth.login.title')} subtitle={t('auth.login.subtitle')}>
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
                label={t('auth.login.email')}
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
          <form.Field name="password">
            {(field) => (
              <Field
                label={t('auth.login.password')}
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
                  {t('auth.login.submit')}
                </Button>
              </Stack>
            )}
          </form.Subscribe>
          <Stack justifyContent="space-between" alignItems="center">
            <Button
              size="sm"
              variant="secondary"
              fill="text"
              type="button"
              onClick={() => void navigate({ to: '/recovery', search: {} })}
            >
              {t('auth.login.forgot')}
            </Button>
            <Stack alignItems="center" gap={0.5}>
              <Text variant="bodySmall" color="secondary">
                {t('auth.login.noAccount')}
              </Text>
              <Button
                size="sm"
                variant="secondary"
                fill="text"
                type="button"
                onClick={() => void navigate({ to: '/register', search: { next } })}
              >
                {t('auth.login.register')}
              </Button>
            </Stack>
          </Stack>
        </Stack>
      </form>
    </AuthCard>
  )
}

/** Dev mode: a static token of STROPPY_DEV_USERS is the password. */
function DevTokenCard({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation()
  const [token, setTokenState] = useState('')
  return (
    <AuthCard title={t('auth.login.devTitle')}>
      <Alert severity="info" title={t('auth.login.devTitle')}>
        {t('auth.login.devBody')}
      </Alert>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (!token.trim()) return
          setToken(token.trim())
          toast.success(t('auth.login.devSet'))
          onDone()
        }}
      >
        <Stack direction="column" gap={1}>
          <Field label={t('auth.login.devToken')} htmlFor="dev-token" required>
            <Input
              id="dev-token"
              autoFocus
              value={token}
              placeholder="dev"
              onChange={(e) => setTokenState(e.currentTarget.value)}
            />
          </Field>
          <Button type="submit">{t('auth.login.submit')}</Button>
        </Stack>
      </form>
    </AuthCard>
  )
}

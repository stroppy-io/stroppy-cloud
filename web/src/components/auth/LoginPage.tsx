import { Page } from '@app/Page'
import { toast } from '@app/Toaster'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Button, Field, Input, Stack, Text, useStyles2 } from '@grafana/ui'
import { applyServerErrors, firstError } from '@helpers/form'
import { login, setToken } from '@lib/auth'
import { useForm } from '@tanstack/react-form'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
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

/** Kratos login (e-mail + password) or, in dev mode, a static token. */
export function LoginPage({ mode, next }: { mode: 'kratos' | 'dev'; next?: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
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
    onSubmit: async ({ value, formApi }) => {
      try {
        await login(value.email.trim(), value.password)
        done()
      } catch (e) {
        // Kratos flow failures (bad credentials, throttling) are plain text.
        applyServerErrors(formApi, e, e instanceof Error ? e.message : t('common.errors.generic'))
      }
    },
  })

  if (mode === 'dev') {
    return <DevTokenCard onDone={done} />
  }

  return (
    <Page>
      <div className={styles.card}>
        <Stack direction="column" gap={2}>
          <Text variant="h4" weight="medium">
            {t('auth.login.title')}
          </Text>
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
              <Button type="submit" disabled={form.state.isSubmitting}>
                {t('auth.login.submit')}
              </Button>
              <Text variant="bodySmall" color="secondary">
                {t('auth.login.noAccount')}{' '}
                <Button
                  size="sm"
                  variant="secondary"
                  fill="text"
                  type="button"
                  onClick={() => void navigate({ to: '/register', search: { next } })}
                >
                  {t('auth.login.register')}
                </Button>
              </Text>
            </Stack>
          </form>
        </Stack>
      </div>
    </Page>
  )
}

/** Dev mode: a static token of STROPPY_DEV_USERS is the password. */
function DevTokenCard({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const [token, setTokenState] = useState('')
  return (
    <Page>
      <div className={styles.card}>
        <Stack direction="column" gap={2}>
          <Text variant="h4" weight="medium">
            {t('auth.login.devTitle')}
          </Text>
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
        </Stack>
      </div>
    </Page>
  )
}

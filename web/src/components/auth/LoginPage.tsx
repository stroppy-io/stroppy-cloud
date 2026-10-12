import { Page } from '@app/Page'
import { toast } from '@app/Toaster'
import { applyKratosErrors } from '@components/auth/kratos-errors'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Button, Field, Input, Stack, Text, useStyles2 } from '@grafana/ui'
import { firstError } from '@helpers/form'
import { login, setToken } from '@lib/auth'
import { type Lang, setLang } from '@lib/i18n'
import { revalidateLogic, useForm } from '@tanstack/react-form'
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
  lang: css({
    position: 'absolute',
    top: theme.spacing(2),
    right: theme.spacing(2),
    display: 'flex',
    gap: theme.spacing(0.5),
  }),
})

/** RU/EN switch available before the profile (and its preference) exists. */
export function LangSwitch() {
  const { i18n } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <div className={styles.lang}>
      {(['ru', 'en'] as Lang[]).map((l) => (
        <Button
          key={l}
          size="sm"
          variant={i18n.language.startsWith(l) ? 'primary' : 'secondary'}
          fill="text"
          onClick={() => setLang(l)}
        >
          {l === 'ru' ? 'Русский' : 'English'}
        </Button>
      ))}
    </div>
  )
}

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
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
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
    <Page>
      <LangSwitch />
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
              <form.Subscribe selector={(s) => [s.isSubmitting, s.errorMap.onServer] as const}>
                {([isSubmitting, serverError]) => (
                  <Stack direction="column" gap={1}>
                    <ServerErrorAlert error={serverError} />
                    <Button type="submit" disabled={isSubmitting}>
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

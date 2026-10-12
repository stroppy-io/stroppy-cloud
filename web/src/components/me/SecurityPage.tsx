import { toast } from '@app/Toaster'
import { applyKratosErrors, kratosErrorText } from '@components/auth/kratos-errors'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Badge,
  Button,
  Field,
  Input,
  LoadingPlaceholder,
  Stack,
  Text,
  useStyles2,
} from '@grafana/ui'
import { firstError } from '@helpers/form'
import {
  changePassword,
  type Flow,
  FlowError,
  flowMessages,
  getFlow,
  logout,
  tokenClaims,
} from '@lib/auth'
import { revalidateLogic, useForm } from '@tanstack/react-form'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

const getStyles = (theme: GrafanaTheme2) => ({
  section: css({ maxWidth: 480, paddingTop: theme.spacing(1) }),
})

/**
 * Account security (Kratos settings flow): the e-mail and its confirmation,
 * the password. A recovery code lands here with its `flow` — the session is
 * privileged for a few minutes and the new password needs no old one.
 */
export function SecurityPage({ flowId }: { flowId?: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const navigate = useNavigate()
  const [flow, setFlow] = useState<Flow>()
  const [loadError, setLoadError] = useState<string>()
  const [needsReauth, setNeedsReauth] = useState(false)
  const claims = tokenClaims()

  useEffect(() => {
    getFlow('settings', flowId)
      .then(setFlow)
      .catch((e: unknown) => setLoadError(kratosErrorText(t, e)))
  }, [flowId, t])

  const schema = useMemo(
    () =>
      z
        .object({
          password: z.string().min(8, t('auth.register.passwordMin')),
          confirm: z.string(),
        })
        .refine((v) => v.password === v.confirm, {
          path: ['confirm'],
          message: t('me.security.mismatch'),
        }),
    [t]
  )
  const form = useForm({
    defaultValues: { password: '', confirm: '' },
    validators: { onDynamic: schema },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    onSubmit: async ({ value, formApi }) => {
      if (!flow) return
      try {
        await changePassword(flow, value.password)
        formApi.reset()
        toast.success(t('me.security.changed'))
        // A settings flow is single-use: the next change starts a fresh one.
        setFlow(await getFlow('settings'))
      } catch (e) {
        // An old session must sign in again before the password changes.
        if (e instanceof FlowError && e.id === 'session_refresh_required') {
          setNeedsReauth(true)
          return
        }
        applyKratosErrors(formApi, e, t, { password: 'password' })
      }
    },
  })

  const reauth = async () => {
    await logout()
    void navigate({ to: '/login', search: { next: '/me/security' } })
  }

  return (
    <div className={styles.section}>
      <Stack direction="column" gap={3}>
        <Stack direction="column" gap={1}>
          <Text variant="h5">{t('me.security.email')}</Text>
          <Stack alignItems="center" gap={1}>
            <Text>{claims.email ?? '—'}</Text>
            {claims.emailVerified ? (
              <Badge text={t('me.security.verified')} color="green" icon="check" />
            ) : (
              <Badge
                text={t('me.security.unverified')}
                color="orange"
                icon="exclamation-triangle"
              />
            )}
          </Stack>
          {!claims.emailVerified && (
            <div>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => void navigate({ to: '/verify', search: {} })}
              >
                {t('me.security.verify')}
              </Button>
            </div>
          )}
        </Stack>

        <Stack direction="column" gap={1}>
          <Text variant="h5">{t('me.security.password')}</Text>
          {loadError && <Alert severity="error" title={loadError} />}
          {needsReauth && (
            <Alert severity="warning" title={t('me.security.reauthTitle')}>
              <Stack direction="column" gap={1}>
                <span>{t('me.security.reauthBody')}</span>
                <div>
                  <Button size="sm" onClick={() => void reauth()}>
                    {t('me.security.reauth')}
                  </Button>
                </div>
              </Stack>
            </Alert>
          )}
          {!flow && !loadError ? (
            <LoadingPlaceholder text={t('common.misc.loading')} />
          ) : flow ? (
            <form
              onSubmit={(e) => {
                e.preventDefault()
                void form.handleSubmit()
              }}
            >
              <Stack direction="column" gap={1}>
                {flowId && flowMessages(flow)[0] && (
                  <Alert severity="info" title={t('me.security.recovered')} />
                )}
                <form.Field name="password">
                  {(field) => (
                    <Field
                      label={t('me.security.newPassword')}
                      description={t('auth.register.passwordHint')}
                      invalid={!!firstError(field.state.meta)}
                      error={firstError(field.state.meta)}
                      htmlFor={field.name}
                      required
                    >
                      <Input
                        id={field.name}
                        type="password"
                        autoComplete="new-password"
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) => field.handleChange(e.currentTarget.value)}
                      />
                    </Field>
                  )}
                </form.Field>
                <form.Field name="confirm">
                  {(field) => (
                    <Field
                      label={t('me.security.confirm')}
                      invalid={!!firstError(field.state.meta)}
                      error={firstError(field.state.meta)}
                      htmlFor={field.name}
                      required
                    >
                      <Input
                        id={field.name}
                        type="password"
                        autoComplete="new-password"
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
                      <div>
                        <Button type="submit" disabled={isSubmitting}>
                          {t('me.security.change')}
                        </Button>
                      </div>
                    </Stack>
                  )}
                </form.Subscribe>
              </Stack>
            </form>
          ) : null}
        </Stack>
      </Stack>
    </div>
  )
}

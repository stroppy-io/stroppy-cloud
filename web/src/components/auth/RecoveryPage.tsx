import { toast } from '@app/Toaster'
import { AuthCard } from '@components/auth/AuthCard'
import { AuthSteps } from '@components/auth/AuthSteps'
import { CodeInput } from '@components/auth/CodeInput'
import { applyKratosErrors, kratosError, kratosErrorText } from '@components/auth/kratos-errors'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { Alert, Button, Field, Input, LoadingPlaceholder, Stack, Text } from '@grafana/ui'
import { firstError, REWARD_EARLY } from '@helpers/form'
import { useCooldown } from '@hooks/useCooldown'
import { type Flow, getFlow, sendRecoveryCode, submitRecoveryCode } from '@lib/auth'
import { useForm } from '@tanstack/react-form'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

/**
 * Password reset (Kratos recovery flow, code method): the e-mail, then the
 * code from the mail; Kratos signs the account in and hands over to a
 * settings flow, where /me/security takes the new password.
 */
export function RecoveryPage({ flowId }: { flowId?: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [flow, setFlow] = useState<Flow>()
  const [loadError, setLoadError] = useState<string>()
  const [email, setEmail] = useState('')

  useEffect(() => {
    getFlow('recovery', flowId)
      .then(setFlow)
      .catch((e: unknown) => setLoadError(kratosErrorText(t, e)))
  }, [flowId, t])

  const emailSchema = useMemo(
    () => z.object({ email: z.string().trim().email(t('common.validation.email')) }),
    [t]
  )
  const emailForm = useForm({
    defaultValues: { email: '' },
    validators: { onDynamic: emailSchema },
    ...REWARD_EARLY,
    onSubmit: async ({ value, formApi }) => {
      if (!flow) return
      try {
        const next = await sendRecoveryCode(flow, value.email.trim())
        setEmail(value.email.trim())
        if (next) setFlow(next)
        cooldown.start()
      } catch (e) {
        applyKratosErrors(formApi, e, t, { email: 'email' })
      }
    },
  })

  const codeSchema = useMemo(
    () =>
      z.object({
        code: z
          .string()
          .trim()
          .regex(/^\d{6}$/, t('auth.verify.codeFormat')),
      }),
    [t]
  )
  const codeForm = useForm({
    defaultValues: { code: '' },
    validators: { onDynamic: codeSchema },
    ...REWARD_EARLY,
    onSubmit: async ({ value, formApi }) => {
      if (!flow) return
      try {
        const settings = await submitRecoveryCode(flow, value.code.trim())
        void navigate({ to: '/me/security', search: { flow: settings } })
      } catch (e) {
        applyKratosErrors(formApi, e, t, { code: 'code' })
      }
    },
  })

  // A used or expired code needs a new one: a fresh flow mails it to the same address.
  const [resending, setResending] = useState(false)
  const cooldown = useCooldown(60)
  const resend = async () => {
    setResending(true)
    try {
      const fresh = await getFlow('recovery')
      setFlow((await sendRecoveryCode(fresh, email)) ?? fresh)
      codeForm.reset()
      cooldown.start()
      toast.success(t('auth.verify.resent', { email }))
    } catch (e) {
      toast.error(kratosError(t, e))
    } finally {
      setResending(false)
    }
  }

  const back = (
    <Stack alignItems="center" gap={0.5}>
      <Text variant="bodySmall" color="secondary">
        {t('auth.recovery.remembered')}
      </Text>
      <Button
        size="sm"
        variant="secondary"
        fill="text"
        type="button"
        onClick={() => void navigate({ to: '/login' })}
      >
        {t('auth.recovery.backToLogin')}
      </Button>
    </Stack>
  )
  const steps = [
    t('auth.recovery.steps.email'),
    t('auth.recovery.steps.code'),
    t('auth.recovery.steps.password'),
  ]

  if (loadError)
    return (
      <AuthCard title={t('auth.recovery.title')}>
        <Alert severity="error" title={t('auth.recovery.flowFailed')}>
          {loadError}
        </Alert>
        <Button onClick={() => void navigate({ to: '/recovery', search: {} })}>
          {t('auth.verify.restart')}
        </Button>
        {back}
      </AuthCard>
    )
  if (!flow)
    return (
      <AuthCard title={t('auth.recovery.title')}>
        <LoadingPlaceholder text={t('common.misc.loading')} />
      </AuthCard>
    )

  const sent = flow.state === 'sent_email'
  return (
    <AuthCard
      title={t('auth.recovery.title')}
      subtitle={
        sent
          ? t('auth.recovery.sentTo', { email: email || t('auth.verify.yourEmail') })
          : t('auth.recovery.intro')
      }
    >
      <AuthSteps steps={steps} current={sent ? 1 : 0} />
      {!sent ? (
        <form
          onSubmit={(e) => {
            e.preventDefault()
            void emailForm.handleSubmit()
          }}
        >
          <Stack direction="column" gap={2}>
            <emailForm.Field name="email">
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
            </emailForm.Field>
            <emailForm.Subscribe selector={(s) => [s.isSubmitting, s.errorMap.onServer] as const}>
              {([isSubmitting, serverError]) => (
                <Stack direction="column" gap={1}>
                  <ServerErrorAlert error={serverError} />
                  <Button size="lg" type="submit" disabled={isSubmitting}>
                    {t('auth.recovery.send')}
                  </Button>
                </Stack>
              )}
            </emailForm.Subscribe>
          </Stack>
        </form>
      ) : (
        <form
          onSubmit={(e) => {
            e.preventDefault()
            void codeForm.handleSubmit()
          }}
        >
          <Stack direction="column" gap={2}>
            <codeForm.Field name="code">
              {(field) => (
                <Field
                  label={t('auth.verify.code')}
                  invalid={!!firstError(field.state.meta)}
                  error={firstError(field.state.meta)}
                  htmlFor={field.name}
                >
                  <CodeInput
                    id={field.name}
                    autoFocus
                    value={field.state.value}
                    invalid={!!firstError(field.state.meta)}
                    onBlur={field.handleBlur}
                    onChange={(v) => field.handleChange(v)}
                    onComplete={() => void codeForm.handleSubmit()}
                  />
                </Field>
              )}
            </codeForm.Field>
            <codeForm.Subscribe selector={(s) => [s.isSubmitting, s.errorMap.onServer] as const}>
              {([isSubmitting, serverError]) => (
                <Stack direction="column" gap={1}>
                  <ServerErrorAlert error={serverError} />
                  <Button size="lg" type="submit" disabled={isSubmitting}>
                    {t('auth.recovery.submit')}
                  </Button>
                </Stack>
              )}
            </codeForm.Subscribe>
            <Stack alignItems="center" gap={0.5}>
              <Text variant="bodySmall" color="secondary">
                {t('auth.verify.noCode')}
              </Text>
              <Button
                size="sm"
                variant="secondary"
                fill="text"
                type="button"
                disabled={!email || resending || cooldown.left > 0}
                onClick={() => void resend()}
              >
                {cooldown.left > 0
                  ? t('auth.verify.resendIn', { seconds: cooldown.left })
                  : t('auth.verify.resend')}
              </Button>
            </Stack>
          </Stack>
        </form>
      )}
      {back}
    </AuthCard>
  )
}

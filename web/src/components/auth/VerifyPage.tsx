import { toast } from '@app/Toaster'
import { AuthCard } from '@components/auth/AuthCard'
import { CodeInput } from '@components/auth/CodeInput'
import { applyKratosErrors, kratosError, kratosErrorText } from '@components/auth/kratos-errors'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { Alert, Button, Field, LoadingPlaceholder, Stack, Text } from '@grafana/ui'
import { firstError, REWARD_EARLY } from '@helpers/form'
import { useCooldown } from '@hooks/useCooldown'
import {
  type Flow,
  flowMessages,
  getFlow,
  nodeValue,
  sendVerificationCode,
  submitVerificationCode,
  tokenClaims,
} from '@lib/auth'
import { useForm } from '@tanstack/react-form'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

/**
 * E-mail confirmation (Kratos verification flow, code method). Reached from
 * registration (`flow` of continue_with), from the mail link (Kratos
 * redirects here with the code already in the flow — it is submitted on
 * its own) or from the "confirm your e-mail" banner (a new flow).
 */
export function VerifyPage({ flowId }: { flowId?: string }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [flow, setFlow] = useState<Flow>()
  const [loadError, setLoadError] = useState<string>()
  const [done, setDone] = useState(false)
  const autoSubmitted = useRef(false)

  const finish = () => {
    setDone(true)
    qc.clear() // /me now carries the confirmed address
    toast.success(t('auth.verify.done'))
  }

  useEffect(() => {
    getFlow('verification', flowId)
      .then(setFlow)
      .catch((e: unknown) => setLoadError(kratosErrorText(t, e)))
  }, [flowId, t])

  const schema = useMemo(
    () =>
      z.object({
        code: z
          .string()
          .trim()
          .regex(/^\d{6}$/, t('auth.verify.codeFormat')),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: { code: '' },
    validators: { onDynamic: schema },
    ...REWARD_EARLY,
    onSubmit: async ({ value, formApi }) => {
      if (!flow) return
      try {
        await submitVerificationCode(flow, value.code.trim())
        finish()
      } catch (e) {
        applyKratosErrors(formApi, e, t, { code: 'code' })
      }
    },
  })

  // The mail link puts the code into the flow: confirm without typing.
  useEffect(() => {
    if (!flow || autoSubmitted.current || flow.state === 'passed_challenge') return
    const code = nodeValue(flow, 'code')
    if (!code) return
    autoSubmitted.current = true
    form.setFieldValue('code', code)
    void form.handleSubmit()
  }, [flow, form])

  const email = (flow && nodeValue(flow, 'email')) || tokenClaims().email || ''
  const sent = flow?.state === 'sent_email'
  const [resending, setResending] = useState(false)
  // A fresh code can be asked for once a minute.
  const cooldown = useCooldown(60)
  // A code already sent (from registration) starts the wait too.
  // biome-ignore lint/correctness/useExhaustiveDependencies: once, when the flow says it was sent
  useEffect(() => {
    if (sent) cooldown.start()
  }, [sent])
  const resend = async () => {
    if (!flow || !email) return
    setResending(true)
    try {
      const next = await sendVerificationCode(flow, email)
      if (next) setFlow(next)
      cooldown.start()
      toast.success(t('auth.verify.resent', { email }))
    } catch (e) {
      toast.error(kratosError(t, e))
    } finally {
      setResending(false)
    }
  }

  if (loadError)
    return (
      <AuthCard title={t('auth.verify.title')}>
        <Alert severity="error" title={t('auth.verify.flowFailed')}>
          {loadError}
        </Alert>
        <Button onClick={() => void navigate({ to: '/verify', search: {} })}>
          {t('auth.verify.restart')}
        </Button>
      </AuthCard>
    )
  if (!flow)
    return (
      <AuthCard title={t('auth.verify.title')}>
        <LoadingPlaceholder text={t('common.misc.loading')} />
      </AuthCard>
    )

  // Nothing to confirm: the address is already confirmed and no mail link brought a flow here.
  const already = !flowId && tokenClaims().emailVerified === true
  if (done || already || flow.state === 'passed_challenge')
    return (
      <AuthCard title={t('auth.verify.doneTitle')} subtitle={t('auth.verify.doneBody')}>
        <Button size="lg" icon="arrow-right" onClick={() => void navigate({ to: '/' })}>
          {t('auth.verify.continue')}
        </Button>
      </AuthCard>
    )

  return (
    <AuthCard
      title={t('auth.verify.title')}
      subtitle={
        sent
          ? t('auth.verify.sentTo', { email: email || t('auth.verify.yourEmail') })
          : t('auth.verify.intro')
      }
    >
      {!sent ? (
        <Button size="lg" disabled={!email || resending} onClick={() => void resend()}>
          {t('auth.verify.send', { email })}
        </Button>
      ) : (
        <form
          onSubmit={(e) => {
            e.preventDefault()
            void form.handleSubmit()
          }}
        >
          <Stack direction="column" gap={2}>
            <form.Field name="code">
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
                    onComplete={() => void form.handleSubmit()}
                  />
                </Field>
              )}
            </form.Field>
            <form.Subscribe selector={(s) => [s.isSubmitting, s.errorMap.onServer] as const}>
              {([isSubmitting, serverError]) => (
                <Stack direction="column" gap={1}>
                  <ServerErrorAlert error={serverError} />
                  <Button size="lg" type="submit" disabled={isSubmitting}>
                    {t('auth.verify.submit')}
                  </Button>
                </Stack>
              )}
            </form.Subscribe>
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
      {flowMessages(flow).length > 0 && !sent && (
        <Text variant="bodySmall" color="secondary">
          {flowMessages(flow)[0]}
        </Text>
      )}
    </AuthCard>
  )
}

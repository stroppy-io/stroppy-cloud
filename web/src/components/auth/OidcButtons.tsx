import { kratosErrorText, kratosText } from '@components/auth/kratos-errors'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Button, Stack, useStyles2 } from '@grafana/ui'
import { type Flow, type FlowKind, getFlow, oidcProviders, startOidc } from '@lib/auth'
import Github from '@thesvg/react/github'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { brandOf } from '@/styles/brand'

const getStyles = (theme: GrafanaTheme2) => {
  const brand = brandOf(theme)
  return {
    // GitHub's own sign-in button: graphite on a light page, light on a
    // dark one, the mark in the text colour.
    github: css({
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: theme.spacing(1.25),
      width: '100%',
      height: 48,
      padding: theme.spacing(0, 2),
      border: `1px solid ${theme.isDark ? '#d1d9e0' : '#1f2328'}`,
      borderRadius: theme.shape.radius.default,
      background: theme.isDark ? '#f6f8fa' : '#1f2328',
      color: theme.isDark ? '#1f2328' : '#ffffff',
      fontFamily: 'inherit',
      fontSize: theme.typography.h5.fontSize,
      fontWeight: theme.typography.fontWeightMedium,
      cursor: 'pointer',
      transition: 'background 120ms ease',
      '&:hover:not(:disabled)': { background: theme.isDark ? '#e6eaef' : '#32383f' },
      '&:focus-visible': { outline: `2px solid ${brand.signal}`, outlineOffset: 2 },
      '&:disabled': { opacity: 0.6, cursor: 'default' },
    }),
    or: css({
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(1.5),
      color: brand.muted,
      fontSize: theme.typography.bodySmall.fontSize,
      '&::before, &::after': { content: '""', flex: 1, height: 1, background: brand.line },
    }),
  }
}

/**
 * The Kratos flow of this screen: the one Kratos sent the browser back to
 * (`?flow=`: a provider error, a sign-in to link) or a fresh one. Undefined
 * while loading, null when Kratos refuses (a session already exists).
 */
export function useAuthFlow(kind: Extract<FlowKind, 'login' | 'registration'>, id?: string) {
  const [flow, setFlow] = useState<Flow | null>()
  useEffect(() => {
    let live = true
    getFlow(kind, id)
      // An expired or foreign flow id: start over.
      .catch(() => (id ? getFlow(kind) : Promise.reject()))
      .then((f) => live && setFlow(f))
      .catch(() => live && setFlow(null))
    return () => {
      live = false
    }
  }, [kind, id])
  return flow
}

/** The flow's own notes (link the account, a provider failure) above the form. */
export function FlowNotes({ flow }: { flow?: Flow | null }) {
  const { t } = useTranslation()
  const [first, ...rest] = flow?.ui.messages ?? []
  if (!first) return null
  const failed = flow?.ui.messages?.some((m) => m.type === 'error')
  return (
    <Alert severity={failed ? 'error' : 'info'} title={kratosText(t, first)}>
      {rest.map((m) => (
        <div key={`${m.id}-${m.text}`}>{kratosText(t, m)}</div>
      ))}
    </Alert>
  )
}

/** "Continue with GitHub" and the like, then "or" when a form follows. */
export function OidcButtons({ flow, divider }: { flow?: Flow | null; divider?: boolean }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const [busy, setBusy] = useState<string>()
  const [error, setError] = useState<string>()
  const providers = flow ? oidcProviders(flow) : []
  if (!flow || !providers.length) return null
  const go = async (provider: string) => {
    setBusy(provider)
    setError(undefined)
    try {
      await startOidc(flow, provider)
    } catch (e) {
      setError(kratosErrorText(t, e))
      setBusy(undefined)
    }
  }
  return (
    <Stack direction="column" gap={2}>
      {providers.map((p) =>
        p === 'github' ? (
          <button
            key={p}
            type="button"
            className={styles.github}
            disabled={!!busy}
            onClick={() => void go(p)}
          >
            <Github variant="mono" width={20} height={20} aria-hidden />
            {t('auth.oidc.github')}
          </button>
        ) : (
          <Button
            key={p}
            size="lg"
            variant="secondary"
            icon="signin"
            fullWidth
            disabled={!!busy}
            onClick={() => void go(p)}
          >
            {t(`auth.oidc.${p}`, { defaultValue: t('auth.oidc.other', { provider: p }) })}
          </Button>
        )
      )}
      {error && <Alert severity="error" title={error} />}
      {divider && <div className={styles.or}>{t('auth.oidc.or')}</div>}
    </Stack>
  )
}

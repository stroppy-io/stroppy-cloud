import '@xterm/xterm/css/xterm.css'
import { ApiError, errorText } from '@api/errors'
import type { ErrorReason } from '@api/types'
import { getWs, type ShellEvent, type ShellHandle } from '@api/ws'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Button, Drawer, Stack, Text, useStyles2, useTheme2 } from '@grafana/ui'
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  term: css({
    height: 'calc(100vh - 220px)',
    minHeight: 320,
    padding: theme.spacing(1),
    background: theme.colors.background.canvas,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    '.xterm': { height: '100%' },
  }),
})

type State =
  | { kind: 'connecting' }
  | { kind: 'open' }
  | { kind: 'closed'; code: number; message?: string }
  | { kind: 'error'; text: string }

function decode(b64: string): Uint8Array {
  const bin = atob(b64)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out
}

function problemText(p: unknown): string {
  const o = (p ?? {}) as {
    code?: string
    detail?: string
    reason?: ErrorReason
    params?: Record<string, string>
  }
  return errorText(
    new ApiError({ status: 0, code: o.code, detail: o.detail, reason: o.reason, params: o.params })
  )
}

/**
 * A shell on one machine of a run, through Graphene's agent. Lives while the drawer is
 * open; closing the drawer closes the shell.
 */
export function ShellDrawer({
  runId,
  machine,
  onClose,
}: {
  runId: string
  machine: string
  onClose: () => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const theme = useTheme2()
  // The drawer mounts its body after opening: a callback ref reruns the effect then.
  const [el, setEl] = useState<HTMLDivElement | null>(null)
  const [state, setState] = useState<State>({ kind: 'connecting' })
  const [attempt, setAttempt] = useState(0)

  // biome-ignore lint/correctness/useExhaustiveDependencies: `attempt` is the reconnect trigger, not a value used inside
  useEffect(() => {
    if (!el) return
    const c = theme.colors
    const term = new Terminal({
      cursorBlink: true,
      fontFamily: theme.typography.fontFamilyMonospace,
      fontSize: 13,
      scrollback: 5000,
      theme: {
        background: c.background.canvas,
        foreground: c.text.primary,
        cursor: c.text.primary,
        selectionBackground: c.action.selected,
      },
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(el)
    fit.fit()
    let handle: ShellHandle | undefined
    let open = false
    let disposed = false
    setState({ kind: 'connecting' })

    void getWs().then((ws) => {
      if (disposed) return
      handle = ws.shell({
        topic: `agent.pty/${runId}/${machine}`,
        payload: { cols: term.cols, rows: term.rows },
        onEvent: (ev: ShellEvent) => {
          if (ev.session_id) {
            open = true
            setState({ kind: 'open' })
            term.focus()
          }
          if (ev.data) term.write(decode(ev.data))
          if (ev.closed) {
            open = false
            setState({ kind: 'closed', code: ev.closed.exit_code, message: ev.closed.message })
            handle?.close()
          }
        },
        onError: (p) => {
          open = false
          setState({ kind: 'error', text: problemText(p) })
          handle?.close()
        },
      })
    })

    const input = term.onData((data) => {
      if (open) handle?.send({ data })
    })
    let timer: number | undefined
    const resize = new ResizeObserver(() => {
      window.clearTimeout(timer)
      timer = window.setTimeout(() => {
        fit.fit()
        if (open) handle?.send({ cols: term.cols, rows: term.rows })
      }, 100)
    })
    resize.observe(el)

    return () => {
      disposed = true
      window.clearTimeout(timer)
      resize.disconnect()
      input.dispose()
      handle?.close()
      term.dispose()
    }
  }, [el, runId, machine, theme, attempt])

  return (
    <Drawer title={machine} subtitle={t('runs.shell.subtitle')} size="lg" onClose={onClose}>
      <Stack direction="column" gap={1.5}>
        {state.kind === 'error' && (
          <Alert severity="error" title={t('runs.shell.failed')}>
            {state.text}
          </Alert>
        )}
        <Stack alignItems="center" justifyContent="space-between">
          <Text color="secondary" variant="bodySmall">
            {state.kind === 'connecting' && t('runs.shell.connecting')}
            {state.kind === 'open' && t('runs.shell.open')}
            {state.kind === 'closed' &&
              t('runs.shell.closed', { code: state.code, message: state.message ?? '' })}
            {state.kind === 'error' && t('runs.shell.closedError')}
          </Text>
          {(state.kind === 'closed' || state.kind === 'error') && (
            <Button
              size="sm"
              variant="secondary"
              icon="sync"
              onClick={() => setAttempt((n) => n + 1)}
            >
              {t('runs.shell.reconnect')}
            </Button>
          )}
        </Stack>
        <div ref={setEl} className={styles.term} />
      </Stack>
    </Drawer>
  )
}

import { css } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { ClipboardButton, Icon, IconButton, Text, useStyles2, useTheme2 } from '@grafana/ui'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Toaster as Sonner, toast as sonner } from 'sonner'
import { describeError } from './describe-error'

// Toasts: one Grafana-styled card for every notification. Call sites use `toast.success(title)`,
// `toast.info(title)`, `toast.warning(title)` and `toast.error(errorOrTitle)` — pass the caught
// error itself to `error`, it is described from the server's problem+json (code → title,
// detail, validation issues, request id). Never replace an error with a generic string.

type Severity = 'success' | 'error' | 'warning' | 'info'

interface ToastOptions {
  description?: ReactNode
  // Short action in the card («Открыть», «Повторить»).
  action?: { label: string; onClick: () => void }
  duration?: number
}

const ICON: Record<Severity, IconName> = {
  success: 'check-circle',
  error: 'exclamation-circle',
  warning: 'exclamation-triangle',
  info: 'info-circle',
}

const getStyles = (theme: GrafanaTheme2) => ({
  card: css({
    width: 380,
    display: 'grid',
    gridTemplateColumns: 'auto minmax(0, 1fr) auto',
    columnGap: theme.spacing(1.5),
    rowGap: theme.spacing(0.5),
    padding: theme.spacing(1.5, 1, 1.5, 2),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.elevated,
    boxShadow: theme.shadows.z3,
    fontFamily: theme.typography.fontFamily,
    color: theme.colors.text.primary,
  }),
  bar: (s: Severity) =>
    css({
      borderLeft: `3px solid ${
        s === 'success'
          ? theme.colors.success.main
          : s === 'error'
            ? theme.colors.error.main
            : s === 'warning'
              ? theme.colors.warning.main
              : theme.colors.info.main
      }`,
    }),
  icon: (s: Severity) =>
    css({
      marginTop: 2,
      color:
        s === 'success'
          ? theme.colors.success.text
          : s === 'error'
            ? theme.colors.error.text
            : s === 'warning'
              ? theme.colors.warning.text
              : theme.colors.info.text,
    }),
  body: css({ minWidth: 0, display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5) }),
  text: css({ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }),
  issues: css({
    margin: 0,
    paddingLeft: theme.spacing(2),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    overflowWrap: 'anywhere',
  }),
  meta: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(0.5), minWidth: 0 }),
  mono: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.disabled,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
  action: css({
    alignSelf: 'flex-start',
    marginTop: theme.spacing(0.5),
    background: 'none',
    border: 0,
    padding: 0,
    color: theme.colors.text.link,
    cursor: 'pointer',
    font: 'inherit',
    fontSize: theme.typography.bodySmall.fontSize,
    '&:hover': { textDecoration: 'underline' },
  }),
})

function ToastCard({
  id,
  severity,
  title,
  description,
  issues,
  requestId,
  action,
}: {
  id: string | number
  severity: Severity
  title: string
  description?: ReactNode
  issues?: string[]
  requestId?: string
  action?: ToastOptions['action']
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  return (
    <div
      className={`${styles.card} ${styles.bar(severity)}`}
      role={severity === 'error' ? 'alert' : 'status'}
    >
      <Icon name={ICON[severity]} size="lg" className={styles.icon(severity)} />
      <div className={styles.body}>
        <Text weight="medium" element="span">
          <span className={styles.text}>{title}</span>
        </Text>
        {description && (
          <Text variant="bodySmall" color="secondary" element="span">
            <span className={styles.text}>{description}</span>
          </Text>
        )}
        {issues && issues.length > 0 && (
          <ul className={styles.issues}>
            {issues.map((i) => (
              <li key={i}>{i}</li>
            ))}
          </ul>
        )}
        {requestId && (
          <span className={styles.meta}>
            <span className={styles.mono} title={requestId}>
              {t('common.errors.requestId', { id: requestId })}
            </span>
            <ClipboardButton
              size="sm"
              variant="secondary"
              fill="text"
              icon="copy"
              getText={() => requestId}
              tooltip={t('common.actions.copy')}
              aria-label={t('common.actions.copy')}
            />
          </span>
        )}
        {action && (
          <button
            type="button"
            className={styles.action}
            onClick={() => {
              action.onClick()
              sonner.dismiss(id)
            }}
          >
            {action.label}
          </button>
        )}
      </div>
      <IconButton
        name="times"
        size="sm"
        tooltip={t('common.actions.close')}
        onClick={() => sonner.dismiss(id)}
      />
    </div>
  )
}

function show(
  severity: Severity,
  title: string,
  o: ToastOptions & { issues?: string[]; requestId?: string } = {}
) {
  return sonner.custom(
    (id) => (
      <ToastCard
        id={id}
        severity={severity}
        title={title}
        description={o.description}
        issues={o.issues}
        requestId={o.requestId}
        action={o.action}
      />
    ),
    { duration: o.duration ?? (severity === 'error' ? 10_000 : 5_000) }
  )
}

export const toast = {
  success: (title: string, o?: ToastOptions) => show('success', title, o),
  info: (title: string, o?: ToastOptions) => show('info', title, o),
  warning: (title: string, o?: ToastOptions) => show('warning', title, o),
  // `input` is the caught error (preferred) or a plain title. `o.title` names the failed action
  // («Не удалось отозвать ссылку»); the described error then goes under it.
  error: (input: unknown, o: ToastOptions & { title?: string } = {}) => {
    const d = describeError(input)
    const detail = [d.title !== o.title ? (o.title ? d.title : undefined) : undefined, d.detail]
      .filter(Boolean)
      .join(' — ')
    return show('error', o.title ?? d.title, {
      ...o,
      description: o.description ?? (detail || undefined),
      issues: d.issues,
      requestId: d.requestId,
    })
  },
  dismiss: (id?: string | number) => sonner.dismiss(id),
}

// Host: bottom-right stack of `ToastCard`s, themed by the active Grafana theme.
export function Toaster() {
  const theme = useTheme2()
  return (
    <Sonner
      theme={theme.isDark ? 'dark' : 'light'}
      position="bottom-right"
      gap={8}
      visibleToasts={4}
      toastOptions={{ unstyled: true }}
    />
  )
}

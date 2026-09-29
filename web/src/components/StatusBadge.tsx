import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, Tooltip, useStyles2 } from '@grafana/ui'
import { runStatusMeta, type StatusTone } from '@helpers/run-status'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => {
  const tone = (t: StatusTone) => {
    const c =
      t === 'secondary'
        ? {
            border: theme.colors.border.medium,
            text: theme.colors.text.secondary,
            bg: theme.colors.background.secondary,
          }
        : {
            border: theme.colors[t].border,
            text: theme.colors[t].text,
            bg: theme.colors[t].transparent,
          }
    return css({ borderColor: c.border, color: c.text, background: c.bg })
  }
  return {
    badge: css({
      display: 'inline-flex',
      alignItems: 'center',
      gap: 4,
      padding: '1px 8px',
      borderRadius: 999,
      border: '1px solid',
      fontSize: theme.typography.bodySmall.fontSize,
      lineHeight: '18px',
      whiteSpace: 'nowrap',
      fontWeight: theme.typography.fontWeightMedium,
    }),
    iconOnly: css({ display: 'inline-flex', alignItems: 'center' }),
    success: tone('success'),
    error: tone('error'),
    warning: tone('warning'),
    info: tone('info'),
    secondary: tone('secondary'),
    spin: css({
      animation: 'spin 2s linear infinite',
      '@keyframes spin': { to: { transform: 'rotate(360deg)' } },
    }),
    iconColor: (t: StatusTone) =>
      css({ color: t === 'secondary' ? theme.colors.text.secondary : theme.colors[t].text }),
  }
}

// Status pill with icon; `iconOnly` for dense lists (tooltip carries the label).
export function StatusBadge({
  status,
  iconOnly,
  label,
  className,
}: {
  status: string | undefined
  iconOnly?: boolean
  label?: string
  className?: string
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const meta = runStatusMeta(status)
  const text = label ?? t(`common.status.${meta.labelKey}`, { defaultValue: status ?? '—' })
  if (iconOnly) {
    return (
      <Tooltip content={text}>
        <span
          className={cx(styles.iconOnly, styles.iconColor(meta.tone), className)}
          role="img"
          aria-label={text}
        >
          <Icon name={meta.icon} className={meta.spin ? styles.spin : undefined} />
        </span>
      </Tooltip>
    )
  }
  return (
    <span className={cx(styles.badge, styles[meta.tone], className)}>
      <Icon name={meta.icon} size="sm" className={meta.spin ? styles.spin : undefined} />
      {text}
    </span>
  )
}

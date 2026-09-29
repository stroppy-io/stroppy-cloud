import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { IconButton, useStyles2 } from '@grafana/ui'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  wrap: css({ display: 'inline-flex', alignItems: 'center', gap: 4, maxWidth: '100%' }),
  mono: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
})

export function CopyText({
  value,
  display,
  mono = true,
  className,
}: {
  value: string
  display?: string
  mono?: boolean
  className?: string
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const [done, setDone] = useState(false)
  return (
    <span className={cx(styles.wrap, className)}>
      <span className={mono ? styles.mono : undefined} title={value}>
        {display ?? value}
      </span>
      <IconButton
        name={done ? 'check' : 'copy'}
        size="sm"
        tooltip={done ? t('common.actions.copied') : t('common.actions.copy')}
        onClick={() => {
          void navigator.clipboard.writeText(value)
          setDone(true)
          window.setTimeout(() => setDone(false), 1500)
        }}
      />
    </span>
  )
}

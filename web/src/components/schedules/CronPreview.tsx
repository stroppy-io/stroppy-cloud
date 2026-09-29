import { RelativeTime } from '@components/RelativeTime'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, Text, useStyles2 } from '@grafana/ui'
import { describeCron, formatInZone, nextFireTimes, parseCron } from '@helpers/cron'
import { localTimeZone, zoneOffsetLabel } from '@helpers/timezones'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  box: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
    padding: theme.spacing(1.5),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
  }),
  human: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    fontWeight: theme.typography.fontWeightMedium,
  }),
  list: css({
    margin: 0,
    padding: 0,
    listStyle: 'none',
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
  }),
  row: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) auto',
    gap: theme.spacing(2),
    fontSize: theme.typography.bodySmall.fontSize,
    fontFamily: theme.typography.fontFamilyMonospace,
  }),
  rel: css({ color: theme.colors.text.secondary, fontFamily: theme.typography.fontFamily }),
  invalid: css({ color: theme.colors.error.text }),
})

// Human description of a cron + its next N fire times in the chosen zone (and relative to now).
export function CronPreview({
  cron,
  timezone,
  count = 5,
}: {
  cron: string
  timezone: string
  count?: number
}) {
  const { t, i18n } = useTranslation()
  const styles = useStyles2(getStyles)
  const parsed = parseCron(cron)
  const times = useMemo(
    () => (parsed.ok ? nextFireTimes(cron, timezone, count) : []),
    [cron, timezone, count, parsed.ok]
  )
  const human = useMemo(
    () => describeCron(cron, (k, o) => String(t(k, o as never)), i18n.language),
    [cron, t, i18n.language]
  )
  const local = localTimeZone()
  return (
    <div className={styles.box}>
      <div className={styles.human}>
        <Icon
          name={parsed.ok ? 'clock-nine' : 'exclamation-triangle'}
          className={parsed.ok ? undefined : styles.invalid}
        />
        <span className={parsed.ok ? undefined : styles.invalid}>{human}</span>
      </div>
      <Text color="secondary" variant="bodySmall">
        {t('schedules.preview.title')}{' '}
        {t('schedules.preview.inZone', { tz: `${timezone} ${zoneOffsetLabel(timezone)}` })}
        {local !== timezone ? ` · ${t('schedules.preview.local')}: ${local}` : ''}
      </Text>
      {times.length === 0 ? (
        <Text color="secondary" variant="bodySmall">
          {t('schedules.preview.none')}
        </Text>
      ) : (
        <ol className={styles.list}>
          {times.map((d) => (
            <li key={d.toISOString()} className={styles.row}>
              <span title={local !== timezone ? formatInZone(d, local, i18n.language) : undefined}>
                {formatInZone(d, timezone, i18n.language)}
              </span>
              <span className={styles.rel}>
                <RelativeTime value={d.toISOString()} />
              </span>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}

export function CronHuman({ cron }: { cron: string }) {
  const { t, i18n } = useTranslation()
  return <>{describeCron(cron, (k, o) => String(t(k, o as never)), i18n.language)}</>
}

import type { Schemas } from '@api/types'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Icon, Stack, Text, useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'

type Issue = NonNullable<Schemas['Fit']['issues']>[number]

const getStyles = (theme: GrafanaTheme2) => ({
  list: css({ display: 'grid', gap: theme.spacing(1) }),
  item: css({
    display: 'grid',
    gridTemplateColumns: 'auto 1fr auto',
    gap: theme.spacing(1),
    alignItems: 'start',
    padding: theme.spacing(1, 1.5),
    borderRadius: theme.shape.radius.default,
    border: '1px solid',
  }),
  error: css({
    borderColor: theme.colors.error.border,
    background: theme.colors.error.transparent,
    color: theme.colors.error.text,
  }),
  warning: css({
    borderColor: theme.colors.warning.border,
    background: theme.colors.warning.transparent,
    color: theme.colors.warning.text,
  }),
  path: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
  }),
  code: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  msg: css({ color: theme.colors.text.primary }),
  ok: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    color: theme.colors.success.text,
    padding: theme.spacing(1.5),
    border: `1px solid ${theme.colors.success.border}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.success.transparent,
  }),
  group: css({
    margin: theme.spacing(1.5, 0, 0.5),
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: 0.4,
  }),
})

// Fit issues grouped by severity. `onGoTo(path)` lets the caller focus the matching form section.
export function ValidationIssues({
  issues,
  onGoTo,
  onApplySuggested,
  compact,
}: {
  issues: Issue[] | undefined
  onGoTo?: (path: string) => void
  onApplySuggested?: (issue: Issue) => void
  compact?: boolean
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const list = issues ?? []
  if (!list.length)
    return (
      <div className={styles.ok}>
        <Icon name="check-circle" />
        <Text>{t('library.validation.allGood')}</Text>
      </div>
    )
  const groups: ('ERROR' | 'WARNING')[] = ['ERROR', 'WARNING']
  return (
    <div>
      {groups.map((sev) => {
        const items = list.filter((i) => i.severity === sev)
        if (!items.length) return null
        return (
          <div key={sev}>
            {!compact && (
              <div className={styles.group}>
                {t(`library.validation.severity.${sev}`)} · {items.length}
              </div>
            )}
            <div className={styles.list}>
              {items.map((i, idx) => (
                <div
                  key={`${i.path}-${i.code}-${idx}`}
                  className={`${styles.item} ${sev === 'ERROR' ? styles.error : styles.warning}`}
                >
                  <Icon name={sev === 'ERROR' ? 'exclamation-circle' : 'exclamation-triangle'} />
                  <div>
                    <div className={styles.msg}>{i.message ?? i.code}</div>
                    <Stack gap={1} alignItems="center" wrap="wrap">
                      <span className={styles.code}>{i.code}</span>
                      {i.path && (
                        <span className={styles.path}>
                          {t('library.validation.at')} {i.path}
                        </span>
                      )}
                      {i.suggested !== undefined && i.suggested !== null && (
                        <Text color="secondary" variant="bodySmall">
                          {t('library.validation.suggested', { value: String(i.suggested) })}
                        </Text>
                      )}
                    </Stack>
                  </div>
                  <Stack gap={0.5}>
                    {onApplySuggested && i.suggested !== undefined && i.suggested !== null && (
                      <Button size="sm" variant="secondary" onClick={() => onApplySuggested(i)}>
                        {t('library.validation.apply')}
                      </Button>
                    )}
                    {onGoTo && i.path && (
                      <Button
                        size="sm"
                        variant="secondary"
                        fill="text"
                        icon="arrow-right"
                        onClick={() => onGoTo(i.path)}
                      >
                        {t('library.validation.goTo')}
                      </Button>
                    )}
                  </Stack>
                </div>
              ))}
            </div>
          </div>
        )
      })}
    </div>
  )
}

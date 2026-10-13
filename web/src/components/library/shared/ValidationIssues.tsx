import type { Schemas } from '@api/types'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Icon, Text, useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'
import { suggestedRoleSize, suggestedText } from './suggested'

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
  body: css({ minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }),
  actions: css({ display: 'flex', gap: theme.spacing(0.5), flexShrink: 0 }),
  all: css({ display: 'flex', justifyContent: 'flex-end', marginBottom: theme.spacing(1) }),
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

// What the reader is shown of an issue: a sentence in their language when
// the issue is one we know (a role without a size), the server's otherwise.
function issueMessage(i: Issue, t: (k: string, o?: Record<string, unknown>) => string) {
  const role = i.path.match(/^sizes\.([^.]+)$/)?.[1]
  if (role && i.code === 'required') return t('library.validation.issue.sizeRequired', { role })
  return i.message ?? i.code
}

/**
 * The server repeats the first failure as a `preflight` issue of the compiled
 * spec; next to the issue it repeats, it is noise.
 */
export function visibleIssues(issues: Issue[] | undefined): Issue[] {
  const list = issues ?? []
  const others = list.some((i) => i.severity === 'ERROR' && i.code !== 'preflight')
  return others ? list.filter((i) => i.code !== 'preflight') : list
}

/** Roles the server suggests a size for (a role left without one). */
export function suggestedSizes(issues: Issue[] | undefined) {
  const out: Record<string, NonNullable<ReturnType<typeof suggestedRoleSize>>> = {}
  for (const i of issues ?? []) {
    const role = i.path.match(/^sizes\.([^.]+)$/)?.[1]
    const sug = suggestedRoleSize(i.suggested)
    if (role && i.code === 'required' && sug) out[role] = sug
  }
  return out
}

// Fit issues grouped by severity, in plain words. `onApplySuggested` applies
// one suggestion, `onApplyAll` every size suggestion at once, `onGoTo(path)`
// opens the form section where the issue is fixed by hand.
export function ValidationIssues({
  issues,
  onGoTo,
  onApplySuggested,
  onApplyAll,
  compact,
}: {
  issues: Issue[] | undefined
  onGoTo?: (path: string) => void
  onApplySuggested?: (issue: Issue) => void
  onApplyAll?: () => void
  compact?: boolean
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const list = visibleIssues(issues)
  if (!list.length)
    return (
      <div className={styles.ok}>
        <Icon name="check-circle" />
        <Text>{t('library.validation.allGood')}</Text>
      </div>
    )
  const sized = Object.keys(suggestedSizes(list)).length
  const groups: ('ERROR' | 'WARNING')[] = ['ERROR', 'WARNING']
  return (
    <div>
      {onApplyAll && sized > 1 && (
        <div className={styles.all}>
          <Button size="sm" icon="bolt" onClick={onApplyAll}>
            {t('library.validation.applyAll', { count: sized })}
          </Button>
        </div>
      )}
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
              {items.map((i, idx) => {
                const hasSuggestion = i.suggested !== undefined && i.suggested !== null
                // Blocking but with a ready fix (a size to apply) reads as «to do», not «broken».
                const red = sev === 'ERROR' && !hasSuggestion
                return (
                  <div
                    key={`${i.path}-${i.code}-${idx}`}
                    className={`${styles.item} ${red ? styles.error : styles.warning}`}
                  >
                    <Icon name={red ? 'exclamation-circle' : 'exclamation-triangle'} />
                    <div className={styles.body}>
                      <div className={styles.msg}>{issueMessage(i, t)}</div>
                      {hasSuggestion && (
                        <Text color="secondary" variant="bodySmall">
                          {t('library.validation.suggested', { value: suggestedText(i.suggested) })}
                        </Text>
                      )}
                    </div>
                    <div className={styles.actions}>
                      {onApplySuggested && hasSuggestion ? (
                        <Button size="sm" variant="secondary" onClick={() => onApplySuggested(i)}>
                          {t('library.validation.applyValue', {
                            value: suggestedText(i.suggested),
                          })}
                        </Button>
                      ) : (
                        onGoTo &&
                        i.path && (
                          <Button
                            size="sm"
                            variant="secondary"
                            fill="text"
                            icon="arrow-right"
                            onClick={() => onGoTo(i.path)}
                          >
                            {t('library.validation.goTo')}
                          </Button>
                        )
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        )
      })}
    </div>
  )
}

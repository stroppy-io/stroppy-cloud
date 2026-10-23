import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, useStyles2 } from '@grafana/ui'
import { brandOf } from '@/styles/brand'

// Where the visitor is in a multi-step auth flow (recovery: e-mail → code → password).

const getStyles = (theme: GrafanaTheme2) => {
  const brand = brandOf(theme)
  return {
    list: css({
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(1),
      margin: 0,
      padding: 0,
      listStyle: 'none',
      fontSize: theme.typography.bodySmall.fontSize,
      color: brand.muted,
      flexWrap: 'wrap',
    }),
    item: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.75) }),
    num: css({
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      width: 20,
      height: 20,
      borderRadius: '50%',
      border: `1px solid ${brand.line}`,
      fontFamily: theme.typography.fontFamilyMonospace,
    }),
    current: css({
      color: brand.text,
      '& > span:first-child': {
        borderColor: theme.colors.primary.main,
        color: theme.colors.primary.text,
      },
    }),
    done: css({
      color: brand.muted,
      '& > span:first-child': {
        borderColor: theme.colors.success.border,
        color: theme.colors.success.text,
      },
    }),
    sep: css({ width: 16, height: 1, background: brand.line }),
  }
}

export function AuthSteps({ steps, current }: { steps: string[]; current: number }) {
  const styles = useStyles2(getStyles)
  return (
    <ol className={styles.list}>
      {steps.map((label, i) => (
        <li key={label} className={styles.item}>
          {i > 0 && <span className={styles.sep} aria-hidden />}
          <span
            className={cx(styles.item, i === current && styles.current, i < current && styles.done)}
            aria-current={i === current ? 'step' : undefined}
          >
            <span className={styles.num}>
              {i < current ? <Icon name="check" size="xs" /> : i + 1}
            </span>
            <span>{label}</span>
          </span>
        </li>
      ))}
    </ol>
  )
}

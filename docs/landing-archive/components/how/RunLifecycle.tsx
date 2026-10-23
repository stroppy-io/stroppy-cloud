import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'

// The phases every run goes through, in order, with what you see in each —
// the bridge between the product model above and the internals screen.

const PHASES = ['provisioning', 'deploying', 'workload', 'collecting', 'teardown'] as const

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ marginTop: theme.spacing(4) }),
  title: css({
    margin: theme.spacing(0, 0, 1.5),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightRegular,
  }),
  list: css({
    display: 'grid',
    gridTemplateColumns: `repeat(${PHASES.length}, minmax(0, 1fr))`,
    gap: theme.spacing(2),
    margin: 0,
    padding: 0,
    listStyle: 'none',
    counterReset: 'phase',
    [theme.breakpoints.down('md')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  phase: css({
    position: 'relative',
    paddingTop: theme.spacing(2),
    borderTop: `2px solid ${theme.colors.border.medium}`,
    counterIncrement: 'phase',
    '&::before': {
      content: 'counter(phase)',
      position: 'absolute',
      top: theme.spacing(-1.5),
      left: 0,
      width: theme.spacing(2.5),
      height: theme.spacing(2.5),
      display: 'grid',
      placeItems: 'center',
      borderRadius: theme.shape.radius.circle,
      background: theme.colors.background.primary,
      border: `1px solid ${theme.colors.primary.border}`,
      color: theme.colors.primary.text,
      fontFamily: theme.typography.fontFamilyMonospace,
      fontSize: theme.typography.bodySmall.fontSize,
    },
  }),
  name: css({ fontWeight: theme.typography.fontWeightMedium }),
  body: css({
    margin: theme.spacing(0.5, 0, 0),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    lineHeight: 1.5,
  }),
})

export function RunLifecycle() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <section className={styles.root} aria-labelledby="run-lifecycle">
      <h3 id="run-lifecycle" className={styles.title}>
        {t('how.lifecycle.title')}
      </h3>
      <ol className={styles.list}>
        {PHASES.map((p) => (
          <li key={p} className={styles.phase}>
            <div className={styles.name}>{t(`how.lifecycle.${p}.title`)}</div>
            <p className={styles.body}>{t(`how.lifecycle.${p}.body`)}</p>
          </li>
        ))}
      </ol>
    </section>
  )
}

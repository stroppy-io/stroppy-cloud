import { css } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Icon, Text, useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  flow: css({
    display: 'grid',
    gridTemplateColumns:
      'minmax(0, 1fr) auto minmax(0, 1fr) auto minmax(0, 1fr) auto minmax(0, 1fr)',
    alignItems: 'stretch',
    gap: theme.spacing(1),
    [theme.breakpoints.down('md')]: {
      gridTemplateColumns: '1fr',
      '& > [data-arrow]': { transform: 'rotate(90deg)' },
    },
  }),
  inputs: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  box: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    padding: theme.spacing(1.5),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
    minWidth: 0,
  }),
  head: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1) }),
  arrow: css({ alignSelf: 'center', color: theme.colors.text.secondary }),
  extras: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
    gap: theme.spacing(1),
  }),
})

function Box({ icon, title, body }: { icon: IconName; title: string; body: string }) {
  const styles = useStyles2(getStyles)
  return (
    <div className={styles.box}>
      <div className={styles.head}>
        <Icon name={icon} />
        <Text weight="medium">{title}</Text>
      </div>
      <Text variant="bodySmall" color="secondary">
        {body}
      </Text>
    </div>
  )
}

/**
 * The product model on one screen: a database and a workload make a test, a
 * test launches runs, runs give results; suites, schedules and providers
 * around them.
 */
export function ConceptMap() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const k = (id: string) => ({
    title: t(`onboarding.concepts.${id}.title`),
    body: t(`onboarding.concepts.${id}.body`),
  })
  const arrow = <Icon name="arrow-right" size="lg" className={styles.arrow} data-arrow />
  return (
    <div className={styles.root}>
      <div className={styles.flow}>
        <div className={styles.inputs}>
          <Box icon="database" {...k('database')} />
          <Box icon="bolt" {...k('workload')} />
        </div>
        {arrow}
        <Box icon="vial" {...k('test')} />
        {arrow}
        <Box icon="play" {...k('run')} />
        {arrow}
        <Box icon="chart-line" {...k('results')} />
      </div>
      <div className={styles.extras}>
        <Box icon="layer-group" {...k('suite')} />
        <Box icon="clock-nine" {...k('schedule')} />
        <Box icon="cloud" {...k('provider')} />
        <Box icon="building" {...k('tenant')} />
      </div>
    </div>
  )
}

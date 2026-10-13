import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'
import { Diagram, type DiagramEdge, type DiagramNode } from './Diagram'

// The product model: what a test is made of, what a run needs and leaves;
// suites and schedules launch runs too; all of it lives in a tenant.

const NODES: Pick<DiagramNode, 'id' | 'icon' | 'area'>[] = [
  { id: 'database', icon: 'database', area: 'db' },
  { id: 'workload', icon: 'bolt', area: 'wl' },
  { id: 'test', icon: 'vial', area: 'test' },
  { id: 'suite', icon: 'layer-group', area: 'suite' },
  { id: 'schedule', icon: 'clock-nine', area: 'sched' },
  { id: 'provider', icon: 'cloud', area: 'prov' },
  { id: 'run', icon: 'play', area: 'run' },
  { id: 'results', icon: 'chart-line', area: 'res' },
]

const EDGES: DiagramEdge[] = [
  { from: 'database', to: 'test' },
  { from: 'workload', to: 'test' },
  { from: 'test', to: 'run' },
  { from: 'test', to: 'suite' },
  { from: 'suite', to: 'run' },
  { from: 'schedule', to: 'run' },
  { from: 'provider', to: 'run' },
  { from: 'run', to: 'results' },
]

const RESULT_PARTS = ['report', 'compare', 'rating', 'share'] as const

const getStyles = (theme: GrafanaTheme2) => ({
  hint: css({
    margin: theme.spacing(0, 0, 1.5),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  chips: css({
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(0.5),
    marginTop: theme.spacing(0.5),
  }),
  chip: css({
    padding: theme.spacing(0, 0.75),
    borderRadius: theme.shape.radius.pill,
    border: `1px solid ${theme.colors.border.weak}`,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
})

export function ConceptDiagram() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const k = (id: string) => `onboarding.concepts.${id}`
  const nodes: DiagramNode[] = NODES.map((n) => ({
    ...n,
    title: t(`${k(n.id)}.title`),
    body: t(`${k(n.id)}.body`),
    more: t(`${k(n.id)}.more`),
    foot: t(`${k(n.id)}.where`),
    extra:
      n.id === 'results' ? (
        <span className={styles.chips}>
          {RESULT_PARTS.map((p) => (
            <span key={p} className={styles.chip}>
              {t(`${k('results')}.parts.${p}`)}
            </span>
          ))}
        </span>
      ) : undefined,
  }))
  return (
    <div>
      <p className={styles.hint}>{t('onboarding.concepts.hint')}</p>
      <Diagram
        columns="minmax(0, 1fr)"
        groups={[
          {
            id: 'tenant',
            label: `${t(`${k('tenant')}.title`)} — ${t(`${k('tenant')}.body`)}`,
            icon: 'building',
            dashed: true,
            columns: 4,
            areas: `"db . suite ." "wl test run res" ". sched prov ."`,
            nodes,
          },
        ]}
        edges={EDGES}
      />
    </div>
  )
}

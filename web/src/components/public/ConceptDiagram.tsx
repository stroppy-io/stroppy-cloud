import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'
import {
  Diagram,
  type DiagramEdge,
  type DiagramFact,
  type DiagramFocus,
  type DiagramNode,
} from './Diagram'

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

/** What people come to do, each a path through the model (0 = all of it). */
export const PRODUCT_STEPS = ['all', 'first', 'compare', 'regress', 'share'] as const

const FOCUS: Record<(typeof PRODUCT_STEPS)[number], DiagramFocus | null> = {
  all: null,
  first: {
    nodes: ['database', 'workload', 'test', 'provider', 'run', 'results'],
    edges: ['database-test', 'workload-test', 'test-run', 'provider-run', 'run-results'],
  },
  compare: {
    nodes: ['test', 'suite', 'run', 'results'],
    edges: ['test-suite', 'suite-run', 'run-results'],
  },
  regress: { nodes: ['schedule', 'run', 'results'], edges: ['schedule-run', 'run-results'] },
  share: { nodes: ['run', 'results'], edges: ['run-results'] },
}

const lowerFirst = (s: string) => s.charAt(0).toLowerCase() + s.slice(1)

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

export function ConceptDiagram({ step = 0, hint = true }: { step?: number; hint?: boolean }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const k = (id: string) => `onboarding.concepts.${id}`
  const nodes: DiagramNode[] = NODES.map((n) => ({
    ...n,
    title: t(`${k(n.id)}.title`),
    body: t(`${k(n.id)}.body`),
    more: t(`${k(n.id)}.more`),
    foot: t(`${k(n.id)}.where`),
    sample: t(`${k(n.id)}.sample`),
    facts: t(`${k(n.id)}.facts`, { returnObjects: true }) as DiagramFact[],
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
      {hint && <p className={styles.hint}>{t('onboarding.concepts.hint')}</p>}
      <Diagram
        columns="minmax(0, 1fr)"
        groups={[
          {
            id: 'tenant',
            label: `${t(`${k('tenant')}.title`)}: ${lowerFirst(t(`${k('tenant')}.body`))}`,
            icon: 'building',
            dashed: true,
            columns: 4,
            areas: `"db . suite ." "wl test run res" ". sched prov ."`,
            nodes,
          },
        ]}
        edges={EDGES}
        focus={FOCUS[PRODUCT_STEPS[step] ?? 'all']}
      />
    </div>
  )
}

import { runQueries } from '@api/queries/runs'
import type { RunOverview } from '@api/types'
import { ErrorState } from '@app/ErrorState'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  EmptyState,
  IconButton,
  LoadingPlaceholder,
  Spinner,
  Text,
  useStyles2,
  useTheme2,
} from '@grafana/ui'
import { isTerminal } from '@helpers/run-status'
import { useRun } from '@hooks/useRun'
import { useTenant } from '@hooks/useTenant'
import { useTopic } from '@hooks/useTopic'
import { useQuery } from '@tanstack/react-query'
import {
  Background,
  BackgroundVariant,
  type NodeMouseHandler,
  Panel,
  ReactFlow,
  ReactFlowProvider,
  useNodesInitialized,
  useReactFlow,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  buildTopologyGraph,
  type EdgeKind,
  type RoleClass,
  type TopoColors,
  type TopoGraph,
  type TopoNode,
} from './build-graph'
import { TopologyDrawer, type TopologySelection } from './TopologyDrawer'
import { topologyNodeTypes } from './TopologyNodes'

const FIT = { padding: 0.15, duration: 300 }

const getStyles = (theme: GrafanaTheme2) => ({
  // Overview embed: fixed height instead of filling.
  mini: css({ flex: 'none', minHeight: 0 }),
  // Fills the run page's tab body (web/AGENTS.md §18).
  frame: css({
    flex: '1 1 auto',
    minHeight: 420,
    position: 'relative',
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.canvas,
    overflow: 'hidden',
    // xyflow draws its own focus ring / attribution chrome; keep ours.
    '& .react-flow__edge-textbg': { rx: 3 },
    '& .react-flow__node': { cursor: 'pointer' },
  }),
  controls: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    padding: theme.spacing(0.5),
    background: theme.colors.background.primary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    boxShadow: theme.shadows.z1,
  }),
  legend: css({
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: theme.spacing(0.5, 1.5),
    padding: theme.spacing(0.75, 1.25),
    background: theme.colors.background.primary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    maxWidth: '70%',
  }),
  legendItem: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.5) }),
  legendGroup: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    '&:not(:last-child)': {
      paddingRight: theme.spacing(1.5),
      borderRight: `1px solid ${theme.colors.border.weak}`,
    },
  }),
  dot: css({
    width: theme.spacing(1),
    height: theme.spacing(1),
    borderRadius: theme.shape.radius.circle,
    display: 'inline-block',
  }),
  line: css({
    width: theme.spacing(2.5),
    height: 0,
    borderTop: '2px solid',
    display: 'inline-block',
  }),
  lineDashed: css({ borderTopStyle: 'dashed' }),
  center: css({
    height: '100%',
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    gap: theme.spacing(1),
    color: theme.colors.text.secondary,
  }),
})

function useTopoColors(): TopoColors {
  const theme = useTheme2()
  return useMemo(() => {
    const byName = (n: string) => theme.visualization.getColorByName(n)
    const roleName: Record<RoleClass, string | undefined> = {
      runner: 'green',
      proxy: 'yellow',
      db: 'blue',
      replica: 'light-blue',
      coordinator: 'purple',
      monitor: 'orange',
      other: undefined,
    }
    return {
      role: (cls) =>
        roleName[cls] ? byName(roleName[cls] as string) : theme.colors.text.secondary,
      status: (s) => {
        switch (s) {
          case 'ready':
          case 'completed':
            return theme.colors.success.main
          case 'failed':
            return theme.colors.error.main
          case 'creating':
          case 'deleting':
          case 'running':
            return theme.colors.warning.main
          case 'pending':
          case 'deleted':
            return theme.colors.text.disabled
          default:
            return theme.colors.text.secondary
        }
      },
      presence: (p) => {
        switch (p) {
          case 'online':
            return theme.colors.success.main
          case 'stale':
            return theme.colors.warning.main
          case 'offline':
          case 'terminated':
            return theme.colors.error.main
          default:
            return theme.colors.text.disabled
        }
      },
      edge: (k) => {
        switch (k) {
          case 'flow':
            return theme.colors.info.main
          case 'replication':
            return byName('purple')
          case 'proxy':
            return byName('yellow')
          case 'coordination':
            return theme.colors.text.secondary
          case 'control':
            return theme.colors.text.disabled
        }
      },
      labelText: theme.colors.text.secondary,
      labelBg: theme.colors.background.primary,
      fontFamily: theme.typography.fontFamilyMonospace,
      fontSize: Number.parseFloat(theme.typography.bodySmall.fontSize) || 12,
    }
  }, [theme])
}

// Overview (live until terminal) + run + resource tree (best effort) for one run.
function useTopologyData(id: string) {
  const { slug } = useTenant()
  const run = useRun(slug, id).data
  const terminal = isTerminal(run?.status)
  const overview = useQuery({
    ...runQueries.overview(slug, id),
    refetchInterval: terminal ? false : 10_000,
  })
  useTopic<RunOverview, RunOverview>({
    topic: `run.overview/${id}`,
    queryKey: runQueries.overview(slug, id).queryKey,
    enabled: !!run && !terminal,
    merge: (_prev, payload) => payload,
  })
  const tree = useQuery({
    ...runQueries.tree(slug, id),
    retry: false,
    refetchInterval: terminal ? false : 30_000,
  })
  return { slug, run, terminal, overview, tree: tree.data }
}

function Legend({ graph, colors }: { graph: TopoGraph; colors: TopoColors }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  if (!graph.nodes.length) return null
  return (
    <div className={styles.legend}>
      <span className={styles.legendGroup}>
        {graph.roleClasses.map((cls) => (
          <span key={cls} className={styles.legendItem}>
            <span className={styles.dot} style={{ background: colors.role(cls) }} />
            {t(`runs.topology.roles.${cls}`)}
          </span>
        ))}
      </span>
      <span className={styles.legendGroup}>
        {graph.statuses.map((s) => (
          <span key={s} className={styles.legendItem}>
            <span className={styles.dot} style={{ background: colors.status(s) }} />
            {t(`common.status.${s}`, { defaultValue: s })}
          </span>
        ))}
      </span>
      {graph.edgeKinds.length > 0 && (
        <span className={styles.legendGroup}>
          {graph.edgeKinds.map((k: EdgeKind) => (
            <span key={k} className={styles.legendItem}>
              <span
                className={cx(
                  styles.line,
                  (k === 'coordination' || k === 'control') && styles.lineDashed
                )}
                style={{ borderTopColor: colors.edge(k) }}
              />
              {t(`runs.topology.edges.${k}`)}
            </span>
          ))}
        </span>
      )}
    </div>
  )
}

function Canvas({
  graph,
  colors,
  interactive,
  selectedId,
  onSelect,
}: {
  graph: TopoGraph
  colors: TopoColors
  interactive: boolean
  selectedId?: string
  onSelect?: (node: TopoNode) => void
}) {
  const styles = useStyles2(getStyles)
  const theme = useTheme2()
  const { t } = useTranslation()
  const rf = useReactFlow()
  const initialized = useNodesInitialized()
  const nodes = useMemo(
    () => graph.nodes.map((n) => (n.id === selectedId ? { ...n, selected: true } : n)),
    [graph.nodes, selectedId]
  )
  // Re-fit when the set of nodes changes (machines appear while provisioning).
  const signature = graph.nodes.map((n) => n.id).join('|')
  // biome-ignore lint/correctness/useExhaustiveDependencies: `signature` is the trigger
  useEffect(() => {
    if (initialized) void rf.fitView(FIT)
  }, [initialized, signature, rf])
  const handleNodeClick: NodeMouseHandler<TopoNode> = (_e, node) => onSelect?.(node)
  return (
    <ReactFlow<TopoNode>
      nodes={nodes}
      edges={graph.edges}
      nodeTypes={topologyNodeTypes}
      fitView
      fitViewOptions={FIT}
      nodesDraggable={false}
      nodesConnectable={false}
      elementsSelectable={false}
      nodesFocusable={interactive}
      edgesFocusable={false}
      panOnDrag={interactive}
      zoomOnScroll={interactive}
      zoomOnPinch={interactive}
      zoomOnDoubleClick={false}
      preventScrolling={interactive}
      minZoom={0.2}
      maxZoom={2}
      proOptions={{ hideAttribution: true }}
      colorMode={theme.isDark ? 'dark' : 'light'}
      onNodeClick={interactive ? handleNodeClick : undefined}
    >
      <Background
        variant={BackgroundVariant.Dots}
        gap={20}
        size={1}
        color={theme.colors.border.weak}
      />
      {interactive && (
        <>
          <Panel position="top-right">
            <div className={styles.controls}>
              <IconButton
                name="search-plus"
                tooltip={t('runs.topology.controls.zoomIn')}
                onClick={() => void rf.zoomIn({ duration: 200 })}
              />
              <IconButton
                name="search-minus"
                tooltip={t('runs.topology.controls.zoomOut')}
                onClick={() => void rf.zoomOut({ duration: 200 })}
              />
              <IconButton
                name="home-alt"
                tooltip={t('runs.topology.controls.fit')}
                onClick={() => void rf.fitView(FIT)}
              />
            </div>
          </Panel>
          <Panel position="bottom-left">
            <Legend graph={graph} colors={colors} />
          </Panel>
        </>
      )}
    </ReactFlow>
  )
}

function Empty({ running }: { running: boolean }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  if (running)
    return (
      <div className={styles.center}>
        <Spinner size="lg" />
        <Text color="secondary">{t('runs.topology.provisioning')}</Text>
      </div>
    )
  return (
    <div className={styles.center}>
      <EmptyState variant="not-found" message={t('runs.topology.empty')} hideImage>
        {t('runs.topology.emptyHint')}
      </EmptyState>
    </div>
  )
}

// Full tab: interactive canvas, controls, legend, node drawer; fills the viewport.
export function TopologyView({ id }: { id: string }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const colors = useTopoColors()
  const { slug, run, terminal, overview, tree } = useTopologyData(id)
  const [selection, setSelection] = useState<TopologySelection | null>(null)
  const graph = useMemo(
    () =>
      overview.data
        ? buildTopologyGraph(
            { overview: overview.data, snapshot: run?.snapshot, tree, running: !terminal },
            colors
          )
        : undefined,
    [overview.data, run?.snapshot, tree, terminal, colors]
  )
  const selectedId =
    selection?.kind === 'machine'
      ? `m:${selection.name}`
      : selection
        ? `c:${selection.id}`
        : undefined

  if (overview.isPending) return <LoadingPlaceholder text={t('common.misc.loading')} />
  if (overview.isError)
    return <ErrorState error={overview.error} onRetry={() => void overview.refetch()} compact />

  return (
    <div className={styles.frame}>
      {graph?.nodes.length ? (
        <ReactFlowProvider>
          <Canvas
            graph={graph}
            colors={colors}
            interactive
            selectedId={selectedId}
            onSelect={(n) => {
              if (n.data.kind === 'machine') setSelection({ kind: 'machine', name: n.data.name })
              else if (n.data.kind === 'component')
                setSelection({ kind: 'component', id: n.data.id })
            }}
          />
        </ReactFlowProvider>
      ) : (
        <Empty running={!terminal} />
      )}
      {selection && overview.data && run && (
        <TopologyDrawer
          selection={selection}
          overview={overview.data}
          run={run}
          slug={slug}
          onClose={() => setSelection(null)}
        />
      )}
    </div>
  )
}

// Compact read-only preview (Overview tab embed): same builder, fit to box, no controls.
export function TopologyMini({ id, height = 260 }: { id: string; height?: number }) {
  const styles = useStyles2(getStyles)
  const colors = useTopoColors()
  const { run, terminal, overview, tree } = useTopologyData(id)
  const graph = useMemo(
    () =>
      overview.data
        ? buildTopologyGraph(
            { overview: overview.data, snapshot: run?.snapshot, tree, running: !terminal },
            colors
          )
        : undefined,
    [overview.data, run?.snapshot, tree, terminal, colors]
  )
  return (
    <div className={cx(styles.frame, styles.mini)} style={{ height }}>
      {graph?.nodes.length ? (
        <ReactFlowProvider>
          <Canvas graph={graph} colors={colors} interactive={false} />
        </ReactFlowProvider>
      ) : (
        <Empty running={!terminal && !overview.isPending} />
      )}
    </div>
  )
}

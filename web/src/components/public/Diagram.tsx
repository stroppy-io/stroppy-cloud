import { css, cx } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Icon, useStyles2, useTheme2 } from '@grafana/ui'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { type ReactNode, useId, useLayoutEffect, useRef, useState } from 'react'

// A diagram of blocks in framed groups with drawn links between them. Hover
// (or focus) lights a block with its links and opens a card next to it — what
// it is and how it works. The product model and the architecture are both
// drawn with it; each says what it shows by data.

export type DiagramNode = {
  id: string
  icon: IconName
  /** Grid area inside its group. */
  area: string
  title: string
  body: string
  /** The hover card: how it works, and a footnote (where in the console, what it runs on). */
  more: string
  foot?: string
  extra?: ReactNode
}

export type DiagramGroup = {
  id: string
  label: string
  icon: IconName
  /** Dashed frame: something that is not ours (the tenant's scope, the user's cloud). */
  dashed?: boolean
  columns: number
  areas: string
  nodes: DiagramNode[]
}

export type DiagramEdge = { from: string; to: string; label?: string }

const CARD_W = 320

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    '--dg-on': theme.colors.primary.main,
    '--dg-off': theme.colors.border.strong,
    position: 'relative',
    display: 'grid',
    alignItems: 'stretch',
    [theme.breakpoints.down('md')]: { gridTemplateColumns: 'minmax(0, 1fr) !important' },
  }),
  group: css({
    position: 'relative',
    padding: theme.spacing(5, 2.5, 2.5),
    borderRadius: 12,
    border: `1px solid ${theme.colors.border.weak}`,
  }),
  dashed: css({ borderStyle: 'dashed', borderColor: theme.colors.border.medium }),
  label: css({
    position: 'absolute',
    top: theme.spacing(1.5),
    left: theme.spacing(2.5),
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  grid: css({
    display: 'grid',
    columnGap: theme.spacing(4),
    rowGap: theme.spacing(3),
    alignItems: 'center',
    [theme.breakpoints.down('md')]: {
      gridTemplateColumns: 'minmax(0, 1fr) !important',
      gridTemplateAreas: 'none !important',
      rowGap: theme.spacing(1.5),
      '& > *': { gridArea: 'auto !important' },
    },
  }),
  edges: css({
    position: 'absolute',
    inset: 0,
    width: '100%',
    height: '100%',
    pointerEvents: 'none',
    overflow: 'visible',
    zIndex: 1,
    [theme.breakpoints.down('md')]: { display: 'none' },
  }),
  edgeLabel: css({
    fontSize: 11,
    fontFamily: theme.typography.fontFamily,
    fill: theme.colors.text.secondary,
  }),
  edgeLabelBg: css({ fill: theme.colors.background.primary }),
  // Labels sit over the blocks so a long one is never cut by a block edge.
  labels: css({ zIndex: 3 }),
  node: css({
    position: 'relative',
    zIndex: 2,
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    padding: theme.spacing(1.5),
    borderRadius: 8,
    border: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.secondary,
    color: theme.colors.text.primary,
    textAlign: 'left',
    cursor: 'default',
    transition: 'border-color 150ms, opacity 150ms',
    '&:focus-visible': { outline: `2px solid ${theme.colors.primary.main}`, outlineOffset: 2 },
  }),
  nodeActive: css({ borderColor: theme.colors.primary.border }),
  nodeDim: css({ opacity: 0.4 }),
  head: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    fontWeight: theme.typography.fontWeightMedium,
  }),
  icon: css({ color: theme.colors.primary.text, flexShrink: 0 }),
  body: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    lineHeight: 1.45,
  }),
  card: css({
    position: 'absolute',
    zIndex: 5,
    width: CARD_W,
    padding: theme.spacing(2),
    borderRadius: 10,
    border: `1px solid ${theme.colors.border.medium}`,
    background: theme.colors.background.elevated,
    boxShadow: theme.shadows.z3,
    pointerEvents: 'none',
    [theme.breakpoints.down('md')]: { display: 'none' },
  }),
  cardTitle: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    fontWeight: theme.typography.fontWeightMedium,
  }),
  cardBody: css({
    margin: theme.spacing(1, 0, 0),
    color: theme.colors.text.secondary,
    lineHeight: 1.55,
    fontSize: theme.typography.body.fontSize,
  }),
  cardFoot: css({
    marginTop: theme.spacing(1.5),
    paddingTop: theme.spacing(1),
    borderTop: `1px solid ${theme.colors.border.weak}`,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
})

type Box = { x: number; y: number; w: number; h: number }

// A curve from the side of `a` facing `b` to the side of `b` facing `a`, and its midpoint.
function edgeGeometry(a: Box, b: Box) {
  const acx = a.x + a.w / 2
  const acy = a.y + a.h / 2
  const bcx = b.x + b.w / 2
  const bcy = b.y + b.h / 2
  if (Math.abs(bcx - acx) > Math.abs(bcy - acy) * 0.6) {
    const dir = bcx > acx ? 1 : -1
    const x1 = dir > 0 ? a.x + a.w : a.x
    const x2 = dir > 0 ? b.x : b.x + b.w
    const mx = (x1 + x2) / 2
    return {
      d: `M ${x1} ${acy} C ${mx} ${acy}, ${mx} ${bcy}, ${x2} ${bcy}`,
      mid: { x: mx, y: (acy + bcy) / 2 },
    }
  }
  const dir = bcy > acy ? 1 : -1
  const y1 = dir > 0 ? a.y + a.h : a.y
  const y2 = dir > 0 ? b.y : b.y + b.h
  const my = (y1 + y2) / 2
  return {
    d: `M ${acx} ${y1} C ${acx} ${my}, ${bcx} ${my}, ${bcx} ${y2}`,
    mid: { x: (acx + bcx) / 2, y: my },
  }
}

/** A path the page lights on its own (a step of a story); hover still wins. */
export type DiagramFocus = { nodes: string[]; edges: string[] }

export function Diagram({
  groups,
  edges,
  columns,
  focus,
  gap = 6,
}: {
  groups: DiagramGroup[]
  edges: DiagramEdge[]
  /** The root grid's columns, e.g. "1fr 2fr 1.4fr" (stacked on phones). */
  columns: string
  focus?: DiagramFocus | null
  /** Room between groups, in theme spacing units: links and their labels cross it. */
  gap?: number
}) {
  const theme = useTheme2()
  const styles = useStyles2(getStyles)
  const reduce = useReducedMotion() ?? false
  const markerId = useId().replace(/:/g, '')
  const root = useRef<HTMLDivElement>(null)
  const refs = useRef<Record<string, HTMLButtonElement | null>>({})
  const [boxes, setBoxes] = useState<Record<string, Box>>({})
  const [rootW, setRootW] = useState(0)
  const [active, setActive] = useState<string | null>(null)

  // Lines and cards follow the blocks wherever the layout puts them.
  useLayoutEffect(() => {
    const el = root.current
    if (!el) return
    const measure = () => {
      const base = el.getBoundingClientRect()
      const next: Record<string, Box> = {}
      for (const [id, node] of Object.entries(refs.current)) {
        const r = node?.getBoundingClientRect()
        if (r) next[id] = { x: r.left - base.left, y: r.top - base.top, w: r.width, h: r.height }
      }
      setBoxes(next)
      setRootW(base.width)
    }
    measure()
    // Blocks move without the root resizing (a step, a font, a relayout).
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    for (const node of Object.values(refs.current)) if (node) ro.observe(node)
    return () => ro.disconnect()
  }, [])

  const all = groups.flatMap((g) => g.nodes)
  const linked = (id: string) => {
    if (active !== null)
      return (
        active === id ||
        edges.some((e) => (e.from === active && e.to === id) || (e.to === active && e.from === id))
      )
    return !focus || focus.nodes.includes(id)
  }
  const shown = all.find((n) => n.id === active)
  const box = active ? boxes[active] : undefined
  // The card sits beside its block: right when there is room, else left.
  const cardLeft = box
    ? box.x + box.w + 12 + CARD_W <= rootW
      ? box.x + box.w + 12
      : Math.max(0, box.x - 12 - CARD_W)
    : 0

  const edgeOn = (e: DiagramEdge) =>
    active !== null ? e.from === active || e.to === active : !!focus?.edges.includes(`${e.from}-${e.to}`)
  const edgeFaded = (e: DiagramEdge) => (active !== null || !!focus) && !edgeOn(e)

  return (
    <div ref={root} className={styles.root} style={{ gridTemplateColumns: columns, gap: theme.spacing(gap) }}>
      <svg className={styles.edges} aria-hidden="true">
        <defs>
          <marker
            id={markerId}
            viewBox="0 0 8 8"
            refX="7"
            refY="4"
            markerWidth="7"
            markerHeight="7"
            orient="auto"
          >
            <path d="M 0 0 L 8 4 L 0 8 z" fill="context-stroke" />
          </marker>
        </defs>
        {edges.map((e, i) => {
          const a = boxes[e.from]
          const b = boxes[e.to]
          if (!a || !b) return null
          const { d } = edgeGeometry(a, b)
          const on = edgeOn(e)
          const faded = edgeFaded(e)
          return (
            <g
              key={`${e.from}-${e.to}`}
              style={{ opacity: faded ? 0.2 : 1, transition: 'opacity 150ms' }}
            >
              <motion.path
                d={d}
                fill="none"
                strokeWidth={on ? 2 : 1.25}
                markerEnd={`url(#${markerId})`}
                style={{ stroke: on ? 'var(--dg-on)' : 'var(--dg-off)' }}
                initial={{ pathLength: reduce ? 1 : 0 }}
                animate={{ pathLength: 1 }}
                transition={reduce ? { duration: 0 } : { duration: 0.6, delay: 0.1 + i * 0.05 }}
              />
            </g>
          )
        })}
      </svg>

      {groups.map((g) => (
        <div key={g.id} className={cx(styles.group, g.dashed && styles.dashed)}>
          <span className={styles.label}>
            <Icon name={g.icon} size="sm" />
            {g.label}
          </span>
          <div
            className={styles.grid}
            style={{
              gridTemplateColumns: `repeat(${g.columns}, minmax(0, 1fr))`,
              gridTemplateAreas: g.areas,
            }}
          >
            {g.nodes.map((n) => (
              <button
                type="button"
                key={n.id}
                ref={(el) => {
                  refs.current[n.id] = el
                }}
                aria-describedby={active === n.id ? `${markerId}-card` : undefined}
                style={{ gridArea: n.area }}
                className={cx(
                  styles.node,
                  active === n.id && styles.nodeActive,
                  !linked(n.id) && styles.nodeDim
                )}
                onMouseEnter={() => setActive(n.id)}
                onMouseLeave={() => setActive(null)}
                onFocus={() => setActive(n.id)}
                onBlur={() => setActive(null)}
              >
                <span className={styles.head}>
                  <Icon name={n.icon} className={styles.icon} />
                  {n.title}
                </span>
                <span className={styles.body}>{n.body}</span>
                {n.extra}
              </button>
            ))}
          </div>
        </div>
      ))}
      <svg className={cx(styles.edges, styles.labels)} aria-hidden="true">
        {edges.map((e) => {
          const a = boxes[e.from]
          const b = boxes[e.to]
          if (!e.label || !a || !b) return null
          const { mid } = edgeGeometry(a, b)
          return (
            <g
              key={`${e.from}-${e.to}`}
              transform={`translate(${mid.x} ${mid.y})`}
              style={{ opacity: edgeFaded(e) ? 0.2 : 1, transition: 'opacity 150ms' }}
            >
              <rect
                className={styles.edgeLabelBg}
                x={-(e.label.length * 3.1 + 6)}
                y={-8}
                width={e.label.length * 6.2 + 12}
                height={16}
                rx={4}
              />
              <text className={styles.edgeLabel} textAnchor="middle" dominantBaseline="central">
                {e.label}
              </text>
            </g>
          )
        })}
      </svg>

      <AnimatePresence>
        {shown && box && (
          <motion.div
            key={shown.id}
            id={`${markerId}-card`}
            role="tooltip"
            className={styles.card}
            style={{ left: cardLeft, top: box.y }}
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            transition={reduce ? { duration: 0 } : { duration: 0.15 }}
          >
            <div className={styles.cardTitle}>
              <Icon name={shown.icon} className={styles.icon} />
              {shown.title}
            </div>
            <p className={styles.cardBody}>{shown.more}</p>
            {shown.foot && <div className={styles.cardFoot}>{shown.foot}</div>}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

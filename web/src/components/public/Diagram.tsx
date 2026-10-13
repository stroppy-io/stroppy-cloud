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
  /** Runs on the user's side (the console in the browser): marked apart from the server's parts. */
  client?: boolean
  /** One concrete value, as a real test would have it ("PostgreSQL 17 · Patroni, 3 nodes"). */
  sample?: string
  /** The card's specifics: what can be chosen, grouped ("Engines": PostgreSQL, MySQL…). */
  facts?: DiagramFact[]
  extra?: ReactNode
}

export type DiagramFact = { label: string; items: string[] }

export type DiagramGroup = {
  id: string
  label: string
  icon: IconName
  /** Dashed frame: something that is not ours (the tenant's scope, the user's cloud). */
  dashed?: boolean
  /** What the parts stand on (Temporal, Kratos, S3…): a quiet row of chips under them, linked by hidden edges. */
  deps?: DiagramNode[]
  depsLabel?: string
  /** Cell in the root grid ("2 / 1"), when groups are laid out as a table. */
  place?: string
  columns: number
  areas: string
  nodes: DiagramNode[]
}

/** `hidden`: a relation that lights on hover and in steps but is not drawn (a block and what it runs on). */
export type DiagramEdge = { from: string; to: string; label?: string; hidden?: boolean }

const CARD_W = 340

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    '--dg-on': theme.colors.primary.main,
    '--dg-off': theme.colors.border.strong,
    position: 'relative',
    display: 'grid',
    alignItems: 'stretch',
    [theme.breakpoints.down('md')]: {
      gridTemplateColumns: 'minmax(0, 1fr) !important',
      '& > *': { gridArea: 'auto !important' },
    },
  }),
  group: css({
    position: 'relative',
    padding: theme.spacing(5, 2.5, 2.5),
    borderRadius: 12,
    border: `1px solid ${theme.colors.border.weak}`,
  }),
  dashed: css({ borderStyle: 'dashed', borderColor: theme.colors.border.medium }),
  clientNode: css({
    borderStyle: 'dashed',
    borderColor: theme.colors.primary.border,
    background: theme.colors.primary.transparent,
  }),
  depsRow: css({
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: theme.spacing(1),
    marginTop: theme.spacing(2),
    paddingTop: theme.spacing(1.5),
    borderTop: `1px dashed ${theme.colors.border.weak}`,
  }),
  depsLabel: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    marginRight: theme.spacing(0.5),
  }),
  dep: css({
    position: 'relative',
    zIndex: 2,
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    height: theme.spacing(4),
    padding: theme.spacing(0, 1.25),
    borderRadius: theme.shape.radius.pill,
    border: `1px dashed ${theme.colors.border.medium}`,
    background: 'none',
    color: theme.colors.text.primary,
    cursor: 'default',
    transition: 'border-color 150ms, opacity 150ms',
    '&:focus-visible': { outline: `2px solid ${theme.colors.primary.main}`, outlineOffset: 2 },
  }),
  depTitle: css({
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  depTech: css({
    color: theme.colors.primary.text,
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
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
    columnGap: theme.spacing(8),
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
    width: '100%',
    minWidth: 0,
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
  sample: css({
    color: theme.colors.primary.text,
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    lineHeight: 1.4,
    overflowWrap: 'break-word',
  }),
  facts: css({
    display: 'grid',
    gap: theme.spacing(1),
    margin: theme.spacing(1.5, 0, 0),
  }),
  factLabel: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    marginBottom: theme.spacing(0.5),
  }),
  factItems: css({ display: 'flex', flexWrap: 'wrap', gap: theme.spacing(0.5) }),
  factItem: css({
    padding: theme.spacing(0, 0.75),
    borderRadius: theme.shape.radius.pill,
    border: `1px solid ${theme.colors.border.medium}`,
    fontSize: theme.typography.bodySmall.fontSize,
    lineHeight: 1.6,
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
function edgeGeometry(a: Box, b: Box, vertical = false) {
  const acx = a.x + a.w / 2
  const acy = a.y + a.h / 2
  const bcx = b.x + b.w / 2
  const bcy = b.y + b.h / 2
  if (!vertical && Math.abs(bcx - acx) > Math.abs(bcy - acy) * 0.6) {
    const dir = bcx > acx ? 1 : -1
    const x1 = dir > 0 ? a.x + a.w : a.x
    const x2 = dir > 0 ? b.x : b.x + b.w
    const mx = (x1 + x2) / 2
    return {
      d: `M ${x1} ${acy} C ${mx} ${acy}, ${mx} ${bcy}, ${x2} ${bcy}`,
      mid: { x: mx, y: (acy + bcy) / 2 },
      across: true,
      at: (_y: number) => ({ x: mx, y: (acy + bcy) / 2 }),
    }
  }
  const dir = bcy > acy ? 1 : -1
  const y1 = dir > 0 ? a.y + a.h : a.y
  const y2 = dir > 0 ? b.y : b.y + b.h
  const my = (y1 + y2) / 2
  // The point of the curve at height `y` (it only moves one way vertically).
  const at = (y: number) => {
    const point = (t: number) => {
      const u = 1 - t
      return {
        x: u * u * u * acx + 3 * u * u * t * acx + 3 * u * t * t * bcx + t * t * t * bcx,
        y: u * u * u * y1 + 3 * u * u * t * my + 3 * u * t * t * my + t * t * t * y2,
      }
    }
    let lo = 0
    let hi = 1
    for (let i = 0; i < 24; i++) {
      const t = (lo + hi) / 2
      if ((point(t).y - y) * dir < 0) lo = t
      else hi = t
    }
    return point((lo + hi) / 2)
  }
  return {
    d: `M ${acx} ${y1} C ${acx} ${my}, ${bcx} ${my}, ${bcx} ${y2}`,
    mid: { x: (acx + bcx) / 2, y: my },
    across: false,
    at,
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
  columnGap,
}: {
  groups: DiagramGroup[]
  edges: DiagramEdge[]
  /** The root grid's columns, e.g. "1fr 2fr 1.4fr" (stacked on phones). */
  columns: string
  focus?: DiagramFocus | null
  /** Room between groups, in theme spacing units: links and their labels cross it. */
  gap?: number
  /** Room between columns of groups, when it differs from `gap`. */
  columnGap?: number
}) {
  const theme = useTheme2()
  const styles = useStyles2(getStyles)
  const reduce = useReducedMotion() ?? false
  const markerId = useId().replace(/:/g, '')
  const root = useRef<HTMLDivElement>(null)
  const refs = useRef<Record<string, HTMLElement | null>>({})
  const [boxes, setBoxes] = useState<Record<string, Box>>({})
  const [rootW, setRootW] = useState(0)
  const [active, setActive] = useState<string | null>(null)

  // Lines and cards follow the blocks wherever the layout puts them.
  useLayoutEffect(() => {
    const el = root.current
    if (!el) return
    const measure = () => {
      const base = el.getBoundingClientRect()
      // A scaled ancestor (a screen fitted to the viewport) scales the
      // measured boxes too; the drawing is in the root's own units.
      const k = el.offsetWidth ? base.width / el.offsetWidth : 1
      const next: Record<string, Box> = {}
      for (const [id, node] of Object.entries(refs.current)) {
        const r = node?.getBoundingClientRect()
        if (r)
          next[id] = {
            x: (r.left - base.left) / k,
            y: (r.top - base.top) / k,
            w: r.width / k,
            h: r.height / k,
          }
      }
      setBoxes(next)
      setRootW(el.offsetWidth)
    }
    measure()
    // Blocks move without the root resizing (a step, a font, a relayout).
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    for (const node of Object.values(refs.current)) if (node) ro.observe(node)
    return () => ro.disconnect()
  }, [])

  const all = groups.flatMap((g) => [...g.nodes, ...(g.deps ?? [])])
  const groupOf: Record<string, string> = Object.fromEntries(
    groups.flatMap((g) => [...g.nodes, ...(g.deps ?? [])].map((n) => [n.id, g.id]))
  )
  // A link between groups stacked one above the other runs top to bottom,
  // whatever the horizontal distance, and is labelled in the gap between them.
  const stacked = (e: DiagramEdge) => {
    const ga = boxes[`group:${groupOf[e.from]}`]
    const gb = boxes[`group:${groupOf[e.to]}`]
    return !!ga && !!gb && ga !== gb && (ga.y + ga.h <= gb.y || gb.y + gb.h <= ga.y)
  }
  const hoverable = (id: string) => ({
    ref: (el: HTMLElement | null) => {
      refs.current[id] = el
    },
    'aria-describedby': active === id ? `${markerId}-card` : undefined,
    onMouseEnter: () => setActive(id),
    onMouseLeave: () => setActive(null),
    onFocus: () => setActive(id),
    onBlur: () => setActive(null),
  })
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
    active !== null
      ? e.from === active || e.to === active
      : !!focus?.edges.includes(`${e.from}-${e.to}`)
  const edgeFaded = (e: DiagramEdge) => (active !== null || !!focus) && !edgeOn(e)

  return (
    <div
      ref={root}
      className={styles.root}
      style={{
        gridTemplateColumns: columns,
        rowGap: theme.spacing(gap),
        columnGap: theme.spacing(columnGap ?? gap),
      }}
    >
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
          if (e.hidden || !a || !b) return null
          const { d } = edgeGeometry(a, b, stacked(e))
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
        <div
          key={g.id}
          ref={(el) => {
            refs.current[`group:${g.id}`] = el
          }}
          className={cx(styles.group, g.dashed && styles.dashed)}
          style={g.place ? { gridArea: g.place } : undefined}
        >
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
                {...hoverable(n.id)}
                style={{ gridArea: n.area }}
                className={cx(
                  styles.node,
                  n.client && styles.clientNode,
                  active === n.id && styles.nodeActive,
                  !linked(n.id) && styles.nodeDim
                )}
              >
                <span className={styles.head}>
                  <Icon name={n.icon} className={styles.icon} />
                  {n.title}
                </span>
                <span className={styles.body}>{n.body}</span>
                {n.sample && <span className={styles.sample}>{n.sample}</span>}
                {n.extra}
              </button>
            ))}
          </div>
          {g.deps && g.deps.length > 0 && (
            <div className={styles.depsRow}>
              <span className={styles.depsLabel}>{g.depsLabel}</span>
              {g.deps.map((n) => (
                <button
                  type="button"
                  key={n.id}
                  {...hoverable(n.id)}
                  className={cx(
                    styles.dep,
                    active === n.id && styles.nodeActive,
                    !linked(n.id) && styles.nodeDim
                  )}
                >
                  <Icon name={n.icon} size="sm" className={styles.icon} />
                  <span className={styles.depTitle}>{n.title}</span>
                  {n.sample && <span className={styles.depTech}>{n.sample}</span>}
                </button>
              ))}
            </div>
          )}
        </div>
      ))}
      <svg className={cx(styles.edges, styles.labels)} aria-hidden="true">
        {edges.map((e) => {
          const a = boxes[e.from]
          const b = boxes[e.to]
          if (e.hidden || !e.label || !a || !b) return null
          const { mid, across, at } = edgeGeometry(a, b, stacked(e))
          // A sideways link between neighbours is short: its label sits above
          // the arrow, not on it, so the arrow stays visible. A link between
          // groups is labelled in the gap between them, clear of their contents.
          const ga = boxes[`group:${groupOf[e.from]}`]
          const gb = boxes[`group:${groupOf[e.to]}`]
          let pos = across ? { x: mid.x, y: mid.y - 13 } : mid
          if (stacked(e) && ga && gb) {
            const [upper, lower] = ga.y < gb.y ? [ga, gb] : [gb, ga]
            pos = at((upper.y + upper.h + lower.y) / 2)
          }
          return (
            <g
              key={`${e.from}-${e.to}`}
              transform={`translate(${pos.x} ${pos.y})`}
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
            {shown.facts && shown.facts.length > 0 && (
              <div className={styles.facts}>
                {shown.facts.map((f) => (
                  <div key={f.label}>
                    <div className={styles.factLabel}>{f.label}</div>
                    <div className={styles.factItems}>
                      {f.items.map((i) => (
                        <span key={i} className={styles.factItem}>
                          {i}
                        </span>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            )}
            {shown.foot && <div className={styles.cardFoot}>{shown.foot}</div>}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

import uPlot from 'uplot'

// Charts that look like the app's Grafana panels — 1px lines with an 8%
// fill, a quiet grid, muted axis labels, a legend and a tooltip — drawn by
// uPlot (what Grafana's TimeSeries draws with) and coloured by the kit's CSS
// variables, so they need no framework and follow data-theme.
//
// Two ways in:
// - mount(target, series, opts) draws into an element;
// - enhance(figure, series, opts) draws next to a static chart the page
//   already has (an SVG and a numeric table made without JavaScript) and
//   hides the static plot only once the live one is up. A failure leaves the
//   static chart as it was. See README "Progressive charts".
//
// The data is the page's own: values, units and series identities are not
// inferred, resampled or merged here. A null value is a gap and stays one.

export interface ChartSeries {
  title: string
  unit?: string
  /**
   * Lines: [time, value] pairs, time in seconds or milliseconds since the
   * epoch. Bars: [category index, value]. A null value is a missing
   * observation: the line breaks there.
   */
  points: [number, number | null][]
}

export type Tone = 'success' | 'warning' | 'error' | 'info' | 'primary' | 'muted' | 'mark'

/** A phase or event on the time axis: a dashed vertical line with a label. */
export interface ChartMarker {
  /** Time, in the same units as the points (epoch seconds or milliseconds). */
  at: number
  label: string
  tone?: Tone
}

export interface ChartOptions {
  /** Pixel height of the plot area (the legend adds to it). Default 200. */
  height?: number
  /** Unit shown on the axis when the series don't say. */
  unit?: string
  /** lines (default) or grouped bars over `categories`. */
  kind?: 'lines' | 'bars'
  /** Bars: the label of each category (index = the point's first value). */
  categories?: string[]
  /** Lines: smooth (default, the app's look) or linear — straight segments between observations. */
  curve?: 'smooth' | 'linear'
  /** Time axis: wall clock (default) or time elapsed since `origin`. */
  time?: 'clock' | 'elapsed'
  /** Elapsed time zero (epoch seconds or ms). Default: the first point. */
  origin?: number
  /** Phases and events drawn over the plot. */
  markers?: ChartMarker[]
  /** Accessible name of the drawn chart. */
  label?: string
}

export interface ChartHandle {
  /** Stops observing size and theme and removes the chart. */
  destroy(): void
}

const FILL_OPACITY = 0.08
const AXIS_FONT_SIZE = 12
const BAR_GROUP = 0.72
/** Elapsed-axis tick steps in seconds: round seconds, minutes and hours, never 1:40. */
const ELAPSED_STEPS = [
  1, 2, 5, 10, 15, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 10800, 21600, 43200, 86400,
]

function cssVar(el: Element, name: string): string {
  return getComputedStyle(el).getPropertyValue(name).trim()
}

/** #rrggbb → rgba with alpha. */
function alpha(color: string, a: number): string {
  const m = /^#([0-9a-f]{6})$/i.exec(color)
  if (!m) return color
  const n = Number.parseInt(m[1], 16)
  return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${a})`
}

/** "6 3" (the --st-dash-N value) → uPlot's dash array; none/empty → solid. */
function dashOf(value: string): number[] | undefined {
  const parts = value
    .split(/[\s,]+/)
    .map(Number)
    .filter((n) => Number.isFinite(n) && n > 0)
  return parts.length > 0 ? parts : undefined
}

const toSeconds = (t: number) => (t > 1e11 ? t / 1000 : t)

const compact = new Intl.NumberFormat('en', { maximumFractionDigits: 1, notation: 'compact' })
const precise = new Intl.NumberFormat('en', { maximumFractionDigits: 2 })

function formatValue(v: number | null | undefined, unit?: string, exact = false): string {
  if (v == null || Number.isNaN(v)) return '—'
  const n = exact || Math.abs(v) < 1000 ? precise.format(v) : compact.format(v)
  return unit ? `${n} ${unit}` : n
}

const pad = (n: number) => String(n).padStart(2, '0')

/** Grafana's time labels: 24-hour, seconds while the span is under an hour, the date past a day. */
function formatTime(t: number, span: number): string {
  const d = new Date(t * 1000)
  const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`
  if (span <= 3600) return `${hm}:${pad(d.getSeconds())}`
  if (span <= 86400) return hm
  return `${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${hm}`
}

function formatStamp(t: number): string {
  const d = new Date(t * 1000)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

/** Elapsed seconds as m:ss, h:mm:ss past an hour; negative before the origin. */
export function formatElapsed(s: number): string {
  const sign = s < 0 ? '−' : ''
  const v = Math.round(Math.abs(s))
  const h = Math.floor(v / 3600)
  const m = Math.floor((v % 3600) / 60)
  const sec = v % 60
  return h > 0 ? `${sign}${h}:${pad(m)}:${pad(sec)}` : `${sign}${m}:${pad(sec)}`
}

/**
 * One x axis for every series, as uPlot wants. A value the page gave as
 * null stays null (a gap: spanGaps is off); a slot that exists only because
 * another series has a point there is undefined, which uPlot steps over.
 */
export function align(series: ChartSeries[], shift = 0): uPlot.AlignedData {
  const tables = series.map(
    (s): uPlot.AlignedData => [
      s.points.map((p) => toSeconds(p[0]) - shift),
      s.points.map((p) => p[1]),
    ]
  )
  if (tables.length === 1) return tables[0]
  return uPlot.join(
    tables,
    tables.map((t) => t.slice(1).map(() => 1))
  )
}

interface Look {
  colors: string[]
  dashes: (number[] | undefined)[]
  muted: string
  line: string
  panel: string
  font: string
}

function lookOf(root: Element, n: number): Look {
  return {
    colors: Array.from({ length: n }, (_, i) => cssVar(root, `--st-series-${(i % 8) + 1}`)),
    // One series is a plain line; several differ by dash too, not by colour alone.
    dashes: Array.from({ length: n }, (_, i) =>
      n > 1 ? dashOf(cssVar(root, `--st-dash-${(i % 8) + 1}`)) : undefined
    ),
    muted: cssVar(root, '--st-muted'),
    line: cssVar(root, '--st-line'),
    panel: cssVar(root, '--st-panel'),
    font: `${AXIS_FONT_SIZE}px ${cssVar(root, '--st-font-sans') || 'sans-serif'}`,
  }
}

function tooltipPlugin(
  series: ChartSeries[],
  look: Look,
  title: (x: number) => string,
  unit: string | undefined,
  bars: boolean
): uPlot.Plugin {
  let tip: HTMLDivElement
  return {
    hooks: {
      init: (u) => {
        tip = document.createElement('div')
        tip.className = 'st-chart-tooltip'
        tip.hidden = true
        u.over.appendChild(tip)
        u.over.addEventListener('mouseleave', () => {
          tip.hidden = true
        })
      },
      setCursor: (u) => {
        const i = u.cursor.idx
        if (i == null || u.cursor.left == null || u.cursor.left < 0) {
          tip.hidden = true
          return
        }
        const head = document.createElement('div')
        head.className = 'st-chart-tooltip-time'
        head.textContent = title(u.data[0][i] as number)
        const rows = series.map((s, k) => {
          const row = document.createElement('div')
          row.className = 'st-chart-tooltip-row'
          const name = document.createElement('span')
          name.textContent = s.title
          const value = document.createElement('b')
          value.textContent = formatValue(u.data[k + 1][i] as number | null, s.unit ?? unit, true)
          row.append(bars ? square(k) : swatch(k, look), name, value)
          return row
        })
        tip.replaceChildren(head, ...rows)
        tip.hidden = false
        const { width } = u.over.getBoundingClientRect()
        const left = u.cursor.left + 12
        tip.style.left = `${left + tip.offsetWidth > width ? u.cursor.left - tip.offsetWidth - 12 : left}px`
        tip.style.top = `${Math.max(0, (u.cursor.top ?? 0) - tip.offsetHeight / 2)}px`
      },
    },
  }
}

const SVG = 'http://www.w3.org/2000/svg'

/** The series sample: a short line in its colour and dash (the static markup is the same). */
function swatch(i: number, look: Look): SVGSVGElement {
  const svg = document.createElementNS(SVG, 'svg')
  svg.setAttribute('class', 'st-swatch')
  svg.setAttribute('data-series', String((i % 8) + 1))
  svg.setAttribute('viewBox', '0 0 24 10')
  svg.setAttribute('aria-hidden', 'true')
  const l = document.createElementNS(SVG, 'line')
  l.setAttribute('x1', '0')
  l.setAttribute('y1', '5')
  l.setAttribute('x2', '24')
  l.setAttribute('y2', '5')
  if (look.dashes.length < 2) l.style.strokeDasharray = 'none'
  svg.appendChild(l)
  return svg
}

/** A bar series' sample: a square in its colour and hatch (the static markup is the same). */
function square(i: number): SVGSVGElement {
  ensureHatchDefs()
  const svg = document.createElementNS(SVG, 'svg')
  svg.setAttribute('class', 'st-swatch-bar')
  svg.setAttribute('data-series', String((i % 8) + 1))
  svg.setAttribute('viewBox', '0 0 12 12')
  svg.setAttribute('aria-hidden', 'true')
  const r = document.createElementNS(SVG, 'rect')
  r.setAttribute('width', '12')
  r.setAttribute('height', '12')
  r.setAttribute('rx', '2')
  svg.appendChild(r)
  return svg
}

/** Phase and event markers: a dashed vertical line and a label along the top. */
function markersPlugin(
  root: Element,
  markers: ChartMarker[],
  shift: number,
  look: Look
): uPlot.Plugin {
  return {
    hooks: {
      draw: (u) => {
        const ctx = u.ctx
        const dpr = uPlot.pxRatio
        ctx.save()
        ctx.font = `${AXIS_FONT_SIZE * dpr}px ${cssVar(root, '--st-font-sans') || 'sans-serif'}`
        ctx.textBaseline = 'top'
        for (const m of markers) {
          const x = u.valToPos(toSeconds(m.at) - shift, 'x', true)
          if (x < u.bbox.left || x > u.bbox.left + u.bbox.width) continue
          const color = cssVar(root, `--st-${m.tone ?? 'muted'}`) || look.muted
          ctx.strokeStyle = color
          ctx.lineWidth = dpr
          ctx.setLineDash([4 * dpr, 3 * dpr])
          ctx.beginPath()
          ctx.moveTo(Math.round(x) + 0.5, u.bbox.top)
          ctx.lineTo(Math.round(x) + 0.5, u.bbox.top + u.bbox.height)
          ctx.stroke()
          ctx.fillStyle = color
          // Below the top tick's label, which sits at the plot's top edge.
          ctx.fillText(m.label, x + 4 * dpr, u.bbox.top + (AXIS_FONT_SIZE + 4) * dpr)
        }
        ctx.restore()
      },
    },
  }
}

/**
 * Bar hatches, one per series so bars differ by more than colour: series 1
 * is solid, 2–8 lay a pattern in the panel colour over the series colour.
 * One description serves the live chart (a canvas pattern), the static SVG
 * (`hatchDefs()`, shipped as hatch.svg) and the legend.
 */
export const HATCH_TILE = 6
const HATCHES: (string | null)[] = [
  null,
  'M0 6 L6 0 M-1 1 L1 -1 M5 7 L7 5',
  'M0 0 L6 6 M5 -1 L7 1 M-1 5 L1 7',
  'M0 6 L6 0 M0 0 L6 6',
  'M0 3 L6 3',
  'M3 0 L3 6',
  'M3 3 m-1 0 a1 1 0 1 0 2 0 a1 1 0 1 0 -2 0',
  'M0 3 L3 0 M3 6 L6 3',
]
const HATCH_DOTS = 6 // index of the dotted hatch: filled, not stroked

/** The hatch of a series (1-based), as an SVG path in a HATCH_TILE square; null = solid. */
export function hatchPath(series: number): string | null {
  return HATCHES[(series - 1) % HATCHES.length]
}

/**
 * The <defs> a static SVG page pastes once (any <svg> in the document):
 * patterns #st-hatch-1..8 in the series colours, which `.st-chart-bar` and bar
 * legend swatches fill with. The kit's bundle ships it as hatch.svg.
 */
export function hatchDefs(): string {
  const patterns = HATCHES.map((d, i) => {
    const n = i + 1
    const marks = d
      ? i === HATCH_DOTS
        ? `<path d="${d}" style="fill:var(--st-panel)"/>`
        : `<path d="${d}" style="fill:none;stroke:var(--st-panel);stroke-width:1.5"/>`
      : ''
    return `<pattern id="st-hatch-${n}" width="${HATCH_TILE}" height="${HATCH_TILE}" patternUnits="userSpaceOnUse"><rect width="${HATCH_TILE}" height="${HATCH_TILE}" style="fill:var(--st-series-${n})"/>${marks}</pattern>`
  })
  return `<svg class="st-hatch-defs" width="0" height="0" aria-hidden="true" focusable="false" style="position:absolute"><defs>${patterns.join('')}</defs></svg>`
}

/** Puts the hatch patterns in the document once, for the live legend's swatches. */
function ensureHatchDefs(): void {
  if (document.getElementById('st-hatch-1')) return
  const holder = document.createElement('div')
  holder.innerHTML = hatchDefs()
  const svg = holder.firstElementChild
  if (svg) document.body.appendChild(svg)
}

/** A canvas pattern of a series' hatch in the current colours (device pixels). */
function hatchPattern(
  ctx: CanvasRenderingContext2D,
  series: number,
  color: string,
  panel: string
): CanvasPattern | string {
  const d = hatchPath(series)
  if (!d) return color
  const dpr = uPlot.pxRatio
  const tile = document.createElement('canvas')
  tile.width = tile.height = Math.round(HATCH_TILE * dpr)
  const c = tile.getContext('2d')
  if (!c) return color
  c.scale(dpr, dpr)
  c.fillStyle = color
  c.fillRect(0, 0, HATCH_TILE, HATCH_TILE)
  const path = new Path2D(d)
  if ((series - 1) % HATCHES.length === HATCH_DOTS) {
    c.fillStyle = panel
    c.fill(path)
  } else {
    c.strokeStyle = panel
    c.lineWidth = 1.5
    c.stroke(path)
  }
  return ctx.createPattern(tile, 'repeat') ?? color
}

/**
 * The y range of bars: always through zero, so a bar's length is its value
 * (a range cut above zero turns a 2.7× difference into a sliver against a
 * full column). Negative values hang below zero. No data at all reads 0..1.
 */
export function barRange(min: number | null, max: number | null): [number, number] {
  const lo = Math.min(0, min ?? 0)
  const hi = Math.max(0, max ?? 0)
  if (lo === hi) return [0, 1]
  const [a, b] = uPlot.rangeNum(lo, hi, 0.1, true)
  return [lo < 0 ? (a ?? lo) : 0, hi > 0 ? (b ?? hi) : 0]
}

/** Grouped bars: each category a group, each series one bar in it, side by side, from zero. */
function groupedBars(k: number, n: number): uPlot.Series.PathBuilder {
  return (u, seriesIdx, idx0, idx1) => {
    const path = new Path2D()
    const xs = u.data[0]
    const ys = u.data[seriesIdx]
    const step = Math.abs(u.valToPos(1, 'x', true) - u.valToPos(0, 'x', true))
    const group = step * BAR_GROUP
    const width = group / n
    const base = u.valToPos(0, 'y', true)
    for (let i = idx0; i <= idx1; i++) {
      const v = ys[i]
      if (v == null) continue
      const left = u.valToPos(xs[i], 'x', true) - group / 2 + k * width
      const top = u.valToPos(v, 'y', true)
      path.rect(left, Math.min(top, base), Math.max(1, width - 1), Math.abs(base - top))
    }
    return { stroke: path, fill: path }
  }
}

/**
 * Refuses what the kit cannot draw faithfully, so enhance() keeps the static
 * chart rather than approximating it.
 */
function check(series: ChartSeries[], opts: ChartOptions): void {
  if (series.length === 0) throw new Error('chart: no series')
  if (opts.kind != null && opts.kind !== 'lines' && opts.kind !== 'bars')
    throw new Error(`chart: kind ${String(opts.kind)} is not supported`)
  if (opts.kind === 'bars' && !opts.categories?.length)
    throw new Error('chart: bars need categories')
  if (opts.time != null && opts.time !== 'clock' && opts.time !== 'elapsed')
    throw new Error(`chart: time ${String(opts.time)} is not supported`)
  for (const s of series)
    for (const p of s.points)
      if (!Number.isFinite(p[0]) || (p[1] !== null && typeof p[1] !== 'number'))
        throw new Error(`chart: series ${s.title} has a point that is not [number, number | null]`)
}

function build(
  root: HTMLElement,
  series: ChartSeries[],
  opts: ChartOptions,
  onReady?: () => void
): uPlot {
  check(series, opts)
  const look = lookOf(root, series.length)
  const unit = opts.unit ?? series[0]?.unit
  const bars = opts.kind === 'bars'
  const elapsed = !bars && opts.time === 'elapsed'
  const first = series.flatMap((s) => s.points.map((p) => toSeconds(p[0]))).sort((a, b) => a - b)[0]
  const shift = elapsed ? (opts.origin != null ? toSeconds(opts.origin) : (first ?? 0)) : 0
  const categories = opts.categories ?? []
  const axis = {
    stroke: look.muted,
    font: look.font,
    grid: { stroke: look.line, width: 1 },
    ticks: { stroke: look.line, width: 1, size: 4 },
  }
  const xAxis: uPlot.Axis = bars
    ? {
        ...axis,
        size: 32,
        grid: { show: false },
        splits: () => categories.map((_, i) => i),
        values: (_u, ticks) => ticks.map((i) => categories[i] ?? ''),
      }
    : elapsed
      ? {
          ...axis,
          size: 32,
          space: 80,
          incrs: ELAPSED_STEPS,
          values: (_u, ticks) => ticks.map(formatElapsed),
        }
      : {
          ...axis,
          size: 32,
          space: 80,
          values: (u, ticks) => {
            const span = (u.scales.x.max ?? 0) - (u.scales.x.min ?? 0)
            return ticks.map((t) => formatTime(t, span))
          },
        }
  const title = bars
    ? (x: number) => categories[x] ?? String(x)
    : elapsed
      ? (x: number) => `+${formatElapsed(x)}`
      : formatStamp
  const plugins = [tooltipPlugin(series, look, title, unit, bars)]
  if (onReady) plugins.push({ hooks: { ready: () => onReady() } })
  if (opts.markers?.length && !bars) plugins.push(markersPlugin(root, opts.markers, shift, look))
  const plot = document.createElement('div')
  root.appendChild(plot)
  const u = new uPlot(
    {
      width: root.clientWidth || 600,
      height: opts.height ?? 200,
      legend: { show: false },
      cursor: {
        points: bars
          ? { show: false }
          : { size: 7, width: 2, fill: (u, i) => u.series[i].stroke as string },
        y: false,
        drag: { x: false, y: false },
      },
      scales: bars
        ? {
            x: { time: false, range: () => [-0.5, Math.max(categories.length, 1) - 0.5] },
            y: { range: (_u, min, max) => barRange(min, max) },
          }
        : { x: { time: !elapsed } },
      axes: [
        xAxis,
        { ...axis, size: 56, values: (_u, ticks) => ticks.map((v) => formatValue(v, unit)) },
      ],
      series: [
        {},
        ...series.map(
          (s, i): uPlot.Series => ({
            label: s.title,
            stroke: look.colors[i],
            width: bars ? 0 : 1,
            dash: bars ? undefined : look.dashes[i],
            fill: bars
              ? (u) => hatchPattern(u.ctx, (i % 8) + 1, look.colors[i], look.panel)
              : alpha(look.colors[i], FILL_OPACITY),
            paths: bars
              ? groupedBars(i, series.length)
              : opts.curve === 'linear'
                ? uPlot.paths.linear?.()
                : uPlot.paths.spline?.(),
            points: { show: false },
            spanGaps: false,
          })
        ),
      ],
      plugins,
    },
    bars
      ? [
          categories.map((_, i) => i),
          ...series.map((s) =>
            categories.map((_, i) => s.points.find((p) => p[0] === i)?.[1] ?? null)
          ),
        ]
      : align(series, shift),
    plot
  )
  if (series.length > 1) {
    const legend = document.createElement('ul')
    legend.className = 'st-legend st-chart-legend'
    series.forEach((s, i) => {
      const li = document.createElement('li')
      li.setAttribute('data-series', String((i % 8) + 1))
      // Bars show their colour and hatch, lines their colour and dash.
      li.append(bars ? square(i) : swatch(i, look), document.createTextNode(s.title))
      legend.appendChild(li)
    })
    root.appendChild(legend)
  }
  return u
}

/**
 * Draws the series into `target` (an element or a selector) and keeps it
 * fitted to its width and to its theme: the nearest data-theme and the
 * reader's colour scheme (data-theme="system"). It prints as drawn on
 * screen; for print-quality output use enhance() over a static chart.
 */
export function mount(
  target: HTMLElement | string,
  series: ChartSeries[],
  opts: ChartOptions = {}
): ChartHandle {
  return mountWith(target, series, opts)
}

function mountWith(
  target: HTMLElement | string,
  series: ChartSeries[],
  opts: ChartOptions,
  onReady?: () => void
): ChartHandle {
  const root = typeof target === 'string' ? document.querySelector<HTMLElement>(target) : target
  if (!root) throw new Error(`chart: no element ${String(target)}`)
  root.classList.add('st-chart')
  if (opts.label) {
    root.setAttribute('role', 'img')
    root.setAttribute('aria-label', opts.label)
  }
  let u: uPlot | undefined
  let first = onReady
  const render = () => {
    u?.destroy()
    u = undefined
    root.replaceChildren()
    const ready = first
    first = undefined
    u = build(root, series, opts, ready)
  }
  render()
  const resize = new ResizeObserver(() => {
    if (u && root.clientWidth && u.width !== root.clientWidth)
      u.setSize({ width: root.clientWidth, height: opts.height ?? 200 })
  })
  resize.observe(root)
  // Colours are read once per render: anything that changes them redraws.
  const themed = root.closest('[data-theme]') ?? document.documentElement
  const theme = new MutationObserver(render)
  theme.observe(themed, { attributes: true, attributeFilter: ['data-theme'] })
  const scheme = window.matchMedia?.('(prefers-color-scheme: dark)')
  scheme?.addEventListener('change', render)
  // Print is not redrawn: the browser copies a canvas before a redraw could
  // land, so a print-time render comes out blank. A chart that must print
  // well goes through enhance(), whose static SVG is what paper gets.
  return {
    destroy() {
      resize.disconnect()
      theme.disconnect()
      scheme?.removeEventListener('change', render)
      u?.destroy()
      root.replaceChildren()
    },
  }
}

/** What enhance() reports on its figure (data-st-chart) and in its events. */
export type EnhanceState = 'live' | 'failed'

// The live chart of each figure: a second enhance() replaces it cleanly.
const enhanced = new WeakMap<HTMLElement, ChartHandle>()

/**
 * Draws the live chart next to the static one inside `figure`
 * (`.st-chart-figure` holding `.st-chart-static`, and optionally a numeric
 * `.st-chart-data` table). Lines default to `curve: 'linear'` here, the way
 * a static chart joins its points.
 *
 * Once the live chart has drawn, the figure gets data-st-chart="live", the
 * static plot is hidden on screen (it stays for print) and an
 * `st-chart:live` event fires. On any failure — bad input, an unsupported
 * kind, an exception while building — the live container is removed, the
 * figure gets data-st-chart="failed", an `st-chart:failed` event fires with
 * `detail.error`, and the static chart stays as it was. Returns the handle,
 * or null on failure.
 */
export function enhance(
  figure: HTMLElement | string,
  series: ChartSeries[],
  opts: ChartOptions = {}
): ChartHandle | null {
  const root = typeof figure === 'string' ? document.querySelector<HTMLElement>(figure) : figure
  if (!root) return null
  enhanced.get(root)?.destroy()
  root.querySelector(':scope > .st-chart-live')?.remove()
  delete root.dataset.stChart
  const live = document.createElement('div')
  live.className = 'st-chart-live'
  const anchor = root.querySelector(':scope > .st-chart-static')
  if (anchor) anchor.after(live)
  else root.prepend(live)
  const fail = (error: unknown) => {
    enhanced.get(root)?.destroy()
    enhanced.delete(root)
    live.remove()
    root.dataset.stChart = 'failed'
    root.dispatchEvent(new CustomEvent('st-chart:failed', { bubbles: true, detail: { error } }))
  }
  try {
    const chart = mountWith(live, series, { curve: 'linear', ...opts }, () => {
      root.dataset.stChart = 'live'
      root.dispatchEvent(new CustomEvent('st-chart:live', { bubbles: true }))
    })
    const handle: ChartHandle = {
      destroy() {
        chart.destroy()
        live.remove()
        enhanced.delete(root)
        delete root.dataset.stChart
      },
    }
    enhanced.set(root, handle)
    return handle
  } catch (error) {
    fail(error)
    return null
  }
}

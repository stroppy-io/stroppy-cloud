import uPlot from 'uplot'

// A time-series chart that looks like the app's Grafana panels — 1px smooth
// lines with an 8% fill, a quiet grid, muted axis labels, a legend and a
// tooltip — drawn by uPlot (what Grafana's TimeSeries draws with) and
// coloured by the kit's CSS variables, so it needs no React and follows
// data-theme. The data has the shape of a run's metric series.

export interface ChartSeries {
  title: string
  unit?: string
  /** [time, value] pairs; time in seconds or milliseconds since the epoch. */
  points: [number, number | null][]
}

export interface ChartOptions {
  /** Pixel height of the plot area (the legend adds to it). Default 200. */
  height?: number
  /** Unit shown on the axis when the series don't say. */
  unit?: string
}

export interface ChartHandle {
  /** Stops observing size and theme and removes the chart. */
  destroy(): void
}

const FILL_OPACITY = 0.08
const AXIS_FONT_SIZE = 12

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

/** Aligns the series on one time axis, as uPlot wants. */
function align(series: ChartSeries[]): uPlot.AlignedData {
  const tables = series.map(
    (s): uPlot.AlignedData => [s.points.map((p) => toSeconds(p[0])), s.points.map((p) => p[1])]
  )
  return tables.length === 1 ? tables[0] : uPlot.join(tables)
}

function tooltipPlugin(series: ChartSeries[], colors: string[], unit?: string): uPlot.Plugin {
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
        const time = document.createElement('div')
        time.className = 'st-chart-tooltip-time'
        time.textContent = formatStamp(u.data[0][i] as number)
        const rows = series.map((s, k) => {
          const row = document.createElement('div')
          row.className = 'st-chart-tooltip-row'
          const swatch = document.createElement('i')
          swatch.style.background = colors[k]
          const name = document.createElement('span')
          name.textContent = s.title
          const value = document.createElement('b')
          value.textContent = formatValue(u.data[k + 1][i] as number | null, s.unit ?? unit, true)
          row.append(swatch, name, value)
          return row
        })
        tip.replaceChildren(time, ...rows)
        tip.hidden = false
        const { width } = u.over.getBoundingClientRect()
        const left = u.cursor.left + 12
        tip.style.left = `${left + tip.offsetWidth > width ? u.cursor.left - tip.offsetWidth - 12 : left}px`
        tip.style.top = `${Math.max(0, (u.cursor.top ?? 0) - tip.offsetHeight / 2)}px`
      },
    },
  }
}

function build(root: HTMLElement, series: ChartSeries[], opts: ChartOptions): uPlot {
  const muted = cssVar(root, '--st-muted')
  const line = cssVar(root, '--st-line')
  const font = `${AXIS_FONT_SIZE}px ${cssVar(root, '--st-font-sans') || 'sans-serif'}`
  const colors = series.map((_, i) => cssVar(root, `--st-series-${(i % 8) + 1}`))
  const unit = opts.unit ?? series[0]?.unit
  const axis = {
    stroke: muted,
    font,
    grid: { stroke: line, width: 1 },
    ticks: { stroke: line, width: 1, size: 4 },
  }
  const plot = document.createElement('div')
  root.appendChild(plot)
  const u = new uPlot(
    {
      width: root.clientWidth || 600,
      height: opts.height ?? 200,
      legend: { show: false },
      cursor: {
        points: { size: 7, width: 2, fill: (u, i) => u.series[i].stroke as string },
        y: false,
        drag: { x: false, y: false },
      },
      scales: { x: { time: true } },
      axes: [
        {
          ...axis,
          size: 32,
          space: 80,
          values: (u, ticks) => {
            const span = (u.scales.x.max ?? 0) - (u.scales.x.min ?? 0)
            return ticks.map((t) => formatTime(t, span))
          },
        },
        { ...axis, size: 56, values: (_u, ticks) => ticks.map((v) => formatValue(v, unit)) },
      ],
      series: [
        {},
        ...series.map((s, i) => ({
          label: s.title,
          stroke: colors[i],
          width: 1,
          fill: alpha(colors[i], FILL_OPACITY),
          paths: uPlot.paths.spline?.(),
          points: { show: false },
          spanGaps: true,
        })),
      ],
      plugins: [tooltipPlugin(series, colors, unit)],
    },
    align(series),
    plot
  )
  if (series.length > 1) {
    const legend = document.createElement('ul')
    legend.className = 'st-legend st-chart-legend'
    series.forEach((s, i) => {
      const li = document.createElement('li')
      li.style.setProperty('--st-tone', colors[i])
      li.textContent = s.title
      legend.appendChild(li)
    })
    root.appendChild(legend)
  }
  return u
}

/**
 * Draws the series into `target` (an element or a selector) and keeps it
 * fitted to its width and to the nearest data-theme.
 */
export function mount(
  target: HTMLElement | string,
  series: ChartSeries[],
  opts: ChartOptions = {}
): ChartHandle {
  const root = typeof target === 'string' ? document.querySelector<HTMLElement>(target) : target
  if (!root) throw new Error(`chart: no element ${String(target)}`)
  root.classList.add('st-chart')
  let u: uPlot | undefined
  const render = () => {
    u?.destroy()
    root.replaceChildren()
    u = build(root, series, opts)
  }
  render()
  const resize = new ResizeObserver(() => {
    if (u && root.clientWidth && u.width !== root.clientWidth)
      u.setSize({ width: root.clientWidth, height: opts.height ?? 200 })
  })
  resize.observe(root)
  // Colours are read once per render: a theme switch redraws.
  const themed = root.closest('[data-theme]') ?? document.documentElement
  const theme = new MutationObserver(render)
  theme.observe(themed, { attributes: true, attributeFilter: ['data-theme'] })
  return {
    destroy() {
      resize.disconnect()
      theme.disconnect()
      u?.destroy()
      root.replaceChildren()
    },
  }
}

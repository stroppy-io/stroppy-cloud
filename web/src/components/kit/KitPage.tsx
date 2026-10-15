import { Logo } from '@components/Logo'
import { Panel } from '@components/library/shared/Panel'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import { dateTime, type GrafanaTheme2, ThemeContext } from '@grafana/data'
import {
  Alert,
  Badge,
  Button,
  Card,
  Checkbox,
  Combobox,
  Field,
  Icon,
  Input,
  LegendDisplayMode,
  RadioButtonGroup,
  Stack,
  Switch,
  Tab,
  TabsBar,
  Text,
  TimeSeries,
  useStyles2,
  useTheme2,
} from '@grafana/ui'
import { seriesToFrames } from '@helpers/dataframe'
import { useMeasure } from '@hooks/useMeasure'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { type ChartSeries, mount } from '@/kit/chart'
import { BAND, BODY, MARK_VIEWBOX, RAYS } from '@/kit/logo'
import { type Mode, type Palette, tokens } from '@/kit/tokens'
import { getAppTheme, getGrafanaTheme } from '@/styles/theme'
import '@/kit/tokens.css'
import '@/kit/fonts.css'
import '@/kit/base.css'
import 'uplot/dist/uPlot.min.css'
import '@/kit/chart.css'

// The kit's showcase (dev only): the tokens of both modes, Grafana's
// components in the theme made from them next to how the app looks today, and
// plain HTML styled by base.css — the look a report or a site gets without
// React. The place to tune the palette before the app switches to it.

type Source = 'stroppy' | 'grafana'

// Ten minutes of a run every 5 s: two load levels with a dip, deterministic so
// both charts draw the same picture.
const START = 1_760_400_000
const SAMPLE: ChartSeries[] = [
  { title: 'main', unit: 'tps', base: 212, swing: 14 },
  { title: 'replica', unit: 'tps', base: 168, swing: 10 },
].map(({ title, unit, base, swing }, k) => ({
  title,
  unit,
  points: Array.from({ length: 121 }, (_, i): [number, number] => {
    const dip = i > 70 && i < 82 ? -60 : 0
    const v = base + swing * Math.sin(i / 7 + k) + 6 * Math.sin(i * 1.7 + k * 3) + dip
    return [START + i * 5, Math.round(v * 10) / 10]
  }),
}))

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    height: '100vh',
    overflowY: 'auto',
    background: theme.colors.background.canvas,
    color: theme.colors.text.primary,
  }),
  bar: css({
    position: 'sticky',
    top: 0,
    zIndex: 2,
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
    padding: theme.spacing(1.5, 3),
    background: theme.colors.background.primary,
    borderBottom: `1px solid ${theme.colors.border.weak}`,
  }),
  grow: css({ flex: 1 }),
  main: css({
    maxWidth: 1200,
    margin: '0 auto',
    padding: theme.spacing(3),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(4),
  }),
  section: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  swatches: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))',
    gap: theme.spacing(1),
  }),
  swatch: css({
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
    overflow: 'hidden',
    background: theme.colors.background.primary,
  }),
  chip: css({ height: theme.spacing(6), borderBottom: `1px solid ${theme.colors.border.weak}` }),
  chipText: css({
    padding: theme.spacing(0.75, 1),
    fontSize: theme.typography.bodySmall.fontSize,
    fontFamily: theme.typography.fontFamilyMonospace,
    display: 'flex',
    flexDirection: 'column',
  }),
  stage: css({
    padding: theme.spacing(3),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(3),
  }),
  doc: css({
    padding: theme.spacing(4),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
  }),
})

export function KitPage() {
  const { t } = useTranslation()
  const [mode, setMode] = useState<Mode>('dark')
  const [source, setSource] = useState<Source>('stroppy')
  const theme = source === 'stroppy' ? getAppTheme(mode) : getGrafanaTheme(mode)
  return (
    <ThemeContext.Provider value={theme}>
      <Showcase mode={mode} source={source} onMode={setMode} onSource={setSource} t={t} />
    </ThemeContext.Provider>
  )
}

function Showcase({
  mode,
  source,
  onMode,
  onSource,
  t,
}: {
  mode: Mode
  source: Source
  onMode: (m: Mode) => void
  onSource: (s: Source) => void
  t: (k: string) => string
}) {
  const styles = useStyles2(getStyles)
  return (
    <div className={styles.root}>
      <header className={styles.bar}>
        <Logo size={28} />
        <Text element="h1" variant="h4">
          {t('kit.title')}
        </Text>
        <div className={styles.grow} />
        <RadioButtonGroup
          size="sm"
          options={[
            { label: t('kit.mode.dark'), value: 'dark' },
            { label: t('kit.mode.light'), value: 'light' },
          ]}
          value={mode}
          onChange={onMode}
        />
        <RadioButtonGroup
          size="sm"
          options={[
            { label: t('kit.source.stroppy'), value: 'stroppy' },
            { label: t('kit.source.grafana'), value: 'grafana' },
          ]}
          value={source}
          onChange={onSource}
        />
      </header>
      <main className={styles.main}>
        <Section title={t('kit.palette.title')} hint={t('kit.palette.hint')}>
          <Swatches palette={tokens.color[mode]} series={tokens.series[mode]} />
        </Section>
        <Section title={t('kit.grafana.title')} hint={t('kit.grafana.hint')}>
          <GrafanaSample t={t} />
        </Section>
        <Section title={t('kit.doc.title')} hint={t('kit.doc.hint')}>
          <DocSample mode={mode} />
        </Section>
      </main>
    </div>
  )
}

function Section({
  title,
  hint,
  children,
}: {
  title: string
  hint: string
  children: React.ReactNode
}) {
  const styles = useStyles2(getStyles)
  return (
    <section className={styles.section}>
      <Text element="h2" variant="h3">
        {title}
      </Text>
      <Text color="secondary">{hint}</Text>
      {children}
    </section>
  )
}

function Swatches({ palette, series }: { palette: Palette; series: string[] }) {
  const styles = useStyles2(getStyles)
  const all: [string, string][] = [
    ...Object.entries(palette),
    ['mark', tokens.mark],
    ...series.map((v, i): [string, string] => [`series-${i + 1}`, v]),
  ]
  return (
    <div className={styles.swatches}>
      {all.map(([name, value]) => (
        <div key={name} className={styles.swatch}>
          <div className={styles.chip} style={{ background: value }} />
          <div className={styles.chipText}>
            <span>{name}</span>
            <Text color="secondary" variant="bodySmall">
              {value}
            </Text>
          </div>
        </div>
      ))}
    </div>
  )
}

function GrafanaSample({ t }: { t: (k: string) => string }) {
  const styles = useStyles2(getStyles)
  const [tab, setTab] = useState('runs')
  const [on, setOn] = useState(true)
  const [db, setDb] = useState<string | null>('postgres')
  return (
    <div className={styles.stage}>
      <Stack direction="column" gap={0.5}>
        <Text element="h3" variant="h2">
          {t('kit.sample.heading')}
        </Text>
        <Text color="secondary">{t('kit.sample.lead')}</Text>
        <Text variant="code">tpcc/tx · 2 × 4 vCPU · 16 GB</Text>
      </Stack>
      <Stack gap={1} wrap="wrap">
        <Button icon="play">{t('kit.sample.launch')}</Button>
        <Button variant="secondary" icon="copy">
          {t('kit.sample.clone')}
        </Button>
        <Button variant="secondary" fill="outline">
          {t('kit.sample.compare')}
        </Button>
        <Button variant="secondary" fill="text">
          {t('kit.sample.cancel')}
        </Button>
        <Button variant="destructive" icon="trash-alt">
          {t('kit.sample.delete')}
        </Button>
      </Stack>
      <Stack gap={1} wrap="wrap" alignItems="center">
        <StatusBadge status="running" />
        <StatusBadge status="completed" />
        <StatusBadge status="pending" />
        <StatusBadge status="failed" />
        <StatusBadge status="cancelled" />
        <Badge color="blue" text="PostgreSQL 17" />
        <Badge color="darkgrey" text="postgres + 1 replica + haproxy" />
        <Badge color="orange" icon="exclamation-triangle" text={t('kit.sample.warn')} />
        <Badge color="purple" icon="cube" text={t('kit.sample.image')} />
      </Stack>
      <TabsBar>
        {['runs', 'logs', 'metrics'].map((id) => (
          <Tab
            key={id}
            label={t(`kit.sample.tabs.${id}`)}
            active={tab === id}
            onChangeTab={() => setTab(id)}
            counter={id === 'runs' ? 12 : undefined}
          />
        ))}
      </TabsBar>
      <Stack gap={2} wrap="wrap" alignItems="flex-end">
        <Field label={t('kit.sample.name')} noMargin>
          <Input width={28} placeholder="tpcc-nightly" prefix={<Icon name="search" />} />
        </Field>
        <Field label={t('kit.sample.engine')} noMargin>
          <Combobox
            width={24}
            options={[
              { label: 'PostgreSQL', value: 'postgres' },
              { label: 'MySQL', value: 'mysql' },
              { label: 'YDB', value: 'ydb' },
            ]}
            value={db}
            onChange={(o) => setDb(o?.value ?? null)}
          />
        </Field>
        <Field label={t('kit.sample.keep')} noMargin>
          <Switch value={on} onChange={(e) => setOn(e.currentTarget.checked)} />
        </Field>
        <Checkbox label={t('kit.sample.favorite')} value={on} onChange={() => setOn(!on)} />
      </Stack>
      <Stack direction="column" gap={1}>
        <Alert severity="info" title={t('kit.sample.alerts.info')} />
        <Alert severity="success" title={t('kit.sample.alerts.success')} />
        <Alert severity="warning" title={t('kit.sample.alerts.warning')} />
        <Alert severity="error" title={t('kit.sample.alerts.error')} />
      </Stack>
      <Stack gap={2} wrap="wrap">
        <Panel title={t('kit.sample.panel')} description={t('kit.sample.panelHint')}>
          <Text>212.5 tps · p99 18 ms</Text>
        </Panel>
        <Card>
          <Card.Heading>{t('kit.sample.card')}</Card.Heading>
          <Card.Description>{t('kit.sample.cardHint')}</Card.Description>
        </Card>
      </Stack>
      <GrafanaChart />
    </div>
  )
}

/** The sample in Grafana's own TimeSeries, as the app draws a run's metrics. */
function GrafanaChart() {
  const theme = useTheme2()
  const [ref, { width }] = useMeasure<HTMLDivElement>()
  const frames = useMemo(
    () =>
      seriesToFrames(
        SAMPLE.map((s) => ({ name: s.title, points: s.points as number[][], unit: s.unit })),
        theme
      ),
    [theme]
  )
  const range = useMemo(() => {
    const from = dateTime(START * 1000)
    const to = dateTime((START + 600) * 1000)
    return { from, to, raw: { from, to } }
  }, [])
  return (
    <div ref={ref}>
      {width > 0 && (
        <TimeSeries
          frames={frames}
          width={width}
          height={200}
          timeRange={range}
          timeZone="browser"
          legend={{
            displayMode: LegendDisplayMode.List,
            placement: 'bottom',
            calcs: [],
            showLegend: true,
          }}
        />
      )}
    </div>
  )
}

/** Plain HTML in the kit's base.css: what a report page looks like. */
function DocSample({ mode }: { mode: Mode }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  return (
    <div className={`st-doc ${styles.doc}`} data-theme={mode}>
      <div className="st-row">
        <svg width="28" height="28" viewBox={MARK_VIEWBOX} aria-hidden="true">
          <g fill="var(--st-mark)">
            {[...RAYS, BAND].map((d) => (
              <path key={d} d={d} />
            ))}
          </g>
          <g fill="var(--st-muted)">
            {BODY.map((d) => (
              <path key={d} d={d} />
            ))}
          </g>
        </svg>
        <small>stroppy 6.0.0 · 2026-10-14</small>
      </div>

      <nav>
        <a href="#summary" aria-current="true">
          {t('kit.doc.nav.summary')}
        </a>
        <a href="#results">{t('kit.doc.nav.results')}</a>
        <a href="#setup">{t('kit.doc.nav.setup')}</a>
        <a href="#logs">{t('kit.doc.nav.logs')}</a>
      </nav>

      <h1>{t('kit.doc.h1')}</h1>
      <div className="st-tags">
        <span className="st-tag" data-tone="success">
          <span className="st-dot" />
          {t('kit.doc.status.completed')}
        </span>
        <span className="st-tag" data-tone="info">
          <span className="st-dot" data-pulse="" />
          {t('kit.doc.status.running')}
        </span>
        <span className="st-tag" data-tone="error">
          {t('kit.doc.status.failed')}
        </span>
        <span className="st-tag" data-tone="warning">
          {t('kit.doc.status.degraded')}
        </span>
        <span className="st-tag" data-tone="primary">
          PostgreSQL 17
        </span>
        <span className="st-tag">TPC-C</span>
        <span className="st-tag" data-tone="mark">
          {t('kit.doc.status.rating')}
        </span>
      </div>
      <p>
        {t('kit.doc.p1')} <a href="#run">{t('kit.doc.link')}</a>. {t('kit.doc.p2')}{' '}
        <mark>{t('kit.doc.mark')}</mark>, <abbr title="transactions per second">tps</abbr>{' '}
        <code>tpcc/tx</code>.
      </p>

      <div className="st-grid">
        <div className="st-stat">
          <span>{t('kit.doc.stat.tps')}</span>
          <strong>
            212.5<small>tps</small>
          </strong>
          <small data-tone="success">▲ 12.4% {t('kit.doc.stat.vsLast')}</small>
        </div>
        <div className="st-stat">
          <span>p99</span>
          <strong>
            18.0<small>ms</small>
          </strong>
          <small data-tone="error">▲ 3.1 ms {t('kit.doc.stat.vsLast')}</small>
        </div>
        <div className="st-stat" data-tone="warning">
          <span>{t('kit.doc.stat.errors')}</span>
          <strong>
            0.4<small>%</small>
          </strong>
          <small>{t('kit.doc.stat.errorsNote')}</small>
        </div>
        <div className="st-stat">
          <span>{t('kit.doc.stat.duration')}</span>
          <strong>
            10:00<small>min</small>
          </strong>
          <small>64 VU</small>
        </div>
      </div>

      <aside>
        <strong>{t('kit.doc.callout.info.title')}</strong>
        {t('kit.doc.callout.info.body')}
      </aside>
      <aside data-tone="success">{t('kit.doc.callout.success')}</aside>
      <aside data-tone="warning">
        <strong>{t('kit.doc.callout.warning.title')}</strong>
        {t('kit.doc.callout.warning.body')}
      </aside>
      <aside data-tone="error">{t('kit.doc.callout.error')}</aside>

      <h2 id="results">{t('kit.doc.h2')}</h2>
      <div className="st-panel">
        <header>
          {t('kit.doc.panel.title')}
          <small>{t('kit.doc.panel.note')}</small>
        </header>
        <table>
          <thead>
            <tr>
              <th>{t('kit.doc.segment')}</th>
              <th className="num">tps</th>
              <th className="num">{t('kit.doc.delta')}</th>
              <th className="num">p99, ms</th>
              <th>{t('kit.doc.share')}</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>warmup</td>
              <td className="num">184.0</td>
              <td className="num" data-tone="muted">
                —
              </td>
              <td className="num">21.4</td>
              <td>
                <span className="st-bar" style={{ ['--value' as string]: '46%' }} />
              </td>
            </tr>
            <tr data-tone="success">
              <td>main</td>
              <td className="num">212.5</td>
              <td className="num" data-tone="success">
                +12.4%
              </td>
              <td className="num">18.0</td>
              <td>
                <span
                  className="st-bar"
                  data-tone="success"
                  style={{ ['--value' as string]: '100%' }}
                />
              </td>
            </tr>
            <tr>
              <td>spike</td>
              <td className="num">151.2</td>
              <td className="num" data-tone="error">
                −8.9%
              </td>
              <td className="num" data-tone="warning">
                44.7
              </td>
              <td>
                <span
                  className="st-bar"
                  data-tone="warning"
                  style={{ ['--value' as string]: '71%' }}
                />
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div className="st-panel">
        <header>
          {t('kit.doc.chart')}
          <small>tps</small>
        </header>
        <KitChart series={SAMPLE} />
      </div>

      <div className="st-grid">
        <div className="st-panel">
          <header>{t('kit.doc.setup')}</header>
          <dl>
            <dt>{t('kit.doc.kv.engine')}</dt>
            <dd>PostgreSQL 17 · Patroni</dd>
            <dt>{t('kit.doc.kv.machines')}</dt>
            <dd>2 × 4 vCPU · 16 GB + 1 × 2 vCPU · 4 GB</dd>
            <dt>{t('kit.doc.kv.cloud')}</dt>
            <dd>Yandex Cloud · ru-central1-a</dd>
          </dl>
        </div>
        <div className="st-panel">
          <header>{t('kit.doc.series')}</header>
          <ul className="st-legend">
            {[1, 2, 3, 4, 5, 6, 7, 8].map((n) => (
              <li key={n} data-series={String(n)}>
                {t('kit.doc.seriesItem', { n })}
              </li>
            ))}
          </ul>
          <progress value={62} max={100} />
          <meter value={0.82} min={0} max={1} low={0.5} high={0.9} optimum={0.3} />
        </div>
      </div>

      <details>
        <summary>postgresql.conf</summary>
        <pre>{'shared_buffers = 4GB\nmax_connections = 200\njit = off'}</pre>
      </details>

      <blockquote>{t('kit.doc.quote')}</blockquote>
      <h3>{t('kit.doc.h3')}</h3>
      <ul>
        <li>{t('kit.doc.li1')}</li>
        <li>
          {t('kit.doc.li2')} <kbd>Ctrl</kbd>+<kbd>C</kbd>
        </li>
      </ul>
      <div className="st-row">
        <a className="st-button" data-variant="primary" href="#open">
          {t('kit.doc.open')}
        </a>
        <a className="st-button" href="#download">
          {t('kit.doc.download')}
        </a>
        <a className="st-button" data-variant="text" href="#compare">
          {t('kit.doc.compare')}
        </a>
      </div>
      <hr />
      <small>{t('kit.doc.footer')}</small>
    </div>
  )
}

/** The kit chart, mounted the way a report page would call it. */
function KitChart({ series }: { series: ChartSeries[] }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!ref.current) return
    const chart = mount(ref.current, series, { unit: 'tps' })
    return () => chart.destroy()
  }, [series])
  return <div ref={ref} />
}

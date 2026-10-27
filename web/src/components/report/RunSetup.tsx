import type { Schemas } from '@api/types'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Badge, Text, Tooltip, useStyles2 } from '@grafana/ui'
import { roleLabel } from '@helpers/roles'
import { Fragment } from 'react'
import { useTranslation } from 'react-i18next'
import { type ConfigField, useConfigFields, useParamsFields } from './config-fields'
import { SimpleTable, SubTitle, valueToText } from './ReportPrimitives'

type RunMachine = Schemas['RunMachine']
type RunTuning = Schemas['RunTuning']

// The setup of a run in the order a specialist reads it: what ran on what
// (stand), how the database was configured, how the load was given. Shared
// by the run page and the public report.

const getStyles = (theme: GrafanaTheme2) => ({
  lead: css({ margin: theme.spacing(0, 0, 1.5) }),
  hint: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    margin: theme.spacing(0, 0, 1),
  }),
  facts: css({
    display: 'flex',
    gap: theme.spacing(3),
    flexWrap: 'wrap',
    margin: theme.spacing(0, 0, 1.5),
  }),
  group: css({
    td: {
      background: theme.colors.background.secondary,
      fontWeight: theme.typography.fontWeightMedium,
      paddingTop: theme.spacing(1),
    },
  }),
  groupNote: css({
    fontWeight: theme.typography.fontWeightRegular,
    color: theme.colors.text.secondary,
    marginLeft: theme.spacing(1),
  }),
  // A value the run kept at the default fades: what differs from stock reads first.
  same: css({ 'td, td *': { color: theme.colors.text.disabled } }),
  dim: css({ color: theme.colors.text.secondary, whiteSpace: 'nowrap' }),
  nowrap: css({ whiteSpace: 'nowrap' }),
  described: css({ borderBottom: `1px dotted ${theme.colors.border.strong}`, cursor: 'help' }),
  extra: css({
    color: theme.colors.text.secondary,
    fontFamily: theme.typography.fontFamilyMonospace,
  }),
  driver: css({ margin: theme.spacing(1.5, 0, 0.5) }),
})

const mb = (v?: number) =>
  v === undefined ? '—' : v >= 1024 ? `${+(v / 1024).toFixed(1)} GB` : `${v} MB`
const pair = (a?: number, b?: number) =>
  a === undefined && b === undefined ? '—' : `${a ?? '—'} / ${b ?? '—'}`
const withUnit = (v: unknown, unit?: string) =>
  v === undefined ? '—' : `${valueToText(v)}${unit && typeof v !== 'object' ? ` ${unit}` : ''}`
const sameAsDefault = (v: unknown, f?: ConfigField) =>
  f?.default !== undefined && valueToText(v) === valueToText(f.default)

export interface RunSetupProps {
  kind?: string
  version?: string
  image?: string
  /** The database's parameters as the run had them (db.<kind>.params). */
  params?: Record<string, unknown>
  topology?: string
  provider?: string
  machines?: RunMachine[]
  tuning?: RunTuning
  /** The workload's segments. */
  segments?: unknown[]
}

export function RunSetup(p: RunSetupProps) {
  return (
    <>
      <StandSection {...p} />
      <ConfigSection {...p} />
      <LoadSection tuning={p.tuning} segments={p.segments} />
    </>
  )
}

function StandSection({ kind, version, image, topology, provider, machines }: RunSetupProps) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <>
      <SubTitle>{t('report.stand.title')}</SubTitle>
      <p className={styles.lead}>
        {t('report.stand.lead', {
          db: [kind, version].filter(Boolean).join(' '),
          topology: topology || '—',
          provider: provider || '—',
        })}
        {image && (
          <>
            {' '}
            <Text color="secondary">
              {t('report.stand.image')} <code>{image}</code>
            </Text>
          </>
        )}
      </p>
      {machines && machines.length > 0 && (
        <SimpleTable>
          <thead>
            <tr>
              <th>{t('report.stand.machine')}</th>
              <th>{t('report.stand.role')}</th>
              <th className="num">{t('report.stand.cpuRam')}</th>
              <th>{t('report.stand.disk')}</th>
              <th className="num">{t('report.stand.iops')}</th>
              <th className="num">{t('report.stand.mbps')}</th>
              <th className="num">
                <Tooltip content={t('report.stand.cacheHint')}>
                  <span className={styles.described}>{t('report.stand.cache')}</span>
                </Tooltip>
              </th>
            </tr>
          </thead>
          <tbody>
            {machines.map((m) => (
              <tr key={m.name}>
                <td className="mono">{m.name}</td>
                <td>{roleLabel(t, m.role)}</td>
                <td className="num">
                  {m.cpu ?? '—'} / {m.memory_gb !== undefined ? `${m.memory_gb} GB` : '—'}
                </td>
                <td>
                  {m.disk_gb !== undefined
                    ? `${m.disk_gb} GB${m.disk_type ? ` ${m.disk_type}` : ''}`
                    : '—'}
                </td>
                <td className="num">{pair(m.disk_read_iops, m.disk_write_iops)}</td>
                <td className="num">{pair(m.disk_read_mbps, m.disk_write_mbps)}</td>
                <td className="num">{m.cache_mb ? mb(m.cache_mb) : '—'}</td>
              </tr>
            ))}
          </tbody>
        </SimpleTable>
      )}
    </>
  )
}

interface Row {
  key: string
  label: string
  value: unknown
  field?: ConfigField
}

// Changed values first, then the ones left at the default; by name within each.
const byChange = (a: Row, b: Row) =>
  Number(sameAsDefault(a.value, a.field)) - Number(sameAsDefault(b.value, b.field)) ||
  a.label.localeCompare(b.label)

// Parameters the stand line or the tuning facts already name.
const SHOWN_ELSEWHERE = new Set(['version', 'image_tag', 'tuning'])

function ConfigSection({ kind, params, tuning }: RunSetupProps) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const paramFields = useParamsFields(kind)
  const auto = tuning?.auto ?? {}
  const configIds = Object.values(auto).flatMap((byId) => Object.keys(byId))
  const configFields = useConfigFields(configIds)

  const deploy: Row[] = Object.entries(params ?? {})
    .filter(([k]) => !SHOWN_ELSEWHERE.has(k))
    .map(([k, v]) => ({
      key: k,
      label: paramFields[k]?.title ?? k,
      value: v,
      field: paramFields[k],
    }))
    .sort(byChange)
  const engine = Object.entries(auto)
    .flatMap(([role, byId]) =>
      Object.entries(byId).map(([id, values]) => ({
        role,
        id,
        rows: Object.entries(values)
          .map(([k, v]) => ({ key: k, label: k, value: v, field: configFields[id]?.[k] }))
          .sort(byChange),
      }))
    )
    .sort((a, b) => a.role.localeCompare(b.role) || a.id.localeCompare(b.id))

  const renderRows = (rows: Row[], mono: boolean) =>
    rows.map((r) => {
      const same = sameAsDefault(r.value, r.field)
      const name = r.field?.description ? (
        <Tooltip content={r.field.description}>
          <span className={styles.described}>{r.label}</span>
        </Tooltip>
      ) : (
        r.label
      )
      return (
        <tr key={r.key} className={same ? styles.same : undefined}>
          <td className={mono ? 'mono' : undefined}>{name}</td>
          <td className={cx('mono', styles.nowrap)}>{withUnit(r.value, r.field?.unit)}</td>
          <td className={cx('mono', styles.dim)}>{withUnit(r.field?.default, r.field?.unit)}</td>
        </tr>
      )
    })

  return (
    <>
      <SubTitle>{t('report.config.title')}</SubTitle>
      {tuning && (
        <div className={styles.facts}>
          {tuning.mode && (
            <span>
              <Text color="secondary">{t('report.config.mode')} </Text>
              {t(`runs.overview.tuning.modes.${tuning.mode}`)}
            </span>
          )}
          {tuning.durability && (
            <span>
              <Text color="secondary">{t('report.config.durability')} </Text>
              <Text color={tuning.durability === 'relaxed' ? 'warning' : undefined}>
                {t(`runs.overview.tuning.durabilities.${tuning.durability}`)}
              </Text>
            </span>
          )}
        </div>
      )}
      <p className={styles.hint}>{t('report.config.hint')}</p>
      <SimpleTable>
        <thead>
          <tr>
            <th>{t('report.config.setting')}</th>
            <th>{t('report.config.value')}</th>
            <th>{t('report.config.default')}</th>
          </tr>
        </thead>
        <tbody>
          {deploy.length > 0 && (
            <>
              <tr className={styles.group}>
                <td colSpan={3}>{t('report.config.deploy')}</td>
              </tr>
              {renderRows(deploy, false)}
            </>
          )}
          {engine.map((g) => (
            <Fragment key={`${g.role}/${g.id}`}>
              <tr className={styles.group}>
                <td colSpan={3}>
                  {t('report.config.engine', {
                    role: roleLabel(t, g.role),
                    config: g.id.replace(/^cfg\./, ''),
                  })}
                  <span className={styles.groupNote}>{t('report.config.engineNote')}</span>
                </td>
              </tr>
              {renderRows(g.rows, true)}
            </Fragment>
          ))}
          {engine.length === 0 && (
            <tr className={styles.group}>
              <td colSpan={3}>
                {t('report.config.engineEmpty')}
                <span className={styles.groupNote}>
                  {tuning?.mode === 'stock' ? t('report.config.stock') : t('report.config.unknown')}
                </span>
              </td>
            </tr>
          )}
        </tbody>
      </SimpleTable>
    </>
  )
}

interface Segment {
  name?: string
  workload?: Record<string, unknown> & {
    script?: string
    scale_factor?: number
    load_workers?: number
    retry_attempts?: number
    pacing?: boolean
  }
  run?: { vus?: number; duration?: string; iterations?: number }
}

// Nested driver options as dotted keys (pool.maxConns), as auto names them.
function flatten(v: Record<string, unknown>, prefix = ''): [string, unknown][] {
  return Object.entries(v).flatMap(([k, x]) =>
    x && typeof x === 'object' && !Array.isArray(x)
      ? flatten(x as Record<string, unknown>, `${prefix}${k}.`)
      : [[`${prefix}${k}`, x] as [string, unknown]]
  )
}

const POOLED = new Set(['postgres', 'crdb', 'picodata', 'mysql', 'ydb'])
const SEGMENT_COLUMNS = new Set([
  'script',
  'scale_factor',
  'load_workers',
  'retry_attempts',
  'pacing',
])

function LoadSection({ tuning, segments }: { tuning?: RunTuning; segments?: unknown[] }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const client = tuning?.client
  const settings = flatten(client?.settings ?? {}).sort(([a], [b]) => a.localeCompare(b))
  const auto = new Set(client?.auto ?? [])
  const segs = (segments ?? []) as Segment[]
  const noPool =
    !!client?.driver_type &&
    POOLED.has(client.driver_type) &&
    !settings.some(([k]) => k.startsWith('pool.') || k.startsWith('url.pool'))
  return (
    <>
      <SubTitle>{t('report.load.title')}</SubTitle>
      {segs.length > 0 && (
        <SimpleTable>
          <thead>
            <tr>
              <th>{t('report.load.segment')}</th>
              <th>{t('report.load.script')}</th>
              <th className="num">{t('report.load.scale')}</th>
              <th className="num">{t('report.load.vus')}</th>
              <th className="num">{t('report.load.loadWorkers')}</th>
              <th className="num">{t('report.load.retries')}</th>
              <th>{t('report.load.pacing')}</th>
              <th className="num">{t('report.load.duration')}</th>
              <th>{t('report.load.other')}</th>
            </tr>
          </thead>
          <tbody>
            {segs.map((s, i) => {
              const w = s.workload ?? {}
              const other = Object.entries(w)
                .filter(([k]) => !SEGMENT_COLUMNS.has(k))
                .map(([k, v]) => `${k}=${valueToText(v)}`)
                .join(' ')
              return (
                <tr key={s.name ?? i}>
                  <td className="mono">{s.name ?? i + 1}</td>
                  <td className="mono">{w.script ?? '—'}</td>
                  <td className="num">{w.scale_factor ?? '—'}</td>
                  <td className="num">{s.run?.vus ?? '—'}</td>
                  <td className="num">{w.load_workers ?? '—'}</td>
                  <td className="num">{w.retry_attempts ?? '—'}</td>
                  <td>{w.pacing ? t('report.load.pacingOn') : t('report.load.pacingOff')}</td>
                  <td className="num">
                    {s.run?.duration ??
                      (s.run?.iterations
                        ? t('report.load.iterations', { n: s.run.iterations })
                        : '—')}
                  </td>
                  <td className={styles.extra}>{other || '—'}</td>
                </tr>
              )
            })}
          </tbody>
        </SimpleTable>
      )}
      {client?.driver_type && (
        <p className={styles.driver}>
          <Text color="secondary">{t('report.load.driver')} </Text>
          <code>{client.driver_type}</code>
        </p>
      )}
      {noPool && (
        <Alert severity="warning" title="">
          {t('report.load.noPool')}
        </Alert>
      )}
      {settings.length > 0 && (
        <SimpleTable>
          <thead>
            <tr>
              <th>{t('report.load.setting')}</th>
              <th>{t('report.config.value')}</th>
            </tr>
          </thead>
          <tbody>
            {settings.map(([k, v]) => (
              <tr key={k}>
                <td className="mono">
                  {k}{' '}
                  {auto.has(k) && (
                    <Tooltip content={t('report.load.autoHint')}>
                      <span>
                        <Badge text={t('report.load.auto')} color="blue" />
                      </span>
                    </Tooltip>
                  )}
                </td>
                <td className="mono">{valueToText(v)}</td>
              </tr>
            ))}
          </tbody>
        </SimpleTable>
      )}
    </>
  )
}

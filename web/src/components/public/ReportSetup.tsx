import { catalogQueries } from '@api/queries/catalog'
import type { Schemas, SharedRun } from '@api/types'
import { KeyValueList } from '@components/KeyValueList'
import type { Schema } from '@components/schema/engine'
import { SchemaForm } from '@components/schema/SchemaForm'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, Tag, Text, useStyles2 } from '@grafana/ui'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  ReportSection,
  SimpleTable,
  SubTitle,
  useReportStyles,
  valueToText,
} from './ReportPrimitives'

type TopologyPreview = Schemas['TopologyPreview']

const getStyles = (theme: GrafanaTheme2) => ({
  two: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)',
    gap: theme.spacing(3),
    [theme.breakpoints.down('md')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  topo: css({
    display: 'flex',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    alignItems: 'center',
    marginTop: theme.spacing(1),
  }),
  node: css({
    display: 'inline-flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: 2,
    padding: theme.spacing(1, 1.5),
    border: `1px solid ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    minWidth: 96,
  }),
  nodeRole: css({ fontWeight: theme.typography.fontWeightMedium }),
  nodeSub: css({
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
  }),
  count: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    color: theme.colors.text.secondary,
  }),
  flows: css({
    marginTop: theme.spacing(1),
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    display: 'flex',
    gap: theme.spacing(1.5),
    flexWrap: 'wrap',
  }),
  segHead: css({
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'baseline',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(1),
  }),
  readonlyForm: css({ pointerEvents: 'none', 'button, input, select': { opacity: 0.9 } }),
})

function Topology({ topo }: { topo: TopologyPreview }) {
  const styles = useStyles2(getStyles)
  return (
    <>
      <div className={styles.topo}>
        {topo.nodes.map((n) => (
          <div key={`${n.role}-${n.engine ?? ''}`} className={styles.node}>
            <span className={styles.nodeRole}>{n.role}</span>
            <span className={styles.nodeSub}>
              {n.engine ?? ''} <span className={styles.count}>×{n.count}</span>
            </span>
            {n.colocated_with && (
              <span className={styles.nodeSub}>
                <Icon name="link" size="xs" /> {n.colocated_with}
              </span>
            )}
          </div>
        ))}
      </div>
      {topo.flows && topo.flows.length > 0 && (
        <div className={styles.flows}>
          {topo.flows.map((f) => (
            <span key={`${f.from}-${f.to}-${f.port ?? ''}`}>
              {f.from} → {f.to} · {f.protocol}
              {f.port ? `:${f.port}` : ''}
            </span>
          ))}
        </div>
      )}
    </>
  )
}

function ParamsBlock({ kind, params }: { kind?: string; params?: Record<string, unknown> }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const rs = useReportStyles()
  const schema = useQuery({
    ...catalogQueries.schema(`db.${kind ?? 'unknown'}.params`),
    enabled: !!kind,
    retry: false,
  })
  const entries = Object.entries(params ?? {})
  if (!entries.length) return <span className={rs.muted}>{t('public.setup.noParams')}</span>
  if (schema.data && Array.isArray((schema.data as { fields?: unknown }).fields))
    return (
      <div className={styles.readonlyForm}>
        <SchemaForm
          schema={schema.data as unknown as Schema}
          value={params}
          onChange={() => undefined}
          readOnly
          dense
          showComputed={false}
        />
      </div>
    )
  return (
    <SimpleTable>
      <tbody>
        {entries.map(([k, v]) => (
          <tr key={k}>
            <th>{k}</th>
            <td className="mono">{valueToText(v)}</td>
          </tr>
        ))}
      </tbody>
    </SimpleTable>
  )
}

interface Segment {
  name?: string
  workload?: { script?: string } & Record<string, unknown>
  run?: { executor?: string; vus?: number; duration?: string } & Record<string, unknown>
  thresholds?: Record<string, unknown>
}

export function ReportSetup({ run, index }: { run: SharedRun; index: number }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const rs = useReportStyles()
  const db = run.database
  const wl = run.workload
  const segments = (wl?.segments ?? []) as Segment[]
  const sizes = run.summary.sizes ?? {}
  return (
    <ReportSection
      id="setup"
      index={index}
      title={t('public.setup.title')}
      hint={t('public.setup.hint')}
    >
      <div className={styles.two}>
        <div>
          <SubTitle>{t('public.setup.database')}</SubTitle>
          <KeyValueList
            items={[
              { label: t('public.setup.kind'), value: db?.kind ?? run.summary.db_kind },
              { label: t('public.setup.version'), value: db?.version ?? run.summary.db_version },
              ...(db?.image
                ? [{ label: t('public.setup.image'), value: <code>{db.image}</code> }]
                : []),
              { label: t('public.setup.provider'), value: run.summary.provider_kind ?? '—' },
              { label: t('public.setup.league'), value: run.summary.league ?? '—' },
              {
                label: t('public.setup.sizes'),
                value: Object.keys(sizes).length
                  ? Object.entries(sizes)
                      .map(
                        ([role, s]) =>
                          `${role}: ${s.size}${s.disk?.gb ? ` · ${s.disk.gb} GB${s.disk.type ? ` ${s.disk.type}` : ''}` : ''}`
                      )
                      .join(', ')
                  : '—',
              },
            ]}
          />
          <SubTitle>
            {t('public.setup.topology')}{' '}
            <Text color="secondary" variant="body">
              · {run.topology?.label ?? run.summary.topology_label ?? '—'}
              {run.topology?.node_count !== undefined &&
                ` · ${t('public.setup.nodes', { count: run.topology.node_count })}`}
            </Text>
          </SubTitle>
          {run.topology ? <Topology topo={run.topology} /> : <span className={rs.muted}>—</span>}
        </div>
        <div>
          <SubTitle>{t('public.setup.params')}</SubTitle>
          <ParamsBlock kind={db?.kind} params={db?.params} />
        </div>
      </div>

      <SubTitle>
        {t('public.setup.workload')}{' '}
        <Text color="secondary" variant="body">
          · {run.summary.workload_name ?? ''} · {t('public.setup.stroppy')}{' '}
          {wl?.stroppy_version ?? '—'} · {t('public.setup.protocol')}{' '}
          {wl?.protocol ?? run.summary.protocol ?? '—'}
        </Text>
      </SubTitle>
      <div className={rs.cards}>
        {segments.map((seg, i) => (
          <div key={seg.name ?? i} className={rs.card}>
            <div className={styles.segHead}>
              <Text weight="medium">{seg.name ?? `${t('public.setup.segment')} ${i + 1}`}</Text>
              {seg.workload?.script && <Tag name={String(seg.workload.script)} />}
            </div>
            <KeyValueList
              items={[
                { label: t('public.setup.executor'), value: seg.run?.executor ?? '—' },
                { label: t('public.setup.vus'), value: seg.run?.vus ?? '—' },
                { label: t('public.setup.duration'), value: seg.run?.duration ?? '—' },
                ...Object.entries(seg.workload ?? {})
                  .filter(([k]) => k !== 'script')
                  .map(([k, v]) => ({ label: k, value: valueToText(v) })),
                ...(seg.thresholds
                  ? [
                      {
                        label: t('public.setup.thresholds'),
                        value: Object.entries(seg.thresholds)
                          .map(([k, v]) => `${k} ≤ ${valueToText(v)}`)
                          .join(', '),
                      },
                    ]
                  : []),
              ]}
            />
          </div>
        ))}
      </div>

      {run.machines && run.machines.length > 0 && (
        <>
          <SubTitle>{t('public.setup.machines')}</SubTitle>
          <SimpleTable>
            <thead>
              <tr>
                <th>{t('public.setup.machine.name')}</th>
                <th>{t('public.setup.machine.role')}</th>
                <th>{t('public.setup.machine.size')}</th>
                <th className="num">{t('public.setup.machine.cpu')}</th>
                <th className="num">{t('public.setup.machine.ram')}</th>
                <th className="num">{t('public.setup.machine.disk')}</th>
                <th>{t('public.setup.machine.type')}</th>
                <th>{t('public.setup.machine.location')}</th>
              </tr>
            </thead>
            <tbody>
              {run.machines.map((m) => (
                <tr key={m.name}>
                  <td className="mono">{m.name}</td>
                  <td>{m.role}</td>
                  <td>{m.size}</td>
                  <td className="num">{m.cpu ?? '—'}</td>
                  <td className="num">{m.memory_gb !== undefined ? `${m.memory_gb} GB` : '—'}</td>
                  <td className="num">
                    {m.disk_gb !== undefined
                      ? `${m.disk_gb} GB${m.disk_type ? ` ${m.disk_type}` : ''}`
                      : '—'}
                  </td>
                  <td className="mono">{m.instance_type ?? '—'}</td>
                  <td>{m.location ?? '—'}</td>
                </tr>
              ))}
            </tbody>
          </SimpleTable>
        </>
      )}
    </ReportSection>
  )
}

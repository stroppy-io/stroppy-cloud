import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Icon, useStyles2 } from '@grafana/ui'
import { Handle, type NodeProps, Position } from '@xyflow/react'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'
import type { CardNodeData, MachineNodeData, PlainNodeData, TopoNode } from './build-graph'

const getStyles = (theme: GrafanaTheme2) => ({
  handle: css({
    opacity: 0,
    width: 6,
    height: 6,
    minWidth: 0,
    minHeight: 0,
    border: 0,
    pointerEvents: 'none',
  }),
  machine: css({
    width: '100%',
    height: '100%',
    boxSizing: 'border-box',
    background: theme.colors.background.primary,
    border: `1px solid ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.default,
    cursor: 'pointer',
    transition: 'box-shadow 0.15s',
    '&:hover': { boxShadow: theme.shadows.z1, borderColor: theme.colors.border.strong },
  }),
  selected: css({
    boxShadow: `0 0 0 2px ${theme.colors.primary.border}`,
    '&:hover': { boxShadow: `0 0 0 2px ${theme.colors.primary.border}` },
  }),
  machineHead: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(1, 1.5, 0),
    minWidth: 0,
  }),
  machineTitle: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.body.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    flex: '1 1 auto',
    minWidth: 0,
  }),
  machineSub: css({
    padding: theme.spacing(0, 1.5),
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
  machineEmpty: css({
    margin: theme.spacing(1, 1.5, 0),
    padding: theme.spacing(1),
    border: `1px dashed ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.disabled,
    textAlign: 'center',
  }),
  dot: css({
    flex: '0 0 auto',
    width: theme.spacing(1),
    height: theme.spacing(1),
    borderRadius: theme.shape.radius.circle,
  }),
  card: css({
    width: '100%',
    height: '100%',
    boxSizing: 'border-box',
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(0, 1.25),
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderLeftWidth: 3,
    borderRadius: theme.shape.radius.default,
    cursor: 'pointer',
    '&:hover': { borderColor: theme.colors.border.strong },
  }),
  cardText: css({ minWidth: 0, flex: '1 1 auto' }),
  cardTitle: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
  cardSub: css({
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
  plain: css({
    width: '100%',
    height: '100%',
    boxSizing: 'border-box',
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(0, 1.5),
    background: theme.colors.background.primary,
    border: `1px dashed ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.secondary,
  }),
})

function SideHandles({ className }: { className: string }) {
  return (
    <>
      <Handle id="tl" type="target" position={Position.Left} className={className} />
      <Handle id="sl" type="source" position={Position.Left} className={className} />
      <Handle id="tr" type="target" position={Position.Right} className={className} />
      <Handle id="sr" type="source" position={Position.Right} className={className} />
      <Handle id="sb" type="source" position={Position.Bottom} className={className} />
      <Handle id="tt" type="target" position={Position.Top} className={className} />
    </>
  )
}

export const MachineNode = memo(function MachineNode({ data, selected }: NodeProps<TopoNode>) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const d = data as MachineNodeData
  const sub = [d.role, d.size, d.address].filter(Boolean).join(' · ')
  return (
    <div className={cx(styles.machine, selected && styles.selected)}>
      <SideHandles className={styles.handle} />
      <div className={styles.machineHead}>
        <span
          className={styles.dot}
          style={{ background: d.statusColor }}
          title={t(`common.status.${d.status}`, { defaultValue: d.status })}
        />
        <span className={styles.machineTitle} title={d.name}>
          {d.name}
        </span>
        <span
          className={styles.dot}
          style={{ background: d.presenceColor, boxShadow: `0 0 0 2px ${d.accent}` }}
          title={`${t('runs.topology.fields.presence')}: ${t(`common.status.${d.presence}`, { defaultValue: d.presence })}`}
        />
      </div>
      <div className={styles.machineSub} title={sub}>
        {sub}
      </div>
      {d.componentCount === 0 && (
        <div className={styles.machineEmpty}>{t('runs.topology.noContainers')}</div>
      )}
    </div>
  )
})

export const CardNode = memo(function CardNode({ data, selected }: NodeProps<TopoNode>) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const d = data as CardNodeData
  const sub = [d.engine, d.image].filter(Boolean).join(' · ')
  return (
    <div
      className={cx(styles.card, selected && styles.selected)}
      style={{ borderLeftColor: d.accent }}
      title={d.id}
    >
      <SideHandles className={styles.handle} />
      <span
        className={styles.dot}
        style={{ background: d.statusColor }}
        title={t(`common.status.${d.status}`, { defaultValue: d.status })}
      />
      <div className={styles.cardText}>
        <div className={styles.cardTitle} style={{ color: d.accent }}>
          {d.role}
        </div>
        <div className={styles.cardSub} title={sub}>
          {sub || d.id}
        </div>
      </div>
    </div>
  )
})

export const PlainNode = memo(function PlainNode({ data }: NodeProps<TopoNode>) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const d = data as PlainNodeData
  return (
    <div className={styles.plain}>
      <SideHandles className={styles.handle} />
      <Icon name={d.kind === 'server' ? 'cloud' : 'database'} />
      <div className={styles.cardText}>
        <div className={styles.cardTitle} style={{ color: d.accent }}>
          {d.kind === 'server' ? t('runs.topology.server') : d.title}
        </div>
        <div className={styles.cardSub}>
          {d.kind === 'server'
            ? t('runs.topology.serverHint')
            : (d.subtitle ?? t('runs.topology.external'))}
        </div>
      </div>
    </div>
  )
})

export const topologyNodeTypes = { machine: MachineNode, card: CardNode, plain: PlainNode }

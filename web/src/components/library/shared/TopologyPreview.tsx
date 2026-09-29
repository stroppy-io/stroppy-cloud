import type { Schemas } from '@api/types'
import { css } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Icon, Stack, Text, Tooltip, useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'

type Topology = Schemas['TopologyPreview']

const ROLE_ICON: Record<string, IconName> = {
  master: 'database',
  replica: 'database',
  storage: 'database',
  database: 'database',
  node: 'database',
  instance: 'database',
  managed: 'cloud',
  etcd: 'layer-group',
  haproxy: 'exchange-alt',
  proxysql: 'exchange-alt',
  maxscale: 'exchange-alt',
  pgbouncer: 'plug',
  runner: 'rocket',
  external: 'external-link-alt',
  noop: 'circle',
}

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))',
    gap: theme.spacing(1),
  }),
  node: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
    padding: theme.spacing(1.5),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    minHeight: 84,
  }),
  primary: css({ borderColor: theme.colors.primary.border }),
  colocated: css({ borderStyle: 'dashed', background: 'transparent' }),
  count: css({
    fontSize: theme.typography.h3.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: 1,
  }),
  role: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    textTransform: 'capitalize',
  }),
  engine: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
  }),
  flows: css({
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(0.5, 2),
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    fontFamily: theme.typography.fontFamilyMonospace,
  }),
  empty: css({ color: theme.colors.text.secondary, padding: theme.spacing(2) }),
})

const PRIMARY_ROLES = new Set(['master', 'storage', 'node', 'instance', 'managed'])

// Nodes-by-role cards + data flows. Read-only view of what the server derived from params.
export function TopologyPreview({
  topology,
  compact,
}: {
  topology: Topology | undefined
  compact?: boolean
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  if (!topology?.nodes.length)
    return <div className={styles.empty}>{t('library.topology.empty')}</div>
  return (
    <Stack direction="column" gap={1.5}>
      {!compact && (
        <Stack gap={1} alignItems="center">
          <Text weight="medium">{topology.label}</Text>
          <Text color="secondary">
            · {t('library.topology.nodes', { count: topology.node_count ?? 0 })}
          </Text>
        </Stack>
      )}
      <div className={styles.grid}>
        {topology.nodes.map((n) => (
          <Tooltip
            key={n.role}
            content={
              n.colocated_with
                ? t('library.topology.colocated', { role: n.role, host: n.colocated_with })
                : t('library.topology.machines', { count: n.count, role: n.role })
            }
          >
            <div
              className={`${styles.node} ${PRIMARY_ROLES.has(n.role) ? styles.primary : ''} ${n.colocated_with ? styles.colocated : ''}`}
            >
              <div className={styles.role}>
                <Icon name={ROLE_ICON[n.role] ?? 'cube'} size="sm" />
                {n.role}
              </div>
              <div className={styles.count}>×{n.count}</div>
              <div className={styles.engine}>
                {n.engine ?? ''}
                {n.colocated_with ? ` @ ${n.colocated_with}` : ''}
              </div>
            </div>
          </Tooltip>
        ))}
      </div>
      {!compact && topology.flows?.length ? (
        <div className={styles.flows}>
          {topology.flows.map((f) => (
            <span key={`${f.from}-${f.to}-${f.port}`}>
              {f.from} <Icon name="arrow-right" size="xs" /> {f.to} · {f.protocol}
              {f.port ? `:${f.port}` : ''}
            </span>
          ))}
        </div>
      ) : null}
    </Stack>
  )
}

import { runQueries } from '@api/queries/runs'
import type { ConnectivityPair, RunConnectivity } from '@api/types'
import { ErrorState } from '@app/ErrorState'
import { SectionTitle } from '@app/PageHeader'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Badge,
  type BadgeColor,
  Button,
  Collapse,
  Icon,
  LoadingPlaceholder,
  Stack,
  Text,
  useStyles2,
} from '@grafana/ui'
import { connectivityRows, connectivityTone } from '@helpers/connectivity'
import { useTenant } from '@hooks/useTenant'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  okLine: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  okIcon: css({ color: theme.colors.success.text }),
  scroll: css({ overflowX: 'auto' }),
  table: css({
    width: '100%',
    borderCollapse: 'collapse',
    fontSize: theme.typography.bodySmall.fontSize,
    'th, td': {
      padding: theme.spacing(0.75, 1),
      borderBottom: `1px solid ${theme.colors.border.weak}`,
      textAlign: 'left',
      whiteSpace: 'nowrap',
      verticalAlign: 'top',
    },
    th: { color: theme.colors.text.secondary, fontWeight: theme.typography.fontWeightMedium },
    'tr:last-child td': { borderBottom: 0 },
    'th.num, td.num': { textAlign: 'right', fontVariantNumeric: 'tabular-nums' },
  }),
  mono: css({ fontFamily: theme.typography.fontFamilyMonospace }),
  error: css({
    color: theme.colors.text.secondary,
    whiteSpace: 'normal',
    wordBreak: 'break-word',
    minWidth: theme.spacing(30),
  }),
  pre: css({
    margin: 0,
    maxHeight: theme.spacing(60),
    overflow: 'auto',
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    whiteSpace: 'pre',
    background: theme.colors.background.secondary,
    borderRadius: theme.shape.radius.default,
    padding: theme.spacing(1.5),
  }),
})

const outcomeColor: Record<ConnectivityPair['outcome'], BadgeColor> = {
  ok: 'green',
  refused: 'orange',
  timeout: 'red',
  other: 'red',
}

function PairsTable({ rows }: { rows: ConnectivityPair[] }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  return (
    <div className={styles.scroll}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>{t('runs.overview.connectivity.from')}</th>
            <th>{t('runs.overview.connectivity.to')}</th>
            <th>{t('runs.overview.connectivity.outcome')}</th>
            <th className="num">{t('runs.overview.connectivity.latency')}</th>
            <th>{t('runs.overview.connectivity.error')}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((p) => (
            <tr key={`${p.from}>${p.address ?? p.to}:${p.port}`}>
              <td className={styles.mono}>{p.from}</td>
              <td>
                <span className={styles.mono}>
                  {p.to}:{p.port}
                </span>
                {p.container && (
                  <Text color="secondary" variant="bodySmall">
                    {' '}
                    {p.container}
                  </Text>
                )}
                {p.critical && p.outcome !== 'ok' && (
                  <>
                    {' '}
                    <Badge text={t('runs.overview.connectivity.required')} color="red" />
                  </>
                )}
              </td>
              <td>
                <Badge
                  text={t(`runs.overview.connectivity.outcomes.${p.outcome}`)}
                  color={outcomeColor[p.outcome]}
                />
              </td>
              <td className="num">
                {p.latency_ms !== undefined ? `${p.latency_ms.toFixed(1)} ms` : '—'}
                {p.attempts && p.attempts > 1 ? ` ×${p.attempts}` : ''}
              </td>
              <td className={styles.error}>{p.error ?? ''}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// One machine's network snapshot: fetched when opened.
function Snapshot({
  runId,
  machine,
  artifactId,
}: {
  runId: string
  machine: string
  artifactId: string
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const [open, setOpen] = useState(false)
  const q = useQuery({ ...runQueries.artifactText(slug, runId, artifactId), enabled: open })
  return (
    <Collapse
      label={t('runs.overview.connectivity.snapshotOf', { machine })}
      isOpen={open}
      onToggle={() => setOpen(!open)}
      collapsible
    >
      {q.isPending ? (
        <LoadingPlaceholder text={t('common.misc.loading')} />
      ) : q.isError ? (
        <ErrorState error={q.error} onRetry={() => void q.refetch()} compact />
      ) : (
        <pre className={styles.pre}>{q.data}</pre>
      )}
    </Collapse>
  )
}

// The connectivity check after deploy: one calm line when every path answered; otherwise the
// failed paths first and the network snapshots of the machines on both ends.
export function ConnectivityPanel({ runId, check }: { runId: string; check: RunConnectivity }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const [showAll, setShowAll] = useState(false)
  const tone = connectivityTone(check)
  if (tone === 'ok') {
    return (
      <div className={styles.okLine}>
        <Icon name="check-circle" className={styles.okIcon} />
        <span>{t('runs.overview.connectivity.allOk', { total: check.total })}</span>
      </div>
    )
  }
  const rows = connectivityRows(check, showAll)
  return (
    <Stack direction="column" gap={1}>
      <SectionTitle
        right={
          <Button size="sm" variant="secondary" fill="text" onClick={() => setShowAll(!showAll)}>
            {showAll
              ? t('runs.overview.connectivity.hideOk')
              : t('runs.overview.connectivity.showAll', { total: check.total })}
          </Button>
        }
      >
        {t('runs.overview.connectivity.title')}
      </SectionTitle>
      <Alert
        severity={tone}
        title={t('runs.overview.connectivity.summary', {
          failed: check.failed,
          total: check.total,
        })}
      >
        {tone === 'error'
          ? t('runs.overview.connectivity.critical')
          : t('runs.overview.connectivity.warning')}
      </Alert>
      <PairsTable rows={rows} />
      {check.snapshots.length > 0 && (
        <Stack direction="column" gap={0.5}>
          <Text variant="bodySmall" color="secondary">
            {t('runs.overview.connectivity.snapshotsHint')}
          </Text>
          {check.snapshots.map((s) => (
            <Snapshot
              key={s.artifact_id}
              runId={runId}
              machine={s.machine}
              artifactId={s.artifact_id}
            />
          ))}
        </Stack>
      )}
    </Stack>
  )
}

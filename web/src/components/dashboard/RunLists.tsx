import type { RatingEntry, Run, Schedule, SuiteRun } from '@api/types'
import { AppLink } from '@app/AppLink'
import { RelativeTime } from '@components/RelativeTime'
import { StatusBadge } from '@components/StatusBadge'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Badge, Text, useStyles2 } from '@grafana/ui'
import { formatDuration, formatMetric } from '@helpers/format'
import { relativeTime } from '@helpers/time'
import { useTranslation } from 'react-i18next'
import { Panel, PanelEmpty } from './Panel'

const getStyles = (theme: GrafanaTheme2) => ({
  row: css({
    display: 'grid',
    gridTemplateColumns: '20px minmax(0, 1fr) auto',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    padding: theme.spacing(0.75, 2),
    color: 'inherit',
    textDecoration: 'none',
    '&:hover': { background: theme.colors.action.hover, color: 'inherit' },
  }),
  main: css({ minWidth: 0 }),
  name: css({
    fontWeight: theme.typography.fontWeightMedium,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  sub: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  right: css({
    textAlign: 'right',
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    whiteSpace: 'nowrap',
    fontVariantNumeric: 'tabular-nums',
  }),
  mono: css({ fontFamily: theme.typography.fontFamilyMonospace, color: theme.colors.text.primary }),
  progress: css({
    height: 4,
    borderRadius: 2,
    background: theme.colors.background.secondary,
    overflow: 'hidden',
    marginTop: theme.spacing(0.5),
    div: { height: '100%', background: theme.colors.info.main, transition: 'width .4s' },
  }),
  rank: css({
    width: 20,
    textAlign: 'center',
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.secondary,
    fontVariantNumeric: 'tabular-nums',
  }),
})

function ViewAll({
  to,
  slug,
  label,
}: {
  to: '/t/$slug/runs' | '/t/$slug/suites' | '/t/$slug/schedules' | '/t/$slug/rating'
  slug: string
  label: string
}) {
  return (
    <AppLink to={to} params={{ slug }}>
      <Text variant="bodySmall">{label}</Text>
    </AppLink>
  )
}

export function LiveNow({ runs, slug }: { runs: Run[]; slug: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <Panel
      title={
        <>
          {t('dashboard.live.title')}{' '}
          {runs.length > 0 && <Badge text={String(runs.length)} color="blue" />}
        </>
      }
      right={<ViewAll to="/t/$slug/runs" slug={slug} label={t('dashboard.live.viewAll')} />}
    >
      {runs.length === 0 ? (
        <PanelEmpty>{t('dashboard.live.empty')}</PanelEmpty>
      ) : (
        runs.map((r) => (
          <AppLink
            key={r.id}
            to="/t/$slug/runs/$id"
            params={{ slug, id: r.id }}
            plain
            className={styles.row}
          >
            <StatusBadge status={r.status} iconOnly />
            <div className={styles.main}>
              <div className={styles.name}>{r.name}</div>
              <div className={styles.sub}>
                {t(`common.phase.${r.phase}`)}
                {r.summary?.segment ? ` · ${t('runs.segment', { name: r.summary.segment })}` : ''}
                {' · '}
                {r.summary?.db_kind} {r.summary?.db_version} · {r.summary?.workload_name}
              </div>
              {r.status !== 'pending' && (
                <div className={styles.progress}>
                  <div style={{ width: `${r.summary?.progress_pct ?? 0}%` }} />
                </div>
              )}
            </div>
            <div className={styles.right}>
              <div className={styles.mono}>
                {t('runs.progress', { pct: r.summary?.progress_pct ?? 0 })}
              </div>
              <RelativeTime value={r.started_at ?? r.created_at} />
            </div>
          </AppLink>
        ))
      )}
    </Panel>
  )
}

export function RecentRuns({ runs, slug }: { runs: Run[]; slug: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <Panel
      title={t('dashboard.recent.title')}
      right={<ViewAll to="/t/$slug/runs" slug={slug} label={t('dashboard.recent.viewAll')} />}
    >
      {runs.length === 0 ? (
        <PanelEmpty>{t('dashboard.recent.empty')}</PanelEmpty>
      ) : (
        runs.map((r) => (
          <AppLink
            key={r.id}
            to="/t/$slug/runs/$id"
            params={{ slug, id: r.id }}
            plain
            className={styles.row}
          >
            <StatusBadge status={r.status} iconOnly />
            <div className={styles.main}>
              <div className={styles.name}>{r.name}</div>
              <div className={styles.sub}>
                {r.summary?.db_kind} {r.summary?.db_version} · {r.summary?.topology_label} ·{' '}
                {r.summary?.workload_name}
                {r.status_reason ? ` · ${r.status_reason}` : ''}
              </div>
            </div>
            <div className={styles.right}>
              <div className={styles.mono}>
                {r.summary?.headline?.tps !== undefined
                  ? formatMetric(r.summary.headline.tps, 'tps')
                  : formatDuration(r.duration ?? undefined)}
              </div>
              <RelativeTime value={r.finished_at ?? r.created_at} />
            </div>
          </AppLink>
        ))
      )}
    </Panel>
  )
}

export function RecentSuiteRuns({ items, slug }: { items: SuiteRun[]; slug: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <Panel
      title={t('dashboard.suiteRuns.title')}
      right={<ViewAll to="/t/$slug/suites" slug={slug} label={t('dashboard.suiteRuns.viewAll')} />}
    >
      {items.length === 0 ? (
        <PanelEmpty>{t('dashboard.suiteRuns.empty')}</PanelEmpty>
      ) : (
        items.map((s) => (
          <AppLink
            key={s.id}
            to="/t/$slug/suite-runs/$id"
            params={{ slug, id: s.id }}
            plain
            className={styles.row}
          >
            <StatusBadge status={s.status} iconOnly />
            <div className={styles.main}>
              <div className={styles.name}>{s.name}</div>
              <div className={styles.sub}>
                {s.suite.name} ·{' '}
                {t('dashboard.suiteRuns.cells', { done: s.progress.done, total: s.progress.total })}
                {s.progress.failed
                  ? ` · ${s.progress.failed} ${t('common.status.failed').toLowerCase()}`
                  : ''}
              </div>
              {(s.status === 'running' || s.status === 'pending') && (
                <div className={styles.progress}>
                  <div style={{ width: `${s.progress.pct ?? 0}%` }} />
                </div>
              )}
            </div>
            <div className={styles.right}>
              <RelativeTime value={s.finished_at ?? s.started_at ?? s.created_at} />
            </div>
          </AppLink>
        ))
      )}
    </Panel>
  )
}

export function Upcoming({ items, slug }: { items: Schedule[]; slug: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <Panel
      title={t('dashboard.upcoming.title')}
      right={
        <ViewAll to="/t/$slug/schedules" slug={slug} label={t('dashboard.upcoming.viewAll')} />
      }
    >
      {items.length === 0 ? (
        <PanelEmpty>{t('dashboard.upcoming.empty')}</PanelEmpty>
      ) : (
        items.map((s) => (
          <AppLink
            key={s.id}
            to="/t/$slug/schedules/$id"
            params={{ slug, id: s.id }}
            plain
            className={styles.row}
          >
            <StatusBadge status={s.last_run?.status ?? 'pending'} iconOnly />
            <div className={styles.main}>
              <div className={styles.name}>{s.name}</div>
              <div className={styles.sub}>
                {s.target.name ?? s.target.id} · <span className={styles.mono}>{s.cron}</span> ·{' '}
                {s.timezone}
              </div>
            </div>
            <div className={styles.right}>
              {t('dashboard.upcoming.next', { when: relativeTime(s.next_run_at) })}
            </div>
          </AppLink>
        ))
      )}
    </Panel>
  )
}

export function TopResults({ items, slug }: { items: RatingEntry[]; slug: string }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <Panel
      title={t('dashboard.top.title')}
      right={<ViewAll to="/t/$slug/rating" slug={slug} label={t('dashboard.top.viewAll')} />}
    >
      {items.length === 0 ? (
        <PanelEmpty>{t('dashboard.top.empty')}</PanelEmpty>
      ) : (
        items.map((e, i) =>
          e.run_id ? (
            <AppLink
              key={`${e.league}-${e.rank}-${i}`}
              to="/t/$slug/runs/$id"
              params={{ slug, id: e.run_id }}
              plain
              className={styles.row}
            >
              <span className={styles.rank}>{e.rank}</span>
              <div className={styles.main}>
                <div className={styles.name}>
                  {e.db_kind} {e.db_version} · {e.topology_label}
                </div>
                <div className={styles.sub}>
                  {e.workload_name} · {t('dashboard.top.league', { league: e.league })} ·{' '}
                  {e.provider_kind}
                </div>
              </div>
              <div className={styles.right}>
                <div className={styles.mono}>{formatMetric(e.value, e.unit)}</div>
                <RelativeTime value={e.run_at} />
              </div>
            </AppLink>
          ) : null
        )
      )}
    </Panel>
  )
}

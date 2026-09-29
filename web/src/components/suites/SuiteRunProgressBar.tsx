import type { SuiteRun } from '@api/types'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Tooltip, useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5), minWidth: 0 }),
  bar: css({
    display: 'flex',
    height: 6,
    borderRadius: theme.shape.radius.pill,
    overflow: 'hidden',
    background: theme.colors.background.secondary,
  }),
  barLg: css({ height: 10 }),
  seg: css({ height: '100%', transition: 'width .4s ease' }),
  done: css({ background: theme.colors.success.main }),
  failed: css({ background: theme.colors.error.main }),
  running: css({
    background: theme.colors.info.main,
    backgroundImage: `linear-gradient(90deg, transparent 0 40%, ${theme.colors.info.shade} 40% 60%, transparent 60%)`,
    backgroundSize: '24px 100%',
    animation: 'slide 1.2s linear infinite',
    '@keyframes slide': { to: { backgroundPosition: '24px 0' } },
  }),
  cancelled: css({ background: theme.colors.text.disabled }),
  legend: css({
    display: 'flex',
    gap: theme.spacing(1.5),
    flexWrap: 'wrap',
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
  }),
  dot: css({
    display: 'inline-block',
    width: 8,
    height: 8,
    borderRadius: '50%',
    marginRight: 4,
    verticalAlign: 'middle',
  }),
})

// Segmented done / failed / running / pending bar with an optional legend line.
export function SuiteRunProgressBar({
  progress,
  size = 'sm',
  legend,
}: {
  progress: SuiteRun['progress']
  size?: 'sm' | 'lg'
  legend?: boolean
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const total = Math.max(1, progress.total)
  const cancelled = progress.cancelled ?? 0
  // Server categories are disjoint: total = done + failed + cancelled + running + pending.
  const ok = progress.done
  const pct = (n: number) => `${(n / total) * 100}%`
  const items: { key: string; n: number; cls: string }[] = [
    { key: 'done', n: ok, cls: styles.done },
    { key: 'failed', n: progress.failed, cls: styles.failed },
    { key: 'cancelled', n: cancelled, cls: styles.cancelled },
    { key: 'running', n: progress.running, cls: styles.running },
  ]
  const title = `${progress.done}/${progress.total} · ${t('suites.suiteRuns.progress.failed')}: ${progress.failed} · ${t('suites.suiteRuns.progress.running')}: ${progress.running} · ${t('suites.suiteRuns.progress.pending')}: ${progress.pending}`
  return (
    <div className={styles.wrap}>
      <Tooltip content={title}>
        <div
          className={cx(styles.bar, size === 'lg' && styles.barLg)}
          role="progressbar"
          aria-valuenow={progress.done}
          aria-valuemax={progress.total}
          aria-label={title}
        >
          {items.map((it) =>
            it.n > 0 ? (
              <div key={it.key} className={cx(styles.seg, it.cls)} style={{ width: pct(it.n) }} />
            ) : null
          )}
        </div>
      </Tooltip>
      {legend && (
        <div className={styles.legend}>
          <span>
            <span className={cx(styles.dot, styles.done)} />
            {ok} {t('suites.suiteRuns.progress.done')}
          </span>
          <span>
            <span className={cx(styles.dot, styles.failed)} />
            {progress.failed} {t('suites.suiteRuns.progress.failed')}
          </span>
          <span>
            <span className={cx(styles.dot, styles.running)} />
            {progress.running} {t('suites.suiteRuns.progress.running')}
          </span>
          <span>
            <span className={styles.dot} style={{ border: '1px solid currentColor' }} />
            {progress.pending} {t('suites.suiteRuns.progress.pending')}
          </span>
          {cancelled > 0 && (
            <span>
              <span className={cx(styles.dot, styles.cancelled)} />
              {cancelled} {t('suites.suiteRuns.progress.cancelled')}
            </span>
          )}
        </div>
      )}
    </div>
  )
}

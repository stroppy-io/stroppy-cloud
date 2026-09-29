import { testQueries } from '@api/queries/library'
import { providerQueries } from '@api/queries/settings'
import type { Schemas } from '@api/types'
import { AppLink } from '@app/AppLink'
import { SectionTitle } from '@app/PageHeader'
import { KeyValueList } from '@components/KeyValueList'
import { RelativeTime } from '@components/RelativeTime'
import { StatusBadge } from '@components/StatusBadge'
import { TagsView } from '@components/TagsEditor'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Icon, Stack, Text, useStyles2 } from '@grafana/ui'
import { formatRoleSizes } from '@helpers/sizes'
import { formatDateTime } from '@helpers/time'
import { useTenant } from '@hooks/useTenant'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import type { SuiteTab } from './SuiteDetailPage'

type Suite = Schemas['Suite']

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) 300px',
    gap: theme.spacing(3),
    marginTop: theme.spacing(2),
    [theme.breakpoints.down('md')]: { gridTemplateColumns: '1fr' },
  }),
  stats: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(4, minmax(0, 1fr))',
    gap: theme.spacing(1),
    [theme.breakpoints.down('sm')]: { gridTemplateColumns: 'repeat(2, 1fr)' },
  }),
  stat: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(1.5),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
  }),
  statLabel: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    textTransform: 'uppercase',
    letterSpacing: 0.4,
  }),
  statValue: css({
    fontSize: theme.typography.h3.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  section: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  testRow: css({
    display: 'grid',
    gridTemplateColumns: '20px minmax(0, 1fr) auto auto',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    padding: theme.spacing(1, 1.5),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
  }),
  muted: css({ color: theme.colors.text.secondary, fontSize: theme.typography.bodySmall.fontSize }),
  desc: css({ whiteSpace: 'pre-wrap', margin: 0 }),
})

export function SuiteOverviewTab({
  suite,
  onEdit,
  onGoTab,
}: {
  suite: Suite
  onEdit: () => void
  onGoTab: (tab: SuiteTab) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug, can } = useTenant()
  const navigate = useNavigate()
  const tests = useQuery(testQueries.options(slug))
  const providers = useQuery(providerQueries.list(slug))
  const refs = (suite.tests ?? []).flatMap((x) => ('ref' in x ? [x.ref] : []))
  const testIds = refs.map((r) => r.id)
  const refName = new Map(refs.map((r) => [r.id, r.name]))
  const byId = new Map((tests.data?.data ?? []).map((x) => [x.id, x]))
  const providerName = (id: string) => providers.data?.data.find((p) => p.id === id)?.name ?? id
  const axes = suite.axes ?? {}
  const lr = suite.summary?.last_run

  return (
    <div className={styles.grid}>
      <Stack direction="column" gap={3}>
        <div className={styles.stats}>
          <div className={styles.stat}>
            <span className={styles.statLabel}>{t('suites.overview.stats.cells')}</span>
            <span className={styles.statValue}>{suite.summary?.cell_count ?? 0}</span>
          </div>
          <div className={styles.stat}>
            <span className={styles.statLabel}>{t('suites.overview.stats.enabled')}</span>
            <span className={styles.statValue}>{suite.summary?.enabled_cell_count ?? 0}</span>
          </div>
          <div className={styles.stat}>
            <span className={styles.statLabel}>{t('suites.overview.stats.runs')}</span>
            <span className={styles.statValue}>{suite.summary?.run_count ?? 0}</span>
          </div>
          <div className={styles.stat}>
            <span className={styles.statLabel}>{t('suites.overview.stats.lastRun')}</span>
            {lr ? (
              <Stack alignItems="center" gap={1}>
                <StatusBadge status={lr.status} />
                <AppLink to="/t/$slug/suite-runs/$id" params={{ slug, id: lr.id }}>
                  <RelativeTime value={lr.started_at} />
                </AppLink>
              </Stack>
            ) : (
              <span className={styles.muted}>{t('suites.noRuns')}</span>
            )}
          </div>
        </div>

        <section className={styles.section}>
          <SectionTitle
            right={
              can('edit-library') ? (
                <Button size="sm" variant="secondary" fill="text" icon="edit" onClick={onEdit}>
                  {t('common.actions.edit')}
                </Button>
              ) : undefined
            }
          >
            {t('suites.overview.description')}
          </SectionTitle>
          {suite.description ? (
            <p className={styles.desc}>{suite.description}</p>
          ) : (
            <span className={styles.muted}>{t('suites.overview.noDescription')}</span>
          )}
        </section>

        <section className={styles.section}>
          <SectionTitle
            right={
              <Button
                size="sm"
                variant="secondary"
                fill="text"
                icon="sliders-v-alt"
                onClick={() => onGoTab('axes')}
              >
                {t('suites.tabs.axes')}
              </Button>
            }
          >
            {t('suites.overview.tests')} · {testIds.length}
          </SectionTitle>
          {testIds.length === 0 && (
            <span className={styles.muted}>{t('suites.overview.noTests')}</span>
          )}
          {testIds.map((id) => {
            const x = byId.get(id)
            return (
              <div key={id} className={styles.testRow}>
                <Icon name="vial" />
                <div style={{ minWidth: 0 }}>
                  <AppLink to="/t/$slug/library/tests/$id" params={{ slug, id }} plain>
                    <Text weight="medium">{x?.name ?? refName.get(id) ?? id}</Text>
                  </AppLink>
                  <div className={styles.muted}>
                    {[
                      x?.summary?.db_kind,
                      x?.summary?.db_version,
                      x?.summary?.topology_label,
                      formatRoleSizes(x?.sizes),
                    ]
                      .filter(Boolean)
                      .join(' · ')}
                  </div>
                </div>
                {/* Status and run count come from the test list; until it loads show nothing
                    rather than «0 запусков · Неизвестно». */}
                {x && (
                  <>
                    <span className={styles.muted}>
                      {t('suites.overview.testRuns', { count: x.summary?.run_count ?? 0 })}
                    </span>
                    <StatusBadge status={x.status} />
                  </>
                )}
              </div>
            )
          })}
        </section>

        <section className={styles.section}>
          <SectionTitle>{t('suites.overview.axes')}</SectionTitle>
          <KeyValueList
            items={[
              {
                label: t('suites.axes.providers'),
                value: axes.provider_profiles?.length
                  ? axes.provider_profiles.map(providerName).join(', ')
                  : t('suites.cells.axis.none'),
              },
              {
                label: t('suites.axes.sizes'),
                value: axes.sizes?.length ? (
                  <Stack direction="column" gap={0}>
                    {axes.sizes.map((s, i) => (
                      <span key={i}>{formatRoleSizes(s)}</span>
                    ))}
                  </Stack>
                ) : (
                  t('suites.cells.axis.none')
                ),
              },
              {
                label: t('suites.axes.versions'),
                value: axes.database_versions?.length
                  ? axes.database_versions.join(', ')
                  : t('suites.cells.axis.none'),
              },
              {
                label: t('suites.axes.variants'),
                value: axes.workload_variants?.length
                  ? axes.workload_variants
                      .map(
                        (v) =>
                          `${v.name ?? '?'}${v.vus ? ` · ${v.vus} vu` : ''}${v.scale_factor ? ` · sf ${v.scale_factor}` : ''}${v.duration ? ` · ${v.duration}` : ''}`
                      )
                      .join('; ')
                  : t('suites.cells.axis.none'),
              },
            ]}
          />
        </section>
      </Stack>

      <Stack direction="column" gap={3}>
        <KeyValueList
          title={t('suites.overview.about')}
          items={[
            { label: t('common.fields.author'), value: suite.author.display_name },
            { label: t('common.fields.created'), value: formatDateTime(suite.created_at) },
            { label: t('common.fields.updated'), value: <RelativeTime value={suite.updated_at} /> },
            {
              label: t('common.fields.tags'),
              value: Object.keys(suite.tags ?? {}).length ? <TagsView value={suite.tags} /> : '—',
            },
          ]}
        />
        <KeyValueList
          title={t('suites.overview.defaults')}
          items={[
            { label: t('suites.overview.concurrency'), value: suite.concurrency },
            {
              label: t('suites.overview.keep'),
              value:
                suite.defaults?.keep && suite.defaults.keep !== '0s'
                  ? suite.defaults.keep
                  : t('suites.overview.keepNone'),
            },
            {
              label: t('suites.overview.rating'),
              value:
                suite.defaults?.rating?.tenant || suite.defaults?.rating?.global
                  ? [
                      suite.defaults.rating.tenant && t('suites.form.ratingTenant'),
                      suite.defaults.rating.global && t('suites.form.ratingGlobal'),
                    ]
                      .filter(Boolean)
                      .join(', ')
                  : t('suites.overview.ratingNone'),
            },
          ]}
        />
        <section className={styles.section}>
          <KeyValueList
            title={t('suites.overview.schedules')}
            items={
              suite.summary?.schedules?.length
                ? suite.summary.schedules.map((s) => ({
                    label: <Icon name="clock-nine" />,
                    value: (
                      <AppLink to="/t/$slug/schedules/$id" params={{ slug, id: s.id }}>
                        {s.name ?? s.id}
                      </AppLink>
                    ),
                  }))
                : [{ label: '—', value: t('suites.overview.noSchedules') }]
            }
          />
          <Button
            size="sm"
            variant="secondary"
            icon="plus"
            disabled={!can('edit-library')}
            onClick={() =>
              void navigate({
                to: '/t/$slug/schedules/new',
                params: { slug },
                search: { kind: 'suite', target: suite.id } as never,
              })
            }
          >
            {t('suites.overview.addSchedule')}
          </Button>
        </section>
      </Stack>
    </div>
  )
}

import { testQueries } from '@api/queries/library'
import type { TenantDashboard } from '@api/types'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, CollapsableSection, Icon, Text, TextLink, useStyles2 } from '@grafana/ui'
import { useOnboarding } from '@hooks/useOnboarding'
import { getPublicConfig } from '@lib/config'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  wrap: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(3),
    maxWidth: 820,
    margin: `${theme.spacing(2)} auto`,
  }),
  head: css({
    display: 'flex',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
  }),
  progress: css({
    height: 4,
    borderRadius: 2,
    background: theme.colors.background.secondary,
    overflow: 'hidden',
    margin: theme.spacing(2, 0),
    div: { height: '100%', background: theme.colors.success.main },
  }),
  steps: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  step: css({
    display: 'grid',
    gridTemplateColumns: '32px minmax(0, 1fr) auto',
    gap: theme.spacing(2),
    alignItems: 'start',
  }),
  num: css({
    width: 32,
    height: 32,
    borderRadius: '50%',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    border: `1px solid ${theme.colors.border.medium}`,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.secondary,
  }),
  current: css({ borderColor: theme.colors.primary.border, color: theme.colors.primary.text }),
  done: css({
    background: theme.colors.success.transparent,
    borderColor: theme.colors.success.border,
    color: theme.colors.success.text,
  }),
  hint: css({ color: theme.colors.text.secondary, marginTop: theme.spacing(0.25) }),
  actions: css({
    display: 'flex',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    justifyContent: 'flex-end',
  }),
  guide: css({
    marginTop: theme.spacing(1),
    ol: { paddingLeft: theme.spacing(2.5), color: theme.colors.text.secondary },
    li: { marginBottom: theme.spacing(0.5) },
  }),
  foot: css({ marginTop: theme.spacing(2), color: theme.colors.text.secondary }),
})

type StepState = 'done' | 'current' | 'todo'

function Step({
  n,
  state,
  title,
  hint,
  actions,
  children,
}: {
  n: number
  state: StepState
  title: string
  hint: string
  actions?: ReactNode
  children?: ReactNode
}) {
  const styles = useStyles2(getStyles)
  return (
    <div className={styles.step}>
      <span
        className={cx(
          styles.num,
          state === 'done' && styles.done,
          state === 'current' && styles.current
        )}
      >
        {state === 'done' ? <Icon name="check" /> : n}
      </span>
      <div>
        <Text weight="medium" color={state === 'done' ? 'secondary' : undefined}>
          {title}
        </Text>
        <div className={styles.hint}>{hint}</div>
        {state !== 'done' && children}
      </div>
      <div className={styles.actions}>{state !== 'done' && actions}</div>
    </div>
  )
}

/** Whether the first-run checklist still has something to say for this tenant. */
export function checklistOpen(data: TenantDashboard, hidden: boolean): boolean {
  return !hidden && (data.run_counts.completed ?? 0) === 0
}

/**
 * The way to the first result in a fresh tenant: tenant → cloud provider →
 * test → launch → results. Each step ticks itself off from the tenant's
 * data; the checklist can be hidden for good.
 */
export function Onboarding({
  slug,
  tenantName,
  data,
}: {
  slug: string
  tenantName: string
  data: TenantDashboard
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const navigate = useNavigate()
  const onboarding = useOnboarding()
  const tests = useQuery(testQueries.options(slug))
  const demo = getPublicConfig()?.demo_report_url

  const hasProvider = (data.providers ?? []).some((p) => p.status === 'ready')
  const hasTest = (tests.data?.data.length ?? 0) > 0
  const runs = data.run_counts.total ?? 0
  const completed = data.run_counts.completed ?? 0
  const lastDone = data.recent_runs.find((r) => r.status === 'completed')
  const done = [true, hasProvider, hasTest, runs > 0, completed > 0]
  const current = done.indexOf(false)
  const state = (i: number): StepState => (done[i] ? 'done' : i === current ? 'current' : 'todo')
  const count = done.filter(Boolean).length

  return (
    <div className={styles.wrap}>
      <div className={styles.head}>
        <div>
          <Text element="h2" variant="h3">
            {t('dashboard.onboarding.title', { tenant: tenantName })}
          </Text>
          <div className={styles.hint}>{t('dashboard.onboarding.subtitle')}</div>
        </div>
        <Button
          size="sm"
          variant="secondary"
          fill="text"
          icon="times"
          onClick={() => void onboarding.set({ checklist_hidden: true })}
        >
          {t('dashboard.onboarding.hide')}
        </Button>
      </div>
      <div
        className={styles.progress}
        role="progressbar"
        aria-valuenow={count}
        aria-valuemin={0}
        aria-valuemax={done.length}
        aria-valuetext={t('dashboard.onboarding.progress', { done: count, total: done.length })}
      >
        <div style={{ width: `${(count / done.length) * 100}%` }} />
      </div>
      <div className={styles.steps}>
        <Step
          n={1}
          state={state(0)}
          title={t('dashboard.onboarding.tenant.title')}
          hint={t('dashboard.onboarding.tenant.hint')}
        />
        <Step
          n={2}
          state={state(1)}
          title={t('dashboard.onboarding.provider.title')}
          hint={t('dashboard.onboarding.provider.hint')}
          actions={
            <Button
              size="sm"
              icon="cloud"
              variant={state(1) === 'current' ? 'primary' : 'secondary'}
              onClick={() => void navigate({ to: '/t/$slug/settings/providers', params: { slug } })}
            >
              {t('dashboard.onboarding.provider.cta')}
            </Button>
          }
        >
          <div className={styles.guide}>
            <CollapsableSection
              label={
                <Text variant="bodySmall" weight="medium">
                  {t('dashboard.onboarding.provider.guideYc')}
                </Text>
              }
              isOpen={false}
            >
              <ol>
                <li>{t('dashboard.onboarding.provider.yc1')}</li>
                <li>{t('dashboard.onboarding.provider.yc2')}</li>
                <li>{t('dashboard.onboarding.provider.yc3')}</li>
                <li>{t('dashboard.onboarding.provider.yc4')}</li>
              </ol>
            </CollapsableSection>
            <CollapsableSection
              label={
                <Text variant="bodySmall" weight="medium">
                  {t('dashboard.onboarding.provider.guideAws')}
                </Text>
              }
              isOpen={false}
            >
              <ol>
                <li>{t('dashboard.onboarding.provider.aws1')}</li>
                <li>{t('dashboard.onboarding.provider.aws2')}</li>
                <li>{t('dashboard.onboarding.provider.aws3')}</li>
              </ol>
            </CollapsableSection>
          </div>
        </Step>
        <Step
          n={3}
          state={state(2)}
          title={t('dashboard.onboarding.test.title')}
          hint={t('dashboard.onboarding.test.hint')}
          actions={
            <>
              <Button
                size="sm"
                icon="book-open"
                variant={state(2) === 'current' ? 'primary' : 'secondary'}
                onClick={() => void navigate({ to: '/examples', search: { kind: 'test' } })}
              >
                {t('dashboard.onboarding.test.example')}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                icon="plus"
                onClick={() =>
                  void navigate({ to: '/t/$slug/library/tests/new', params: { slug } })
                }
              >
                {t('dashboard.onboarding.test.cta')}
              </Button>
            </>
          }
        />
        <Step
          n={4}
          state={state(3)}
          title={t('dashboard.onboarding.launch.title')}
          hint={t('dashboard.onboarding.launch.hint')}
          actions={
            <Button
              size="sm"
              icon="play"
              variant={state(3) === 'current' ? 'primary' : 'secondary'}
              onClick={() => void navigate({ to: '/t/$slug/library/tests', params: { slug } })}
            >
              {t('dashboard.onboarding.launch.cta')}
            </Button>
          }
        />
        <Step
          n={5}
          state={state(4)}
          title={t('dashboard.onboarding.results.title')}
          hint={t('dashboard.onboarding.results.hint')}
          actions={
            <Button
              size="sm"
              icon="chart-line"
              variant={state(4) === 'current' ? 'primary' : 'secondary'}
              onClick={() =>
                lastDone
                  ? void navigate({
                      to: '/t/$slug/runs/$id',
                      params: { slug, id: lastDone.id },
                    })
                  : void navigate({ to: '/t/$slug/runs', params: { slug } })
              }
            >
              {t('dashboard.onboarding.results.cta')}
            </Button>
          }
        />
      </div>
      {demo && (
        <div className={styles.foot}>
          {t('dashboard.onboarding.demoLead')}{' '}
          <TextLink href={demo} external inline>
            {t('dashboard.onboarding.demo')}
          </TextLink>
        </div>
      )}
    </div>
  )
}

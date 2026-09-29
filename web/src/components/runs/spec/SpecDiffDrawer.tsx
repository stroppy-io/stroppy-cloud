import { isApiError } from '@api/errors'
import { testQueries } from '@api/queries/library'
import type { Run } from '@api/types'
import { AppLink } from '@app/AppLink'
import { ErrorState } from '@app/ErrorState'
import { SpecChangesTable } from '@components/runs/compare/SpecChangesTable'
import { Alert, Drawer, LoadingPlaceholder, Stack, Text } from '@grafana/ui'
import { diffValues, type SpecChange } from '@helpers/spec-diff'
import { useTenant } from '@hooks/useTenant'
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

// Snapshot (what actually ran) vs the current Test definition in the library.
export function SpecDiffDrawer({ run, onClose }: { run: Run; onClose: () => void }) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const test = useQuery(testQueries.detail(slug, run.test_ref.id))
  const changes = useMemo<SpecChange[] | undefined>(() => {
    const tst = test.data
    if (!tst) return undefined
    const db =
      tst.resolved?.database ??
      (tst.database && 'inline' in tst.database ? tst.database.inline : undefined)
    const wl =
      tst.resolved?.workload ??
      (tst.workload && 'inline' in tst.workload ? tst.workload.inline : undefined)
    const out: SpecChange[] = []
    if (db) {
      out.push(...diffValues(run.snapshot.database.kind, db.kind, 'database.kind'))
      out.push(...diffValues(run.snapshot.database.version, db.version, 'database.version'))
      out.push(...diffValues(run.snapshot.database.params, db.params, 'database.params'))
      out.push(
        ...diffValues(run.snapshot.database.configs ?? {}, db.configs ?? {}, 'database.configs')
      )
    } else if (tst.database && 'ref' in tst.database && tst.database.ref.name) {
      out.push({
        path: 'database',
        op: 'replace',
        a: run.snapshot.database_name,
        b: tst.database.ref.name,
      })
    }
    if (wl) {
      out.push(
        ...diffValues(
          run.snapshot.workload.stroppy_version,
          wl.stroppy_version,
          'workload.stroppy_version'
        )
      )
      out.push(...diffValues(run.snapshot.workload.protocol, wl.protocol, 'workload.protocol'))
      out.push(...diffValues(run.snapshot.workload.segments, wl.segments, 'workload.segments'))
      out.push(
        ...diffValues(run.snapshot.workload.options ?? {}, wl.options ?? {}, 'workload.options')
      )
    }
    out.push(...diffValues(run.snapshot.sizes, tst.sizes ?? {}, 'sizes'))
    out.push(
      ...diffValues(
        run.snapshot.provider_profile.id,
        tst.provider_profile_id ?? undefined,
        'provider_profile'
      )
    )
    out.push(...diffValues(run.snapshot.keep ?? '0s', tst.keep ?? '0s', 'keep'))
    return out
  }, [test.data, run])

  return (
    <Drawer
      title={t('runs.spec.diff.title')}
      subtitle={
        run.test_ref.name
          ? t('runs.spec.diff.subtitle', { name: run.test_ref.name })
          : t('runs.spec.diff.subtitleNoName')
      }
      size="md"
      onClose={onClose}
    >
      {test.isPending ? (
        <LoadingPlaceholder text={t('common.misc.loading')} />
      ) : test.isError ? (
        isApiError(test.error) && test.error.status === 404 ? (
          <Alert severity="info" title={t('runs.spec.diff.testGone')}>
            {t('runs.spec.diff.testGoneHint')}
          </Alert>
        ) : (
          <ErrorState error={test.error} onRetry={() => void test.refetch()} compact />
        )
      ) : (
        <Stack direction="column" gap={2}>
          <Text color="secondary">
            {t('runs.spec.diff.legend')}{' '}
            <AppLink to="/t/$slug/library/tests/$id" params={{ slug, id: run.test_ref.id }}>
              {test.data.name}
            </AppLink>
          </Text>
          {changes && changes.length === 0 ? (
            <Alert severity="success" title={t('runs.spec.diff.same')}>
              {t('runs.spec.diff.sameHint')}
            </Alert>
          ) : (
            <SpecChangesTable
              changes={changes ?? []}
              aLabel={t('runs.spec.diff.snapshot')}
              bLabel={t('runs.spec.diff.test')}
            />
          )}
        </Stack>
      )}
    </Drawer>
  )
}

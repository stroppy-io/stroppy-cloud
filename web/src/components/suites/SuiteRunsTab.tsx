import { suiteQueries } from '@api/queries/suites'
import type { Schemas } from '@api/types'
import { Box } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useQuery } from '@tanstack/react-query'
import { SuiteRunsTable } from './SuiteRunsTable'

export function SuiteRunsTab({ suite }: { suite: Schemas['Suite'] }) {
  const { slug } = useTenant()
  const runs = useQuery({
    ...suiteQueries.runsOf(slug, suite.id),
    // running suite runs move; poll lightly while any is active
    refetchInterval: (q) =>
      q.state.data?.data.some((r) => r.status === 'running' || r.status === 'cancelling')
        ? 3000
        : false,
  })
  return (
    <Box marginTop={2}>
      <SuiteRunsTable
        rows={runs.data?.data ?? []}
        loading={runs.isPending}
        error={runs.isError ? runs.error : undefined}
        onRetry={() => void runs.refetch()}
      />
    </Box>
  )
}

import { workloadQueries } from '@api/queries/library'
import {
  toWorkloadListQuery,
  WORKLOAD_LIST_DEFAULTS,
  WorkloadListPage,
  workloadListSearchSchema,
} from '@components/library/workloads/WorkloadListPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/library/workloads/')({
  staticData: { crumb: 'nav.workloads' },
  validateSearch: workloadListSearchSchema,
  search: { middlewares: [stripSearchParams(WORKLOAD_LIST_DEFAULTS)] },
  loaderDeps: ({ search }) => ({ search }),
  loader: ({ context, params, deps }) =>
    context.queryClient.ensureInfiniteQueryData(
      workloadQueries.list(params.slug, toWorkloadListQuery(deps.search))
    ),
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <WorkloadListPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})

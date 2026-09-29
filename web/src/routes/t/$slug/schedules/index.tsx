import { scheduleQueries } from '@api/queries/suites'
import {
  SCHEDULE_LIST_DEFAULTS,
  ScheduleListPage,
  scheduleListSearchSchema,
  toScheduleListQuery,
} from '@components/schedules/ScheduleListPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/schedules/')({
  validateSearch: scheduleListSearchSchema,
  search: { middlewares: [stripSearchParams(SCHEDULE_LIST_DEFAULTS)] },
  loaderDeps: ({ search }) => ({ search }),
  loader: ({ context, params, deps }) =>
    context.queryClient.ensureInfiniteQueryData(
      scheduleQueries.list(params.slug, toScheduleListQuery(deps.search))
    ),
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <ScheduleListPage
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})

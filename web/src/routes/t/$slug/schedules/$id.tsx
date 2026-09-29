import { scheduleQueries } from '@api/queries/suites'
import { SCHEDULE_TABS, ScheduleDetailPage } from '@components/schedules/ScheduleDetailPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { z } from 'zod'

const searchSchema = z.object({
  tab: z.enum(SCHEDULE_TABS).default('settings').catch('settings'),
})

export const Route = createFileRoute('/t/$slug/schedules/$id')({
  validateSearch: searchSchema,
  search: { middlewares: [stripSearchParams({ tab: 'settings' })] },
  loader: async ({ context, params }) => {
    const entity = await context.queryClient.ensureQueryData(
      scheduleQueries.detail(params.slug, params.id)
    )
    return { crumb: entity.name }
  },
  component: () => {
    const { id } = Route.useParams()
    const { tab } = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <ScheduleDetailPage
        id={id}
        tab={tab}
        onTabChange={(next) => void navigate({ search: (prev) => ({ ...prev, tab: next }) })}
      />
    )
  },
})

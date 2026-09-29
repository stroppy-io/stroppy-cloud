import { ScheduleCreatePage } from '@components/schedules/ScheduleCreatePage'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

// Pre-fills the target when opened from a test or suite page.
const searchSchema = z.object({
  kind: z.enum(['test', 'suite']).optional().catch(undefined),
  target: z.string().optional().catch(undefined),
})

export const Route = createFileRoute('/t/$slug/schedules/new')({
  staticData: { crumb: 'schedules.new' },
  validateSearch: searchSchema,
  component: () => {
    const search = Route.useSearch()
    return <ScheduleCreatePage kind={search.kind} target={search.target} />
  },
})

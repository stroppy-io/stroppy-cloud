import { suiteQueries } from '@api/queries/suites'
import { SUITE_TABS, SuiteDetailPage } from '@components/suites/SuiteDetailPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { z } from 'zod'

const searchSchema = z.object({
  tab: z.enum(SUITE_TABS).default('overview').catch('overview'),
})

export const Route = createFileRoute('/t/$slug/suites/$id')({
  validateSearch: searchSchema,
  search: { middlewares: [stripSearchParams({ tab: 'overview' })] },
  loader: async ({ context, params }) => {
    const entity = await context.queryClient.ensureQueryData(
      suiteQueries.detail(params.slug, params.id)
    )
    return { crumb: entity.name }
  },
  component: () => {
    const { id } = Route.useParams()
    const { tab } = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <SuiteDetailPage
        id={id}
        tab={tab}
        onTabChange={(next) => void navigate({ search: (prev) => ({ ...prev, tab: next }) })}
      />
    )
  },
})

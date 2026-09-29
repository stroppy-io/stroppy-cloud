import { suiteRunQueries } from '@api/queries/suites'
import { SUITE_RUN_TABS, SuiteRunPage } from '@components/suites/SuiteRunPage'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { z } from 'zod'

const searchSchema = z.object({
  tab: z.enum(SUITE_RUN_TABS).default('summary').catch('summary'),
})

export const Route = createFileRoute('/t/$slug/suite-runs/$id')({
  validateSearch: searchSchema,
  search: { middlewares: [stripSearchParams({ tab: 'summary' })] },
  loader: async ({ context, params }) => {
    const entity = await context.queryClient.ensureQueryData(
      suiteRunQueries.detail(params.slug, params.id)
    )
    return { crumb: entity.name }
  },
  component: () => {
    const { id } = Route.useParams()
    const { tab } = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <SuiteRunPage
        id={id}
        tab={tab}
        onTabChange={(next) => void navigate({ search: (prev) => ({ ...prev, tab: next }) })}
      />
    )
  },
})

import { TestWizard, testWizardSearchSchema } from '@components/library/tests/TestWizard'
import { createFileRoute, stripSearchParams } from '@tanstack/react-router'

// Wizard: the draft is created on the server at step 1 and lives in `?id=`; steps in `?step=`.
export const Route = createFileRoute('/t/$slug/library/tests/new')({
  staticData: { crumb: 'library.tests.new' },
  validateSearch: testWizardSearchSchema,
  search: { middlewares: [stripSearchParams({ step: 1 })] },
  component: () => {
    const search = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <TestWizard
        search={search}
        onSearchChange={(next) =>
          void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
        }
      />
    )
  },
})

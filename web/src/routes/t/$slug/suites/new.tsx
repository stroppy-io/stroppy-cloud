import { SuiteCreatePage } from '@components/suites/SuiteCreatePage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/t/$slug/suites/new')({
  staticData: { crumb: 'suites.new' },
  component: SuiteCreatePage,
})

import { createFileRoute, Outlet } from '@tanstack/react-router'

// Layout route: contributes the section breadcrumb for every child page.
export const Route = createFileRoute('/t/$slug/suites')({
  staticData: { crumb: 'nav.suites' },
  component: Outlet,
})

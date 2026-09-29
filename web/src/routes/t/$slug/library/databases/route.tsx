import { createFileRoute, Outlet } from '@tanstack/react-router'

// Layout route: contributes the section breadcrumb for every child page.
export const Route = createFileRoute('/t/$slug/library/databases')({
  staticData: { crumb: 'nav.databases' },
  component: Outlet,
})

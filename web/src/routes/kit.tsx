import { KitPage } from '@components/kit/KitPage'
import { createFileRoute, notFound } from '@tanstack/react-router'

// The UI kit showcase: tokens, the Grafana theme made from them, plain HTML
// in base.css. A development tool; production builds answer 404.
export const Route = createFileRoute('/kit')({
  staticData: { bare: true },
  beforeLoad: () => {
    if (!import.meta.env.DEV) throw notFound()
  },
  component: KitPage,
})

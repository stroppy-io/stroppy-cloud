import { ErrorState } from '@app/ErrorState'
import { Page } from '@app/Page'
import { Button, EmptyState, ErrorBoundary, Icon, LoadingPlaceholder } from '@grafana/ui'
import { getAccessToken } from '@lib/auth'
import { loadPublicConfig } from '@lib/config'
import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Outlet, redirect, useNavigate } from '@tanstack/react-router'
import { lazy } from 'react'
import { useTranslation } from 'react-i18next'

export interface RouterContext {
  queryClient: QueryClient
}

const Devtools = import.meta.env.DEV
  ? lazy(() =>
      Promise.all([
        import('@tanstack/react-devtools'),
        import('@tanstack/react-query-devtools'),
        import('@tanstack/react-router-devtools'),
      ]).then(([dev, q, r]) => ({
        default: () => (
          <dev.TanStackDevtools
            plugins={[
              { name: 'Query', render: <q.ReactQueryDevtoolsPanel /> },
              { name: 'Router', render: <r.TanStackRouterDevtoolsPanel /> },
            ]}
          />
        ),
      }))
    )
  : () => null

function Pending() {
  const { t } = useTranslation()
  return (
    <Page>
      <LoadingPlaceholder text={t('common.misc.loading')} />
    </Page>
  )
}

function NotFound() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  return (
    <Page>
      <EmptyState
        variant="not-found"
        image={<Icon name="question-circle" size="xxxl" />}
        message={t('common.errors.pageNotFound')}
        button={
          <Button onClick={() => void navigate({ to: '/' })}>{t('common.errors.goHome')}</Button>
        }
      >
        {t('common.errors.pageNotFoundHint')}
      </EmptyState>
    </Page>
  )
}

// After a deploy the old page still references chunks that no longer exist; one reload fixes it.
function isStaleChunkError(error: unknown): boolean {
  const msg = error instanceof Error ? error.message : String(error)
  return /dynamically imported module|Importing a module script failed|Loading chunk/.test(msg)
}

function ChunkReload({ error, reset }: { error: unknown; reset: () => void }) {
  if (isStaleChunkError(error)) {
    const key = 'stroppy.chunk-reload'
    let reloaded = false
    try {
      reloaded = sessionStorage.getItem(key) === window.location.href
      if (!reloaded) sessionStorage.setItem(key, window.location.href)
    } catch {
      // ignore
    }
    if (!reloaded) {
      window.location.reload()
      return null
    }
  }
  return (
    <Page>
      <ErrorState error={error} onRetry={reset} />
    </Page>
  )
}

// Everything outside these prefixes needs a Kratos session (whose cookie
// must exchange into a JWT) or the static dev token.
export const PUBLIC_PREFIXES = [
  '/land',
  '/how',
  '/kit',
  '/login',
  '/register',
  '/verify',
  '/recovery',
  '/auth/',
  '/s/',
  '/leaderboard',
  '/embed/',
]

/** Pages a guest reads without signing in (`/` included: the landing). */
export function isPublicPath(pathname: string): boolean {
  return pathname === '/' || PUBLIC_PREFIXES.some((p) => pathname.startsWith(p))
}

export const Route = createRootRouteWithContext<RouterContext>()({
  beforeLoad: async ({ location }) => {
    // `/` is public too: the landing for guests, the app for the signed in.
    if (isPublicPath(location.pathname)) return
    const cfg = await loadPublicConfig()
    if (cfg.auth.mode === 'kratos' && !(await getAccessToken())) {
      throw redirect({ to: '/login', search: { next: location.href } })
    }
  },
  component: () => (
    <ErrorBoundary>
      {({ error }) =>
        error ? (
          <Page>
            <ErrorState error={error} onRetry={() => window.location.reload()} />
          </Page>
        ) : (
          <>
            <Outlet />
            <Devtools />
          </>
        )
      }
    </ErrorBoundary>
  ),
  notFoundComponent: NotFound,
  errorComponent: ({ error, reset }) => <ChunkReload error={error} reset={reset} />,
  pendingComponent: Pending,
  pendingMs: 250,
  pendingMinMs: 300,
})

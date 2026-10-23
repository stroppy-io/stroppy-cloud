import { ApiError } from '@api/errors'
import { Toaster } from '@app/Toaster'
import { ThemeModeContext } from '@app/theme-context'
import { css, Global } from '@emotion/react'
import { ThemeContext } from '@grafana/data'
import { GlobalStyles } from '@grafana/ui'
import { onAuthLost } from '@lib/auth'
import { isPublicPath } from '@routes/__root'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import '@lib/i18n'
import { routeTree } from './routeTree.gen'
import { getAppTheme, type ThemeMode } from './styles/theme'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 10_000,
      retry: (count, err) => !(err instanceof ApiError && err.status < 500) && count < 2,
      refetchOnWindowFocus: false,
    },
  },
})

export const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: 'intent',
  defaultPreloadStaleTime: 0,
  scrollRestoration: true,
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

// Grafana's GlobalStyles leave the UA body margin. The shell owns the viewport (AppShell scrolls
// its content column itself), so the document never scrolls in either direction.
const bodyReset = css({
  'html, body': { margin: 0, padding: 0, height: '100%', overflow: 'clip' },
  // @grafana/ui 13 without the visual-refresh flag pins MenuItem's label to text.primary, so a
  // disabled item greys only its (invisible) container and reads as enabled. Let the label
  // inherit the item's disabled colour.
  '[data-role="menuitem"][data-disabled] > div > span': { color: 'inherit' },
})

// The chosen theme is shared with stroppy.io: a cookie on the parent domain
// that both read (`stroppy-theme=dark|light`). Elsewhere (a local stand) the
// cookie stays on this host. localStorage keeps a choice made before the
// cookie existed. Nothing chosen → the system's scheme.
const THEME_COOKIE = 'stroppy-theme'
const THEME_KEY = 'stroppy.theme'
const SHARED_DOMAIN = 'stroppy.io'

function asMode(v: string | null | undefined): ThemeMode | undefined {
  return v === 'dark' || v === 'light' ? v : undefined
}

function readThemeCookie(): ThemeMode | undefined {
  const m = document.cookie.match(new RegExp(`(?:^|; )${THEME_COOKIE}=([^;]*)`))
  return asMode(m?.[1])
}

function writeThemeCookie(m: ThemeMode) {
  const host = window.location.hostname
  const shared = host === SHARED_DOMAIN || host.endsWith(`.${SHARED_DOMAIN}`)
  const domain = shared ? `; domain=.${SHARED_DOMAIN}` : ''
  const secure = window.location.protocol === 'https:' ? '; secure' : ''
  // biome-ignore lint/suspicious/noDocumentCookie: the Cookie Store API is not in every browser yet
  document.cookie = `${THEME_COOKIE}=${m}; path=/; max-age=31536000; samesite=lax${domain}${secure}`
}

function initialMode(): ThemeMode {
  const cookie = readThemeCookie()
  if (cookie) return cookie
  try {
    const stored = asMode(localStorage.getItem(THEME_KEY))
    if (stored) return stored
  } catch {
    // ignore
  }
  return window.matchMedia?.('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

export function App() {
  const [mode, setModeState] = useState<ThemeMode>(initialMode)
  const setMode = (m: ThemeMode) => {
    setModeState(m)
    writeThemeCookie(m)
    try {
      localStorage.setItem(THEME_KEY, m)
    } catch {
      // ignore
    }
  }
  const theme = getAppTheme(mode)
  // A 401 anywhere (expired session, revoked token) drops the cache and
  // returns to the login page — except on a public page (a shared report,
  // the rating, an embed): a guest there has nothing to sign in to, and one
  // stray call needing a session must not take the page away.
  useEffect(() => {
    onAuthLost(() => {
      queryClient.clear()
      if (isPublicPath(window.location.pathname)) return
      void router.navigate({ to: '/login' })
    })
  }, [])
  // @grafana/ui reads its strings through a plain `t()` (no subscription) and memoizes parts of
  // TimeRangePicker & co., so a language switch remounts the routed tree; router state lives in
  // the URL and data in the Query cache, so only transient local UI state is reset.
  const { i18n } = useTranslation()
  useEffect(() => {
    document.documentElement.style.colorScheme = mode
  }, [mode])
  return (
    <ThemeContext.Provider value={theme}>
      <GlobalStyles />
      <Global styles={bodyReset} />
      <ThemeModeContext.Provider value={{ mode, setMode }}>
        <QueryClientProvider client={queryClient}>
          <RouterProvider key={i18n.language} router={router} />
          <Toaster />
        </QueryClientProvider>
      </ThemeModeContext.Provider>
    </ThemeContext.Provider>
  )
}

import { useBlocker, useRouter } from '@tanstack/react-router'
import { type RefObject, useEffect, useRef } from 'react'

// The postMessage protocol of an embed (/embed/*) with the page that frames
// it — see web/AGENTS.md §6a. Embed → parent:
//   { type: 'stroppy-embed:ready' }                    once the data has drawn
//   { type: 'stroppy-embed:height', height: <int px> } whenever the height changes
// Parent → embed (only from an allowed origin):
//   { type: 'stroppy-embed:theme', theme: 'dark' | 'light' }

export type EmbedTheme = 'dark' | 'light'

/** Room an open dropdown needs below its field: Grafana's menu is at most 8.5 rows. */
export const EMBED_MENU_ROOM = 320

const post = (message: Record<string, unknown>) => {
  // Nothing secret travels: the embed shows public data only.
  if (window.parent !== window) window.parent.postMessage(message, '*')
}

/** The height the frame needs: the content, plus room under an open dropdown. */
export function embedHeight(root: HTMLElement): number {
  let bottom = root.offsetTop + root.offsetHeight
  for (const field of root.querySelectorAll('[aria-expanded="true"]')) {
    bottom = Math.max(
      bottom,
      field.getBoundingClientRect().bottom + window.scrollY + EMBED_MENU_ROOM
    )
  }
  return Math.ceil(bottom)
}

/** Whether a message is a theme command from an allowed page. */
export function themeCommand(
  event: Pick<MessageEvent, 'origin' | 'data'>,
  origins: readonly string[]
): EmbedTheme | undefined {
  if (!origins.includes(event.origin)) return undefined
  const data = event.data as { type?: unknown; theme?: unknown } | null
  if (data?.type !== 'stroppy-embed:theme') return undefined
  return data.theme === 'dark' || data.theme === 'light' ? data.theme : undefined
}

/**
 * Talks to the framing page: reports the root's height (rounded, only on a
 * change; an open dropdown counts), says ready once `ready` turns true, and
 * applies theme commands from `origins` (others are ignored).
 */
export function useEmbedBridge(
  root: RefObject<HTMLElement | null>,
  {
    ready,
    origins,
    onTheme,
  }: { ready: boolean; origins: readonly string[]; onTheme: (t: EmbedTheme) => void }
) {
  const sent = useRef(0)
  const announced = useRef(false)
  const themeRef = useRef(onTheme)
  themeRef.current = onTheme

  useEffect(() => {
    const el = root.current
    if (!el) return
    let frame = 0
    const measure = () => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        const height = embedHeight(el)
        if (height !== sent.current) {
          sent.current = height
          post({ type: 'stroppy-embed:height', height })
        }
      })
    }
    measure()
    const resize = new ResizeObserver(measure)
    resize.observe(el)
    // A dropdown opening or closing changes no size of the root.
    const mutations = new MutationObserver(measure)
    mutations.observe(el, {
      subtree: true,
      childList: true,
      attributes: true,
      attributeFilter: ['aria-expanded'],
    })
    return () => {
      cancelAnimationFrame(frame)
      resize.disconnect()
      mutations.disconnect()
    }
  }, [root])

  useEffect(() => {
    if (!ready || announced.current) return
    announced.current = true
    post({ type: 'stroppy-embed:ready' })
  }, [ready])

  useEffect(() => {
    const listen = (event: MessageEvent) => {
      if (event.source !== window.parent) return
      const theme = themeCommand(event, origins)
      if (theme) themeRef.current(theme)
    }
    window.addEventListener('message', listen)
    return () => window.removeEventListener('message', listen)
  }, [origins])
}

/** Whether a navigation leaves the embeds: then it belongs in a new tab, not in the frame. */
export function leavesEmbed(pathname: string): boolean {
  return !pathname.startsWith('/embed/')
}

/**
 * Keeps the frame on its embed: any navigation out of /embed/* (a row, a
 * link in an empty state, whatever is added later) opens in a new tab and
 * the frame stays — a report drawn inside the host page has no way back.
 * Rows and cell links open their tab directly (a click keeps the browser's
 * popup permission); this is the guard behind them.
 */
export function useEmbedNavigationGuard() {
  const router = useRouter()
  useBlocker({
    shouldBlockFn: ({ next }) => {
      if (!leavesEmbed(next.pathname)) return false
      const href = router.buildLocation({ to: next.pathname, search: next.search } as never).href
      window.open(href, '_blank', 'noopener')
      return true
    },
  })
}

import type { Problem } from '@api/types'
import type { MockStore } from './store'

export type Method = 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'

export interface HandlerCtx {
  params: Record<string, string>
  query: URLSearchParams
  body: unknown
  store: MockStore
  headers: Headers
}

export type HandlerResult =
  | {
      status?: number
      json?: unknown
      text?: string
      contentType?: string
      headers?: Record<string, string>
    }
  | { problem: Partial<Problem> & { status: number; code: string } }

export type Handler = (ctx: HandlerCtx) => HandlerResult | Promise<HandlerResult>

interface Route {
  method: Method
  pattern: string
  re: RegExp
  keys: string[]
  handler: Handler
}

const routes: Route[] = []

// Pattern syntax: `/api/v1/t/:slug/runs/:id:cancel` — `:name` segments are params;
// a trailing `:verb` (after the last `/segment`) is matched literally.
export function route(method: Method, pattern: string, handler: Handler): void {
  const keys: string[] = []
  const re = new RegExp(
    `^${pattern
      .split('/')
      .map((seg) => {
        // ":id:cancel" → param id + literal ":cancel"
        const m = seg.match(/^:([a-zA-Z_]+)(:[a-z-]+)?$/)
        if (m) {
          keys.push(m[1])
          return `([^/:]+)${m[2] ? escapeRe(m[2]) : ''}`
        }
        return escapeRe(seg)
      })
      .join('/')}$`
  )
  routes.push({ method, pattern, re, keys, handler })
}

function escapeRe(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export function problem(
  status: number,
  code: string,
  detail?: string,
  extra?: Partial<Problem>
): HandlerResult {
  return { problem: { status, code, detail, title: code, ...extra } }
}

export function notFound(what = 'resource'): HandlerResult {
  return problem(404, 'not_found', `${what} not found`)
}

export function noContent(): HandlerResult {
  return { status: 204 }
}

export function match(
  method: string,
  pathname: string
): { route: Route; params: Record<string, string> } | undefined {
  for (const r of routes) {
    if (r.method !== method) continue
    const m = pathname.match(r.re)
    if (!m) continue
    const params: Record<string, string> = {}
    r.keys.forEach((k, i) => {
      params[k] = decodeURIComponent(m[i + 1])
    })
    return { route: r, params }
  }
  return undefined
}

export function listRoutes(): string[] {
  return routes.map((r) => `${r.method} ${r.pattern}`)
}

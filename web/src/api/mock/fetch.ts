import { match } from './router'
import { getSimulation } from './simulation'
import { store } from './store'
import './handlers/index'

// Seeded running runs advance from the first request, not only once a WS topic is subscribed.
getSimulation(store)

const MIN_LATENCY = 60
const MAX_LATENCY = 220

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms))
}

function problemResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/problem+json' },
  })
}

// A drop-in `fetch` serving the OpenAPI contract from the in-memory store.
// `?__fail=<status>` on any request forces an error for UI error-state demos;
// `?__delay=<ms>` overrides latency.
export const mockFetch: typeof fetch = async (input, init) => {
  const req = input instanceof Request ? input : new Request(input, init)
  const url = new URL(req.url, window.location.origin)
  const method = req.method.toUpperCase()

  const forcedFail = url.searchParams.get('__fail')
  const delay = Number(
    url.searchParams.get('__delay') ?? MIN_LATENCY + Math.random() * (MAX_LATENCY - MIN_LATENCY)
  )
  await sleep(delay)

  if (forcedFail) {
    const status = Number(forcedFail) || 500
    return problemResponse(status, {
      type: 'about:blank',
      title: 'Forced failure',
      status,
      code: `forced_${status}`,
      detail: 'Requested via __fail',
    })
  }

  const hit = match(method, url.pathname)
  if (!hit) {
    console.warn('[mock] unhandled', method, url.pathname)
    return problemResponse(404, {
      type: 'about:blank',
      title: 'Not found',
      status: 404,
      code: 'mock_unhandled',
      detail: `${method} ${url.pathname} has no mock handler`,
    })
  }

  let body: unknown
  if (method !== 'GET' && method !== 'DELETE') {
    const text = await req.text()
    if (text) {
      try {
        body = JSON.parse(text)
      } catch {
        return problemResponse(400, {
          type: 'about:blank',
          title: 'Bad JSON',
          status: 400,
          code: 'bad_json',
        })
      }
    }
  }

  try {
    const result = await hit.route.handler({
      params: hit.params,
      query: url.searchParams,
      body,
      store,
      headers: req.headers,
    })
    if ('problem' in result) {
      const p = { type: 'about:blank', title: result.problem.code, ...result.problem }
      return problemResponse(p.status, p)
    }
    if (result.text !== undefined)
      return new Response(result.text, {
        status: result.status ?? 200,
        headers: { 'content-type': result.contentType ?? 'text/plain', ...result.headers },
      })
    const status = result.status ?? (result.json === undefined ? 204 : 200)
    if (result.json === undefined) return new Response(null, { status, headers: result.headers })
    return new Response(JSON.stringify(result.json), {
      status,
      headers: {
        'content-type': 'application/json',
        'x-stroppy-version': store.version,
        ...result.headers,
      },
    })
  } catch (e) {
    console.error('[mock] handler threw', method, url.pathname, e)
    return problemResponse(500, {
      type: 'about:blank',
      title: 'Mock handler error',
      status: 500,
      code: 'mock_error',
      detail: String(e),
    })
  }
}

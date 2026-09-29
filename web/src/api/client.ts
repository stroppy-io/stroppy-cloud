import { getToken, notifyAuthLost } from '@lib/auth'
import createClient, { type Middleware } from 'openapi-fetch'
import { toApiError } from './errors'
import { stringifyRequest } from './json'
import { apiMode } from './mode'
import type { Paths } from './types'

const IDEMPOTENT_ACTIONS =
  /(:launch|:run-now|:quick-run|:rerun|:retry-failed)$|\/runs$|\/suite-runs$/

const auth: Middleware = {
  async onRequest({ request }) {
    request.headers.set('Authorization', `Bearer ${getToken()}`)
    if (request.method === 'POST' && IDEMPOTENT_ACTIONS.test(new URL(request.url).pathname)) {
      request.headers.set('Idempotency-Key', crypto.randomUUID())
    }
    return request
  },
  async onResponse({ response }) {
    if (response.status === 401) notifyAuthLost()
    return response
  },
}

// One typed client for the whole app. The mock is a `fetch` implementation, so every
// query/mutation is written once against the OpenAPI types and never knows the mode.
// In mock mode the mock module is loaded lazily on first request so it never ships in the real bundle.
let mockFetchPromise: Promise<typeof fetch> | undefined
const lazyMockFetch: typeof fetch = (input, init) => {
  mockFetchPromise ??= import('./mock/fetch').then((m) => m.mockFetch)
  return mockFetchPromise.then((f) => f(input, init))
}

export const api = createClient<Paths>({
  baseUrl: '',
  fetch: apiMode === 'mock' ? lazyMockFetch : undefined,
  bodySerializer: (body) => stringifyRequest(body),
})
api.use(auth)

// Unwrap an openapi-fetch result into data or a thrown ApiError. Use in every queryFn/mutationFn.
export async function unwrap<T>(
  p: Promise<{ data?: T; error?: unknown; response: Response }>
): Promise<NonNullable<T>> {
  const { data, error, response } = await p
  if (error !== undefined || !response.ok) throw await toApiError(response)
  return data as NonNullable<T>
}

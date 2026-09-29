// The single switch between the real server and the in-memory mock API.
// Real by default; `VITE_API_MODE=mock` turns the mock on (never in a production build).
export type ApiMode = 'mock' | 'real'
export const apiMode: ApiMode =
  import.meta.env.VITE_API_MODE === 'mock' && import.meta.env.DEV ? 'mock' : 'real'

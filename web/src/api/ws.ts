import { getAccessToken } from '@lib/auth'
import { apiMode } from './mode'

// One WebSocket per tab. Frames (server code is the truth, see internal/transport/ws):
//   client → { type: 'subscribe'|'unsubscribe'|'ping', topic, sub_id, cursor? }
//   server → { type: 'ack'|'event'|'error'|'pong', sub_id, topic, cursor?, payload }
export interface Subscription<T> {
  topic: string
  cursor?: string | null
  onEvent: (payload: T, cursor?: string) => void
  onError?: (problem: unknown) => void
}

export type WsStatus = 'connecting' | 'open' | 'closed' | 'mock'

export interface WsClient {
  subscribe<T>(sub: Subscription<T>): () => void
  status(): WsStatus
  onStatus(cb: (s: WsStatus) => void): () => void
}

interface Frame {
  type: string
  sub_id?: string
  topic?: string
  cursor?: string
  payload?: unknown
  problem?: unknown
}

function createRealWs(path: string): WsClient {
  let socket: WebSocket | undefined
  let status: WsStatus = 'closed'
  let attempt = 0
  const subs = new Map<string, Subscription<unknown>>()
  const statusCbs = new Set<(s: WsStatus) => void>()

  const setStatus = (s: WsStatus) => {
    status = s
    for (const cb of statusCbs) cb(s)
  }

  const send = (frame: Frame) => {
    if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(frame))
  }

  const connect = () => {
    if (
      socket &&
      (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)
    )
      return
    setStatus('connecting')
    void getAccessToken().then((token) => {
      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      socket = new WebSocket(
        `${proto}://${location.host}${path}?token=${encodeURIComponent(token)}`
      )
      socket.onopen = () => {
        attempt = 0
        setStatus('open')
        for (const [id, s] of subs)
          send({ type: 'subscribe', sub_id: id, topic: s.topic, cursor: s.cursor ?? undefined })
      }
      socket.onmessage = (ev) => {
        let frame: Frame
        try {
          frame = JSON.parse(ev.data)
        } catch {
          return
        }
        const s = frame.sub_id ? subs.get(frame.sub_id) : undefined
        if (!s) return
        if (frame.type === 'event') {
          if (frame.cursor) s.cursor = frame.cursor
          s.onEvent(frame.payload, frame.cursor)
        } else if (frame.type === 'error') {
          s.onError?.(frame.problem ?? frame.payload)
        }
      }
      socket.onclose = () => {
        setStatus('closed')
        if (subs.size === 0) return
        const delay = Math.min(30_000, 1000 * 2 ** attempt++)
        window.setTimeout(connect, delay)
      }
      socket.onerror = () => socket?.close()
    })
  }

  const ping = window.setInterval(() => send({ type: 'ping' }), 25_000)
  window.addEventListener('beforeunload', () => window.clearInterval(ping))

  return {
    subscribe(sub) {
      const id = crypto.randomUUID()
      subs.set(id, sub as Subscription<unknown>)
      connect()
      send({ type: 'subscribe', sub_id: id, topic: sub.topic, cursor: sub.cursor ?? undefined })
      return () => {
        subs.delete(id)
        send({ type: 'unsubscribe', sub_id: id })
        if (subs.size === 0) {
          socket?.close()
          socket = undefined
        }
      }
    },
    status: () => status,
    onStatus(cb) {
      statusCbs.add(cb)
      return () => statusCbs.delete(cb)
    },
  }
}

async function createMockWs(): Promise<WsClient> {
  const [{ store }, { getSimulation }] = await Promise.all([
    import('./mock/store'),
    import('./mock/simulation'),
  ])
  const sim = getSimulation(store)
  return {
    subscribe(sub) {
      return sim.subscribe(sub.topic, (payload, cursor) => sub.onEvent(payload as never, cursor))
    },
    status: () => 'mock',
    onStatus: () => () => undefined,
  }
}

// Lazy so the mock bundle never loads in real mode.
let clientPromise: Promise<WsClient> | undefined
export function getWs(): Promise<WsClient> {
  if (!clientPromise)
    clientPromise =
      apiMode === 'mock' ? createMockWs() : Promise.resolve(createRealWs('/api/v1/ws'))
  return clientPromise
}

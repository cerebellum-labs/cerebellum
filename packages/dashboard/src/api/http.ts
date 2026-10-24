// The dashboard API on the server: JSON over HTTP, with the admin session in
// a cookie the page can't read, and events over one shared WebSocket.
import type { Connection, DashboardApi } from './client'
import { ApiError } from './errors'
import type { DashboardEvent, DecisionDetail, DecisionPage, DeviceView, Stats } from './types'

const RECONNECT_MIN_MS = 1_000
const RECONNECT_MAX_MS = 30_000

export function createHttpApi(baseUrl: string): DashboardApi {
  const base = baseUrl.replace(/\/+$/, '')

  async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
    let res: Response
    try {
      res = await fetch(`${base}/api/dashboard${path}`, {
        method,
        // The admin cookie lives on the API's host, a sibling of this page's.
        credentials: 'include',
        headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
        body: body === undefined ? undefined : JSON.stringify(body),
      })
    } catch {
      throw new ApiError(0, 'Could not reach the server.')
    }
    if (!res.ok) throw new ApiError(res.status, await errorMessage(res))
    return (await res.json()) as T
  }

  const listeners = new Set<(event: DashboardEvent) => void>()
  const stream = createStream(streamUrl(base), (event) => {
    for (const listener of listeners) listener(event)
  })

  return {
    async login(password) {
      await request('POST', '/login', { password })
    },
    async logout() {
      await request('POST', '/logout')
    },

    async listDevices() {
      return (await request<{ devices: DeviceView[] }>('GET', '/devices')).devices
    },
    createDevice(input) {
      return request<{ device: DeviceView; token: string }>('POST', '/devices', input)
    },
    async rotateToken(deviceId) {
      return (await request<{ token: string }>('POST', `/devices/${deviceId}/rotate-token`)).token
    },
    async deleteDevice(deviceId) {
      await request('DELETE', `/devices/${deviceId}`)
    },

    listDecisions({ device_id, status, source, changed, before, limit }) {
      const query = new URLSearchParams()
      if (device_id) query.set('device_id', device_id)
      if (status) query.set('status', status)
      if (source) query.set('source', source)
      if (changed) query.set('changed', 'true')
      if (before) query.set('before', before)
      if (limit) query.set('limit', String(limit))
      const qs = query.size ? `?${query}` : ''
      return request<DecisionPage>('GET', `/decisions${qs}`)
    },
    getDecision(id) {
      return request<DecisionDetail>('GET', `/decisions/${id}`)
    },
    stats(deviceId) {
      return request<Stats>('GET', deviceId ? `/stats?device_id=${deviceId}` : '/stats')
    },
    frameUrl(decisionId) {
      return `${base}/api/dashboard/frames/${decisionId}`
    },
    decisionUrl(decisionId) {
      return `${base}/api/dashboard/decisions/${decisionId}`
    },

    subscribe(listener) {
      listeners.add(listener)
      if (listeners.size === 1) stream.open()
      return () => {
        listeners.delete(listener)
        if (listeners.size === 0) stream.close()
      }
    },
    connection: () => stream.state(),
    onConnection: (listener) => stream.onState(listener),
  }
}

/** The server's {"error": {"message": ...}}, or the status text. */
async function errorMessage(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: { message?: string } }
    if (body.error?.message) return body.error.message
  } catch {
    // Not JSON: a proxy's error page.
  }
  return res.statusText || `HTTP ${res.status}`
}

function streamUrl(base: string): string {
  const http = base || window.location.origin
  return `${http.replace(/^http/, 'ws')}/api/dashboard/ws`
}

/**
 * One WebSocket for every listener, open while anyone listens. It reconnects
 * with a growing delay and, once back, says so: events sent in between are
 * lost, so the page reloads what it shows.
 */
function createStream(url: string, emit: (event: DashboardEvent) => void) {
  let socket: WebSocket | null = null
  let timer: number | undefined
  let delay = RECONNECT_MIN_MS
  let wanted = false
  let dropped = false
  let state: Connection = 'connecting'
  const stateListeners = new Set<() => void>()

  function setState(next: Connection) {
    if (next === state) return
    state = next
    for (const listener of stateListeners) listener()
  }

  function connect() {
    timer = undefined
    const ws = new WebSocket(url)
    socket = ws
    ws.onopen = () => {
      delay = RECONNECT_MIN_MS
      if (dropped) emit({ type: 'stream.reconnected' })
      dropped = false
      setState('open')
    }
    ws.onmessage = (message) => {
      try {
        emit(JSON.parse(String(message.data)) as DashboardEvent)
      } catch {
        // A malformed event is skipped; the next ones still count.
      }
    }
    ws.onclose = () => {
      if (socket !== ws || !wanted) return
      socket = null
      dropped = true
      setState('reconnecting')
      timer = window.setTimeout(connect, delay)
      delay = Math.min(delay * 2, RECONNECT_MAX_MS)
    }
  }

  return {
    open() {
      wanted = true
      if (!socket && timer === undefined) connect()
    },
    close() {
      wanted = false
      window.clearTimeout(timer)
      timer = undefined
      socket?.close()
      socket = null
      setState('connecting')
    },
    state: () => state,
    onState(listener: () => void) {
      stateListeners.add(listener)
      return () => {
        stateListeners.delete(listener)
      }
    },
  }
}

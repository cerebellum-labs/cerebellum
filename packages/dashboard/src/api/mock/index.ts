// In-memory implementation of the dashboard API: two devices, a seeded
// journal and, while someone listens to events, a fruit sorter that keeps
// deciding. It follows the server's rules from the spec closely enough that
// the screens can be built and checked against it.
import { cloudDisagreed } from '@/lib/decision'

import type { DashboardApi, RobotEvent, RobotState, Simulator } from '../client'
import { ApiError } from '../errors'
import type {
  Correction,
  DashboardEvent,
  Decision,
  DecisionPage,
  DecisionQuery,
  DecisionView,
  DeviceView,
  ModelCall,
  Stats,
  UUID,
} from '../types'
import { frameDataUrl, type BBox, type Item } from './frames'
import { build, newId, pendingStatus, type Built, type ScenarioKind } from './scenarios'

interface DeviceRecord {
  id: UUID
  key: string
  name: string
  last_seen_at: string | null
  agent_version: string | null
  metadata: Record<string, unknown>
  created_at: string
  deleted: boolean
}

const ONLINE_WINDOW_MS = 15_000
const ADMIN_KEY = 'cerebellum.mock.admin'
const MINUTE = 60_000
/** A review lets the model reason, so it writes more than an answer. */
const REVIEW_COST_USD = 0.0009

const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))
const iso = (ms: number) => new Date(ms).toISOString()

function randomToken(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(32))
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

/** The robot's light for a decision it just made, before the server hears of it. */
const LOCAL_LIGHT: Record<ScenarioKind, RobotEvent & { type: 'decided' }> = {
  mandarin: { type: 'decided', light: 'green', action_class: 'reversible' },
  uncertain: { type: 'decided', light: 'green', action_class: 'reversible' },
  hand: { type: 'decided', light: 'red', action_class: 'reflex' },
  cup: { type: 'decided', light: 'amber', action_class: 'irreversible' },
  kiwi: { type: 'decided', light: 'amber', action_class: 'irreversible' },
  removed: { type: 'decided', light: 'amber', action_class: 'irreversible' },
}

export function createMockApi(): { api: DashboardApi; simulator: Simulator } {
  const now = Date.now()
  const devices = new Map<UUID, DeviceRecord>()
  const decisions = new Map<UUID, Decision>()
  const frames = new Map<UUID, { item: Item; bbox: BBox; label: string }>()
  const corrections = new Map<UUID, Correction>()
  const listeners = new Set<(event: DashboardEvent) => void>()

  const sorter: DeviceRecord = {
    id: newId(),
    key: 'fruit-sorter-1',
    name: 'Fruit sorter',
    last_seen_at: iso(now),
    agent_version: '0.1.0',
    metadata: { camera: 'usb-640x480', board: 'Raspberry Pi 5' },
    created_at: iso(now - 3 * 24 * 60 * MINUTE),
    deleted: false,
  }
  // Idle since its session two hours ago, but its agent still syncs.
  const laptop: DeviceRecord = {
    id: newId(),
    key: 'laptop-1',
    name: 'Laptop webcam',
    last_seen_at: iso(now),
    agent_version: '0.1.0',
    metadata: { camera: 'facetime-hd' },
    created_at: iso(now - 24 * 60 * MINUTE),
    deleted: false,
  }
  devices.set(sorter.id, sorter)
  devices.set(laptop.id, laptop)

  function store(b: Built) {
    decisions.set(b.decision.id, b.decision)
    frames.set(b.decision.id, { item: b.item, bbox: b.bbox, label: b.label })
    if (b.correction) corrections.set(b.correction.id, b.correction)
  }

  // The journal before the page opens: a sorting session, a stretch without
  // network (decided at 22 and 20 minutes ago, synced three minutes later)
  // and an older session on the laptop.
  const seed: [ScenarioKind, number, number?][] = [
    ['mandarin', 46], ['cup', 44], ['kiwi', 43], ['hand', 41], ['mandarin', 39],
    ['uncertain', 37], ['cup', 35], ['mandarin', 33], ['removed', 31], ['kiwi', 29],
    ['mandarin', 27], ['cup', 25], ['mandarin', 22, 3 * MINUTE], ['kiwi', 20, 3 * MINUTE],
    ['mandarin', 15], ['hand', 13], ['cup', 11], ['uncertain', 9], ['mandarin', 6], ['kiwi', 3],
  ]
  for (const [kind, minutesAgo, offlineFor] of seed) {
    store(build(kind, sorter.id, now - minutesAgo * MINUTE, offlineFor))
  }
  for (const [kind, minutesAgo] of [['mandarin', 140], ['cup', 135], ['kiwi', 131]] as const) {
    store(build(kind, laptop.id, now - minutesAgo * MINUTE))
  }

  const isAdmin = () => sessionStorage.getItem(ADMIN_KEY) === '1'
  function requireAdmin() {
    if (!isAdmin()) throw new ApiError(401, 'Sign in to change devices.')
  }

  function deviceView(d: DeviceRecord): DeviceView {
    const seen = d.last_seen_at ? Date.parse(d.last_seen_at) : 0
    return {
      id: d.id,
      key: d.key,
      name: d.name,
      online: Date.now() - seen < ONLINE_WINDOW_MS,
      last_seen_at: d.last_seen_at,
      agent_version: d.agent_version,
      metadata: d.metadata,
      created_at: d.created_at,
    }
  }

  function decisionView(d: Decision): DecisionView {
    const device = d.device_id ? devices.get(d.device_id) : undefined
    return {
      ...d,
      device_key: device?.key ?? 'unknown',
      device_name: device?.name ?? 'Unknown device',
      has_frame: frames.has(d.id),
      corrected_action: latestCorrection(d.id)?.action ?? null,
      cloud_cost_usd: (d.cost_usd ?? 0) + (d.reviewed_at ? REVIEW_COST_USD : 0),
    }
  }

  /** The calls the server would have logged: the answer of a server decision, the review of an uncertain one. */
  function callsOf(d: Decision): ModelCall[] {
    const calls: ModelCall[] = []
    const reply = (action: string, confidence: number, reason: string) =>
      JSON.stringify({ action, confidence, reason })
    if (d.source === 'server' && d.action && d.answered_at) {
      calls.push({
        id: newId(),
        purpose: 'decide',
        model: d.model ?? '',
        action: d.action,
        confidence: d.confidence,
        reason: d.reason,
        output: reply(d.action, d.confidence ?? 0, d.reason ?? ''),
        error: null,
        latency_ms: d.latency_ms ?? 0,
        input_tokens: d.input_tokens,
        output_tokens: d.output_tokens,
        cost_usd: d.cost_usd ?? 0,
        created_at: d.answered_at,
      })
    }
    const correction = latestCorrection(d.id)
    if (d.reviewed_at) {
      const action = correction?.action ?? d.action ?? ''
      const reason = correction?.reason ?? 'Agrees with the robot.'
      calls.push({
        id: newId(),
        purpose: 'review',
        model: correction?.model ?? 'nvidia/Nemotron-3.5-Super-VL',
        action,
        confidence: 0.9,
        reason,
        output: reply(action, 0.9, reason),
        error: null,
        latency_ms: 6_400,
        input_tokens: 1_210,
        output_tokens: 590,
        cost_usd: REVIEW_COST_USD,
        created_at: d.reviewed_at,
      })
    }
    return calls
  }

  function latestCorrection(decisionId: UUID): Correction | undefined {
    let latest: Correction | undefined
    for (const c of corrections.values()) {
      if (c.decision_id === decisionId && (!latest || c.created_at > latest.created_at)) latest = c
    }
    return latest
  }

  function emit(event: DashboardEvent) {
    for (const listener of listeners) listener(event)
  }

  function upsert(d: Decision) {
    decisions.set(d.id, { ...d })
    emit({ type: 'decision.upserted', decision: decisionView(d) })
  }

  // The live sorter. Runs only while someone listens, like a WebSocket would.
  // The simulator can cut its network: it keeps deciding, queues what must
  // wait for the server and syncs everything once the network is back.
  let timers: number[] = []
  const later = (ms: number, fn: () => void) => {
    timers.push(window.setTimeout(fn, ms))
  }
  const liveKinds: ScenarioKind[] = ['mandarin', 'cup', 'mandarin', 'kiwi', 'hand', 'uncertain', 'cup', 'mandarin', 'removed']
  let next = 0

  const robotListeners = new Set<(event: RobotEvent) => void>()
  const robot: RobotState = { online: true, queued: 0, unsynced: 0, offlineSince: null }
  /** Decisions made without network, in the order they were made. */
  let offline: { kind: ScenarioKind; done: Built; at: number }[] = []

  function tellRobot(event: RobotEvent) {
    for (const listener of robotListeners) listener(event)
  }
  function robotChanged() {
    robot.unsynced = offline.length
    tellRobot({ type: 'state', state: { ...robot } })
  }

  function heartbeat() {
    if (!robot.online) return
    for (const device of [sorter, laptop]) {
      device.last_seen_at = iso(Date.now())
      emit({ type: 'device.updated', device: deviceView(device) })
    }
  }

  /** Moves the parts of a decision that happen after the robot reached the server. */
  function shifted(d: Decision, by: number, actedOffline: boolean): Decision {
    const move = (t: string | null) => (t ? iso(Date.parse(t) + by) : null)
    return {
      ...d,
      received_at: move(d.received_at) ?? d.received_at,
      answered_at: move(d.answered_at),
      cancelled_at: move(d.cancelled_at),
      reviewed_at: move(d.reviewed_at),
      executed_at: actedOffline ? d.executed_at : move(d.executed_at),
    }
  }

  /**
   * Plays a decision the way the server sees it, starting now: at once for a
   * fresh one, after the fact for one made offline.
   */
  function deliver(kind: ScenarioKind, done: Built, at: number) {
    const by = Date.now() - at
    const late = by > 1_000
    const irreversible = pendingStatus(kind) != null
    const final = shifted(done.decision, by, !irreversible)

    if (irreversible) {
      if (kind === 'removed' && late) {
        upsert(final)
        return
      }
      upsert({
        ...final,
        status: 'waiting_for_server',
        action: null,
        source: null,
        model: null,
        confidence: null,
        reason: null,
        latency_ms: null,
        cost_usd: null,
        input_tokens: null,
        output_tokens: null,
        answered_at: null,
        executed_at: null,
        received_at: iso(Date.now()),
      })
      if (kind === 'removed') {
        later(1_200, () => upsert(final))
        return
      }
      const answered = final.latency_ms ?? 1_800
      later(answered, () => upsert({ ...final, status: 'decided_on_server', executed_at: null }))
      later(answered + 900, () => upsert(final))
      return
    }

    if (kind === 'uncertain' && done.correction) {
      const correction = { ...done.correction, created_at: iso(Date.parse(done.correction.created_at) + by) }
      upsert({ ...final, status: 'executed', reviewed_at: null })
      later(4_000, () => {
        corrections.set(correction.id, { ...correction, applied_at: null })
        upsert(final)
        emit({ type: 'correction.created', correction: { ...correction, applied_at: null } })
      })
      later(12_000, () => corrections.set(correction.id, { ...correction, applied_at: iso(Date.now()) }))
      return
    }

    if (kind === 'mandarin' && !late) {
      upsert({ ...final, status: 'decided_on_device', executed_at: null })
      later(500, () => upsert(final))
      return
    }
    upsert(final)
  }

  function decideOne() {
    const kind = liveKinds[next++ % liveKinds.length]
    const at = Date.now()
    const done = build(kind, sorter.id, at)
    frames.set(done.decision.id, { item: done.item, bbox: done.bbox, label: done.label })
    tellRobot(LOCAL_LIGHT[kind])

    if (robot.online) {
      deliver(kind, done, at)
      return
    }
    offline.push({ kind, done, at })
    if (pendingStatus(kind)) {
      robot.queued++
      if (kind === 'removed') {
        // The item leaves the zone while the robot waits: the agent cancels it.
        later(1_200, () => {
          if (robot.online) return
          robot.queued--
          tellRobot({ type: 'decided', light: 'off', action_class: 'irreversible' })
          robotChanged()
        })
      }
    }
    robotChanged()
  }

  function setOnline(online: boolean) {
    if (online === robot.online) return
    robot.online = online
    robot.offlineSince = online ? null : Date.now()
    if (!online) {
      // The server stops hearing from the robots: they share the network.
      for (const device of [sorter, laptop]) {
        device.last_seen_at = iso(Date.now() - ONLINE_WINDOW_MS)
        emit({ type: 'device.updated', device: deviceView(device) })
      }
      robotChanged()
      return
    }
    heartbeat()
    const queued = offline
    offline = []
    robot.queued = 0
    robotChanged()
    tellRobot({ type: 'synced', count: queued.length })
    queued.forEach(({ kind, done, at }, i) => later(300 + i * 350, () => deliver(kind, done, at)))
  }

  function start() {
    later(1_200, decideOne)
    timers.push(window.setInterval(heartbeat, 5_000))
    timers.push(window.setInterval(decideOne, 6_000))
  }

  function stop() {
    for (const t of timers) {
      clearTimeout(t)
      clearInterval(t)
    }
    timers = []
  }

  const simulator: Simulator = {
    state: () => ({ ...robot }),
    setOnline,
    subscribe(listener) {
      robotListeners.add(listener)
      return () => robotListeners.delete(listener)
    },
  }

  const api: DashboardApi = {
    // In memory, the stream never breaks.
    connection: () => 'open',
    onConnection: () => () => {},

    async login(password) {
      await wait(250)
      if (!password) throw new ApiError(401, 'Enter the dashboard password.')
      sessionStorage.setItem(ADMIN_KEY, '1')
    },
    async logout() {
      sessionStorage.removeItem(ADMIN_KEY)
    },

    async listDevices() {
      await wait(120)
      return [...devices.values()].filter((d) => !d.deleted).map(deviceView)
    },

    async createDevice({ key, name }) {
      await wait(250)
      requireAdmin()
      if (!/^[a-z0-9][a-z0-9-]{1,62}$/.test(key)) {
        throw new ApiError(400, 'Use lowercase letters, digits and dashes for the key, like fruit-sorter-2.')
      }
      if ([...devices.values()].some((d) => d.key === key)) {
        throw new ApiError(400, `A device with the key ${key} already exists.`)
      }
      const device: DeviceRecord = {
        id: newId(),
        key,
        name: name.trim() || key,
        last_seen_at: null,
        agent_version: null,
        metadata: {},
        created_at: iso(Date.now()),
        deleted: false,
      }
      devices.set(device.id, device)
      const view = deviceView(device)
      emit({ type: 'device.updated', device: view })
      return { device: view, token: randomToken() }
    },

    async rotateToken(deviceId) {
      await wait(250)
      requireAdmin()
      if (!devices.get(deviceId) || devices.get(deviceId)?.deleted) throw new ApiError(404, 'This device no longer exists.')
      return randomToken()
    },

    async deleteDevice(deviceId) {
      await wait(250)
      requireAdmin()
      const device = devices.get(deviceId)
      if (!device) throw new ApiError(404, 'This device no longer exists.')
      device.deleted = true
      emit({ type: 'device.updated', device: deviceView(device) })
    },

    async listDecisions({ device_id, status, source, changed, before, limit = 50 }: DecisionQuery): Promise<DecisionPage> {
      await wait(150)
      const size = Math.min(Math.max(limit, 1), 200)
      // The server pages by UUIDv7, which sorts by time; the mock's ids are
      // random, so it pages by decided_at and finds the cursor's time.
      const cursor = before ? decisions.get(before)?.decided_at : undefined
      const matching = [...decisions.values()]
        .filter((d) => (!device_id || d.device_id === device_id) && (!status || d.status === status))
        .filter((d) => (!source || d.source === source) && (!changed || cloudDisagreed(d)))
        .filter((d) => !cursor || d.decided_at < cursor)
        .sort((a, b) => b.decided_at.localeCompare(a.decided_at))
      const page = matching.slice(0, size)
      return {
        decisions: page.map(decisionView),
        next_before: matching.length > size ? page[page.length - 1].id : null,
      }
    },

    async getDecision(id) {
      await wait(100)
      const decision = decisions.get(id)
      if (!decision) throw new ApiError(404, 'There is no decision with this id.')
      return {
        decision: decisionView(decision),
        corrections: [...corrections.values()].filter((c) => c.decision_id === id),
        calls: callsOf(decision),
      }
    },

    async stats(deviceId) {
      await wait(120)
      const list = [...decisions.values()].filter((d) => !deviceId || d.device_id === deviceId)
      const stats: Stats = {
        total: list.length,
        by_source: { device: 0, server: 0 },
        by_status: {
          decided_on_device: 0,
          waiting_for_server: 0,
          decided_on_server: 0,
          executed: 0,
          corrected: 0,
          cancelled: 0,
        },
        by_class: { reflex: 0, reversible: 0, irreversible: 0 },
        corrections: [...corrections.values()].filter((c) => list.some((d) => d.id === c.decision_id)).length,
        cost_usd: 0,
        avg_latency_ms: { device: null, server: null },
      }
      const latency = { device: [] as number[], server: [] as number[] }
      for (const d of list) {
        if (d.source) stats.by_source[d.source]++
        stats.by_status[d.status]++
        stats.by_class[d.action_class]++
        stats.cost_usd += d.cost_usd ?? 0
        if (d.source && d.latency_ms != null) latency[d.source].push(d.latency_ms)
      }
      for (const source of ['device', 'server'] as const) {
        const values = latency[source]
        stats.avg_latency_ms[source] = values.length ? Math.round(values.reduce((a, b) => a + b, 0) / values.length) : null
      }
      return stats
    },

    frameUrl(decisionId) {
      const frame = frames.get(decisionId)
      return frame ? frameDataUrl(frame.item, frame.bbox, frame.label) : ''
    },
    decisionUrl: () => null,

    subscribe(listener) {
      listeners.add(listener)
      if (listeners.size === 1) start()
      return () => {
        listeners.delete(listener)
        if (listeners.size === 0) stop()
      }
    },
  }

  return { api, simulator }
}

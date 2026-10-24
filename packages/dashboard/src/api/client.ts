import { createHttpApi } from './http'
import { createMockApi } from './mock'
import type {
  ActionClass,
  DashboardEvent,
  DecisionDetail,
  DecisionPage,
  DecisionQuery,
  DeviceView,
  Stats,
  UUID,
} from './types'

export { ApiError } from './errors'

/** The dashboard API (packages/server/internal/dashboard). */
export interface DashboardApi {
  login(password: string): Promise<void>
  logout(): Promise<void>

  listDevices(): Promise<DeviceView[]>
  createDevice(input: { key: string; name: string }): Promise<{ device: DeviceView; token: string }>
  rotateToken(deviceId: UUID): Promise<string>
  deleteDevice(deviceId: UUID): Promise<void>

  listDecisions(query: DecisionQuery): Promise<DecisionPage>
  getDecision(id: UUID): Promise<DecisionDetail>
  stats(deviceId?: UUID): Promise<Stats>
  frameUrl(decisionId: UUID): string
  /** The decision's record on the API, to open as is; null where there is no API to open. */
  decisionUrl(decisionId: UUID): string | null

  /** Server events (WebSocket /api/dashboard/ws). Returns the unsubscribe function. */
  subscribe(listener: (event: DashboardEvent) => void): () => void
  /** The state of the event stream, for showing whether the page is live. */
  connection(): Connection
  /** Calls `listener` when the stream's state changes. Returns the unsubscribe function. */
  onConnection(listener: () => void): () => void
}

/** The event stream: opening, open, or broken and trying again. */
export type Connection = 'connecting' | 'open' | 'reconnecting'

/** What the simulated robot is doing right now, beyond what the server sees. */
export interface RobotState {
  online: boolean
  /** Irreversible decisions waiting for the network. */
  queued: number
  /** Decisions made offline that the server hasn't seen yet. */
  unsynced: number
  /** When the network went down, epoch ms; null while online. */
  offlineSince: number | null
}

export type RobotEvent =
  | { type: 'decided'; light: 'green' | 'blue' | 'amber' | 'red' | 'off'; action_class: ActionClass }
  | { type: 'state'; state: RobotState }
  | { type: 'synced'; count: number }

/** Demo controls for the simulated robot: only the mock has one. */
export interface Simulator {
  state(): RobotState
  setOnline(online: boolean): void
  subscribe(listener: (event: RobotEvent) => void): () => void
}

// The server's API, unless VITE_API_MOCK=true asks for the in-memory mock
// with a simulated fruit sorter: for a demo without a robot, and the only
// one with a simulator. Vite replaces the flag at build time, so a build for
// the server leaves the mock out.
const mock = import.meta.env.VITE_API_MOCK === 'true' ? createMockApi() : null
export const api: DashboardApi = mock?.api ?? createHttpApi(import.meta.env.VITE_API_URL ?? '')
export const simulator: Simulator | null = mock?.simulator ?? null
export const isMock = mock !== null

import { createHttpApi } from './http'
import { createMockApi } from './mock'
import type {
  ActionClass,
  Correction,
  DashboardEvent,
  DecisionPage,
  DecisionQuery,
  DecisionView,
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
  getDecision(id: UUID): Promise<{ decision: DecisionView; corrections: Correction[] }>
  stats(deviceId?: UUID): Promise<Stats>
  frameUrl(decisionId: UUID): string

  /** Server events (WebSocket /api/dashboard/ws). Returns the unsubscribe function. */
  subscribe(listener: (event: DashboardEvent) => void): () => void
}

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

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

/** The dashboard API from section 7 of the spec. */
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

// Until the server implements section 7, the dashboard runs on an in-memory
// mock with a simulated fruit sorter. An HTTP implementation of this same
// interface replaces it here, with no simulator.
const mock = createMockApi()
export const api: DashboardApi = mock.api
export const simulator: Simulator | null = mock.simulator
export const isMock = true

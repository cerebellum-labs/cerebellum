// Types of the dashboard API, as the server sends them
// (packages/server/internal/dashboard).

export type UUID = string
/** ISO 8601, always UTC with Z: "2026-10-21T18:09:00.123Z". */
export type ISODateTime = string
export type Json = Record<string, unknown>

export type ActionClass = 'reflex' | 'reversible' | 'irreversible'
export type DecisionSource = 'device' | 'server'
export type DecisionStatus =
  | 'decided_on_device'
  | 'waiting_for_server'
  | 'decided_on_server'
  | 'executed'
  | 'corrected'
  | 'cancelled'

export interface Decision {
  /** UUIDv7 made by the robot's agent: ids sort by when the decision was made. */
  id: UUID
  device_id: UUID
  /** Bumped by the agent on every change it syncs. */
  revision: number
  task: string
  options: string[]
  action_class: ActionClass
  status: DecisionStatus
  /** Who decided: null while the server hasn't answered. */
  source: DecisionSource | null
  needs_review: boolean
  metadata: Json

  proposed_action: string | null
  action: string | null
  prompt: string | null
  local_model: string | null
  local_confidence: number | null
  /** The model behind the action: the local one on the device, the cloud one on the server. */
  model: string | null
  confidence: number | null
  /** The cloud model's explanation of a server decision. */
  reason: string | null
  /** The cloud call of a server decision: how long it took and what it cost. */
  latency_ms: number | null
  cost_usd: number | null
  input_tokens: number | null
  output_tokens: number | null
  /** Why the robot cancelled, like the item leaving before the answer. */
  cancel_reason: string | null

  /** The robot's clock. */
  decided_at: ISODateTime
  /** The server's clock: when the decision first reached it. */
  received_at: ISODateTime
  /** When the cloud model answered a server decision. */
  answered_at: ISODateTime | null
  executed_at: ISODateTime | null
  cancelled_at: ISODateTime | null
  reviewed_at: ISODateTime | null
  /** When the robot carried out the correction, if there is one. */
  correction_applied_at: ISODateTime | null
}

/** A review that disagreed with the device. A decision has one at most, and it shares the decision's id. */
export interface Correction {
  id: UUID
  decision_id: UUID
  action: string
  reason: string
  model: string
  created_at: ISODateTime
  applied_at: ISODateTime | null
}

export interface DeviceView {
  id: UUID
  key: string
  name: string
  /** last_seen_at less than 15 seconds ago. */
  online: boolean
  last_seen_at: ISODateTime | null
  agent_version: string | null
  metadata: Json
  created_at: ISODateTime
}

export type DecisionView = Decision & {
  device_key: string
  device_name: string
  has_frame: boolean
  /** The correction's action, null if there is none: the journal shows both answers. */
  corrected_action: string | null
}

export interface Stats {
  total: number
  by_source: Record<DecisionSource, number>
  by_status: Record<DecisionStatus, number>
  by_class: Record<ActionClass, number>
  corrections: number
  cost_usd: number
  avg_latency_ms: Record<DecisionSource, number | null>
}

export type DashboardEvent =
  | { type: 'decision.upserted'; decision: DecisionView }
  | { type: 'correction.created'; correction: Correction }
  | { type: 'device.updated'; device: DeviceView }
  /** The event stream is back after a break: what was sent meanwhile is lost. */
  | { type: 'stream.reconnected' }

export interface DecisionQuery {
  device_id?: UUID
  status?: DecisionStatus
  /** next_before of the previous page. */
  before?: UUID
  /** 50 by default, at most 200. */
  limit?: number
}

export interface DecisionPage {
  decisions: DecisionView[]
  /** The cursor of the next page, null after the last one. */
  next_before: UUID | null
}

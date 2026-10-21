// Types of the dashboard API, as in the spec (sections 3, 4 and 7).

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
  id: UUID
  task: string
  options: string[]
  action_class: ActionClass
  status: DecisionStatus
  needs_review: boolean
  attempts: number
  metadata: Json
  decided_at: ISODateTime
  updated_at: ISODateTime

  device_id: UUID | null
  proposed_action: string | null
  action: string | null
  source: DecisionSource | null
  model: string | null
  confidence: number | null
  reason: string | null
  prompt: string | null
  local_model: string | null
  local_confidence: number | null
  latency_ms: number | null
  cost_usd: number | null
  input_tokens: number | null
  output_tokens: number | null
  executed_at: ISODateTime | null
  reviewed_at: ISODateTime | null
  synced_at: ISODateTime | null
}

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
  /**
   * The action of the latest correction, null if there is none. Corrections
   * are separate records, so the journal gets it joined in to show both
   * answers without loading every decision's corrections.
   */
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

export interface DecisionQuery {
  device_id?: UUID
  status?: DecisionStatus
  before?: ISODateTime
  /** 50 by default, at most 200. */
  limit?: number
}

export interface DecisionPage {
  decisions: DecisionView[]
  next_before: ISODateTime | null
}

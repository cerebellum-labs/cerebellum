// The fruit sorter from section 10 of the spec: what the detector sees, what
// the robot proposes, and how each case ends.
import type { Correction, Decision, DecisionStatus, ISODateTime, UUID } from '../types'
import type { BBox, Item } from './frames'

const MODEL_ID = 'nvidia/Nemotron-3.5-Super-VL'
const LOCAL_MODEL = 'yolox-nano'
const PRICE_INPUT_PER_M = 0.3
const PRICE_OUTPUT_PER_M = 0.9

const SORT_PROMPT = 'Is the object in the photo a fruit? Answer basket if it is a fruit, discard otherwise.'

export type ScenarioKind = 'mandarin' | 'cup' | 'kiwi' | 'uncertain' | 'hand' | 'removed'

export interface Built {
  decision: Decision
  item: Item
  bbox: BBox
  label: string
  correction?: Correction
}

const iso = (ms: number): ISODateTime => new Date(ms).toISOString()
const jitter = (base: number, spread: number) => Math.round(base + (Math.random() - 0.5) * 2 * spread)

export function newId(): UUID {
  return crypto.randomUUID()
}

function base(deviceId: UUID, at: number, item: Item, label: string, localConfidence: number | null): Omit<Built, 'decision'> & { decision: Decision } {
  const bbox: BBox = [jitter(250, 40), jitter(170, 30), 0, 0]
  const size = item === 'hand' ? 220 : jitter(130, 15)
  bbox[2] = bbox[0] + size
  bbox[3] = bbox[1] + (item === 'hand' ? 150 : size)
  return {
    item,
    bbox,
    label: localConfidence == null ? label : `${label} ${localConfidence.toFixed(2)}`,
    decision: {
      id: newId(),
      task: 'sort_item',
      options: ['basket', 'discard'],
      action_class: 'reversible',
      status: 'executed',
      source: null,
      needs_review: false,
      metadata: {
        detector: { label, confidence: localConfidence },
        bbox,
        servo_angle: 90,
      },
      device_id: deviceId,
      proposed_action: null,
      action: null,
      model: null,
      confidence: null,
      reason: null,
      prompt: SORT_PROMPT,
      local_model: LOCAL_MODEL,
      local_confidence: localConfidence,
      latency_ms: null,
      cost_usd: null,
      input_tokens: null,
      output_tokens: null,
      cancel_reason: null,
      decided_at: iso(at),
      received_at: iso(at + 2_000),
      answered_at: null,
      executed_at: null,
      cancelled_at: null,
      reviewed_at: null,
    },
  }
}

function serverCall(d: Decision, action: string, confidence: number, reason: string, at: number): number {
  const latency = jitter(1_750, 450)
  const input = jitter(1_180, 60)
  const output = jitter(38, 8)
  d.source = 'server'
  d.model = MODEL_ID
  d.action = action
  d.confidence = confidence
  d.reason = reason
  d.latency_ms = latency
  d.input_tokens = input
  d.output_tokens = output
  d.cost_usd = (input * PRICE_INPUT_PER_M + output * PRICE_OUTPUT_PER_M) / 1e6
  d.received_at = iso(at)
  d.answered_at = iso(at + latency)
  return latency
}

function onDevice(d: Decision, action: string, at: number) {
  d.source = 'device'
  d.model = LOCAL_MODEL
  d.action = action
  d.confidence = d.local_confidence
  d.executed_at = iso(at + 400)
}

/**
 * One finished decision. `offlineFor` delays the server answer and the sync,
 * as if the network was down when the robot decided.
 */
export function build(kind: ScenarioKind, deviceId: UUID, at: number, offlineFor = 0): Built {
  switch (kind) {
    case 'mandarin': {
      const b = base(deviceId, at, 'mandarin', 'orange', 0.94)
      b.decision.proposed_action = 'basket'
      b.decision.status = 'executed'
      onDevice(b.decision, 'basket', at)
      b.decision.metadata.servo_angle = 30
      if (offlineFor) b.decision.received_at = iso(at + offlineFor)
      return b
    }
    case 'cup':
    case 'kiwi': {
      const kiwi = kind === 'kiwi'
      const b = base(deviceId, at, kiwi ? 'kiwi' : 'cup', kiwi ? 'unknown' : 'cup', kiwi ? null : 0.88)
      const d = b.decision
      d.proposed_action = 'discard'
      d.action_class = 'irreversible'
      const answeredAt = at + offlineFor
      const latency = serverCall(
        d,
        kiwi ? 'basket' : 'discard',
        kiwi ? 0.91 : 0.97,
        kiwi ? 'A kiwi, which is a fruit.' : 'A plastic cup, not a fruit.',
        answeredAt,
      )
      d.status = 'executed'
      d.executed_at = iso(answeredAt + latency + 500)
      d.metadata.servo_angle = kiwi ? 30 : 150
      return b
    }
    case 'uncertain': {
      const b = base(deviceId, at, 'ball', 'orange', 0.63)
      const d = b.decision
      d.proposed_action = 'basket'
      d.needs_review = true
      onDevice(d, 'basket', at)
      d.metadata.servo_angle = 30
      const reviewedAt = at + offlineFor + 4_000
      const correction: Correction = {
        id: d.id,
        decision_id: d.id,
        action: 'discard',
        reason: 'A yellow tennis ball, not an orange.',
        model: MODEL_ID,
        created_at: iso(reviewedAt),
        applied_at: iso(reviewedAt + 9_000),
      }
      d.status = 'corrected'
      d.reviewed_at = iso(reviewedAt)
      return { ...b, correction }
    }
    case 'hand': {
      const b = base(deviceId, at, 'hand', 'person', 0.88)
      const d = b.decision
      d.task = 'safety'
      d.options = ['stop']
      d.action_class = 'reflex'
      d.prompt = null
      d.proposed_action = 'stop'
      onDevice(d, 'stop', at)
      d.executed_at = d.decided_at
      d.status = 'executed'
      return b
    }
    case 'removed': {
      const b = base(deviceId, at, 'cup', 'cup', 0.71)
      const d = b.decision
      d.proposed_action = 'discard'
      d.action_class = 'irreversible'
      d.status = 'cancelled'
      d.received_at = iso(at)
      d.cancel_reason = 'The item left the zone before the server answered.'
      d.cancelled_at = iso(at + 1_200)
      return b
    }
  }
}

/** The status a decision passes through before it is finished, for the live feed. */
export function pendingStatus(kind: ScenarioKind): DecisionStatus | null {
  return kind === 'cup' || kind === 'kiwi' || kind === 'removed' ? 'waiting_for_server' : null
}

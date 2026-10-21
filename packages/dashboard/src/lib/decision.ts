// How decisions read on screen. The colors are the sorter's LEDs from the
// demo: green basket, blue discard, yellow waiting, red stop, white correction.
import type { ActionClass, Decision, DecisionSource, DecisionStatus } from '@/api/types'

export type LedColor = 'green' | 'blue' | 'amber' | 'red' | 'white' | 'off'

export const STATUS_LABEL: Record<DecisionStatus, string> = {
  decided_on_device: 'Decided on device',
  waiting_for_server: 'Waiting for server',
  decided_on_server: 'Decided by server',
  executed: 'Done',
  corrected: 'Corrected',
  cancelled: 'Cancelled',
}

export const STATUSES = Object.keys(STATUS_LABEL) as DecisionStatus[]

export const CLASS_LABEL: Record<ActionClass, string> = {
  reflex: 'Reflex',
  reversible: 'Reversible',
  irreversible: 'Irreversible',
}

export const SOURCE_LABEL: Record<DecisionSource, string> = {
  device: 'On device',
  server: 'In the cloud',
}

const ACTION_LED: Record<string, LedColor> = {
  basket: 'green',
  discard: 'blue',
  stop: 'red',
}

/** The light the robot showed for this decision. */
export function ledOf(d: Pick<Decision, 'status' | 'action'>): LedColor {
  if (d.status === 'waiting_for_server') return 'amber'
  if (d.status === 'corrected') return 'white'
  if (d.status === 'cancelled' || !d.action) return 'off'
  return ACTION_LED[d.action] ?? 'off'
}

/** "sort_item" → "Sort item". */
export function humanize(name: string): string {
  const words = name.replace(/[_-]+/g, ' ').trim()
  return words.charAt(0).toUpperCase() + words.slice(1)
}

/**
 * "nvidia/Nemotron-3.5-Super-VL" → "Nemotron 3.5 Super VL". Local model names
 * like "yolox-nano" are shown as the robot reports them.
 */
export function modelName(model: string): string {
  if (!model.includes('/')) return model
  return model.split('/').pop()!.replace(/-/g, ' ')
}

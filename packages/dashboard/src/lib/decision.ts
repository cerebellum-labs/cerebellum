// How decisions read on screen. The colors are the sorter's LEDs from the
// demo: green basket, blue discard, yellow waiting, red stop, white correction.
import type { ActionClass, Decision, DecisionSource, DecisionStatus } from '@/api/types'

export type LedColor = 'green' | 'blue' | 'amber' | 'red' | 'white' | 'off'

// Said in the reader's terms: the robot and the cloud, not device and server.
export const STATUS_LABEL: Record<DecisionStatus, string> = {
  decided_on_device: 'Decided on the robot',
  waiting_for_server: 'Asking the cloud',
  decided_on_server: 'Decided in the cloud',
  executed: 'Done',
  corrected: 'Corrected by the cloud',
  cancelled: 'Cancelled',
}

export const CLASS_LABEL: Record<ActionClass, string> = {
  reflex: 'Reflex',
  reversible: 'Reversible',
  irreversible: 'Irreversible',
}

export const SOURCE_LABEL: Record<DecisionSource, string> = {
  device: 'On the robot',
  server: 'In the cloud',
}

/**
 * Where the cloud model runs: the server calls it on Nebius Token Factory
 * (NEBIUS_API_KEY, NEBIUS_BASE_URL).
 */
export const CLOUD_PROVIDER = 'Nebius'

/** What each of the robot's lights means, as the legend and a decision explain it. */
export const LIGHT: Record<LedColor, { name: string; lit: string; meaning: string }> = {
  green: { name: 'Basket', lit: 'Green light', meaning: 'A fruit, put in the basket' },
  blue: { name: 'Discard', lit: 'Blue light', meaning: 'Not a fruit, thrown out' },
  red: { name: 'Stop', lit: 'Red light', meaning: 'A person in the zone, the robot froze' },
  amber: { name: 'Waiting', lit: 'Yellow light', meaning: 'Not sure, asked the cloud model' },
  white: { name: 'Corrected', lit: 'White light', meaning: 'The cloud model fixed a decision' },
  off: { name: 'Cancelled', lit: 'No light', meaning: 'The item left before the answer' },
}

const ACTION_LED: Record<string, LedColor> = {
  basket: 'green',
  discard: 'blue',
  stop: 'red',
}

/**
 * Did the cloud disagree with the robot, by overruling its proposal or by
 * correcting it afterwards? The journal's "changed" filter, and the robot's
 * mistakes when the cloud is taken as right.
 */
export function cloudDisagreed(d: Pick<Decision, 'status' | 'source' | 'proposed_action' | 'action'>): boolean {
  if (d.status === 'corrected') return true
  return d.source === 'server' && d.proposed_action != null && d.action != null && d.action !== d.proposed_action
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

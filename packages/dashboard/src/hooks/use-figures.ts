import type { DecisionView } from '@/api/types'
import { useRecentDecisions } from '@/hooks/use-recent-decisions'

const average = (values: number[]) => (values.length ? values.reduce((a, b) => a + b, 0) / values.length : null)

/** Would the robot alone have done the wrong thing? The cloud's answer is taken as right. */
function robotWasWrong(d: DecisionView): boolean {
  if (d.status === 'corrected') return true
  return d.source === 'server' && d.proposed_action != null && d.action != null && d.action !== d.proposed_action
}

/** What the same decisions would have taken with the cloud answering every one. */
function figures(rows: DecisionView[]): Figures | null {
  const decided = rows.filter((d) => d.source)
  const cloud = average(rows.filter((d) => d.source === 'server' && d.latency_ms != null).map((d) => d.latency_ms!))
  if (cloud == null) return null
  // A decision on the device takes milliseconds against the cloud's seconds,
  // and the server isn't told how long: it counts as no wait at all.
  const blended = average(decided.map((d) => (d.source === 'device' ? 0 : (d.latency_ms ?? cloud))))
  const perCall = average(rows.filter((d) => d.source === 'server' && d.cost_usd).map((d) => d.cost_usd!)) ?? 0
  const spent = rows.reduce((sum, d) => sum + (d.cost_usd ?? 0), 0)
  if (blended == null) return null
  return {
    decided: decided.length,
    cloud,
    blended,
    spent,
    cloudOnly: decided.length * perCall,
    mistakes: rows.filter(robotWasWrong).length,
    faster: cloud / blended,
    saved: decided.length * perCall ? Math.max(0, 1 - spent / (decided.length * perCall)) : 0,
  }
}

export interface Figures {
  decided: number
  /** Average time to decide, ms: in the cloud, and overall. */
  cloud: number
  blended: number
  spent: number
  /** What the cloud would have cost answering every decision. */
  cloudOnly: number
  /** Decisions where the cloud overruled or corrected the robot. */
  mistakes: number
  /** How many times faster than asking the cloud every time. */
  faster: number
  /** Share of the cloud bill saved against asking every time. */
  saved: number
}

/**
 * What Cerebellum gains over asking the cloud every time, from the journal's
 * latest decisions. Undefined while loading, null until the cloud has
 * answered at least one decision.
 */
export function useFigures(): Figures | null | undefined {
  const recent = useRecentDecisions()
  return recent.data && figures(recent.data)
}

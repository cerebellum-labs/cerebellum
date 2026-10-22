import type { DecisionView } from '@/api/types'
import { useRecentDecisions } from '@/hooks/use-recent-decisions'
import { cloudDisagreed } from '@/lib/decision'

const average = (values: number[]) => (values.length ? values.reduce((a, b) => a + b, 0) / values.length : null)

/** The middle value: what a typical decision takes, not pulled up by the few slow ones. */
function median(values: number[]): number | null {
  if (!values.length) return null
  const sorted = [...values].sort((a, b) => a - b)
  const mid = Math.floor(sorted.length / 2)
  return sorted.length % 2 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2
}

/** What the same decisions would have taken with the cloud answering every one. */
function figures(rows: DecisionView[]): Figures | null {
  const decided = rows.filter((d) => d.source)
  const cloudTimes = rows.filter((d) => d.source === 'server' && d.latency_ms != null).map((d) => d.latency_ms!)
  const cloud = average(cloudTimes)
  if (cloud == null) return null
  // A decision on the device takes milliseconds against the cloud's seconds;
  // from an agent that doesn't report how long, it counts as no wait at all.
  const times = decided.map((d) => d.latency_ms ?? (d.source === 'device' ? 0 : cloud))
  const blended = average(times)
  const perCall = average(rows.filter((d) => d.source === 'server' && d.cost_usd).map((d) => d.cost_usd!)) ?? 0
  const spent = rows.reduce((sum, d) => sum + (d.cost_usd ?? 0), 0)
  if (blended == null) return null
  return {
    decided: decided.length,
    cloud,
    blended,
    cloudTypical: median(cloudTimes)!,
    typical: median(times)!,
    spent,
    cloudOnly: decided.length * perCall,
    // The robot alone would have done the wrong thing: the cloud is taken as right.
    mistakes: rows.filter(cloudDisagreed).length,
    faster: cloud / blended,
    saved: decided.length * perCall ? Math.max(0, 1 - spent / (decided.length * perCall)) : 0,
  }
}

export interface Figures {
  decided: number
  /** Average time to decide, ms: in the cloud, and overall. */
  cloud: number
  blended: number
  /** Median time to decide, ms: in the cloud, and overall. */
  cloudTypical: number
  typical: number
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

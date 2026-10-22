import { useState } from 'react'

import type { DecisionView } from '@/api/types'

/**
 * Keeps new decisions from pushing the journal under the reader. While `hold`
 * is on, decisions newer than the newest one shown wait behind a count, and
 * `release` shows them. Off, every decision shows as it comes.
 */
export function useHeldRows(rows: DecisionView[], hold: boolean) {
  // The newest decision the reader has seen, while holding.
  const [cutoff, setCutoff] = useState<string | null>(null)
  const [holding, setHolding] = useState(false)

  // Adjusted during render, so the first render after a change is already
  // right: no frame with the new rows in place before they are held back.
  const newest = rows[0]?.decided_at ?? null
  if (hold !== holding) {
    setHolding(hold)
    setCutoff(hold ? newest : null)
  } else if (hold && cutoff === null && newest) {
    setCutoff(newest)
  }

  const effective = hold ? (cutoff ?? newest) : null
  const shown = effective ? rows.filter((d) => d.decided_at <= effective) : rows
  const held = rows.length - shown.length
  return { shown, held, release: () => setCutoff(newest) }
}

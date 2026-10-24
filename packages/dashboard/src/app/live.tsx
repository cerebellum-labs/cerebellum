import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, type ReactNode } from 'react'

import { api } from '@/api/client'
import { LiveContext } from '@/app/context'

/** How long a decision that just changed stays highlighted in the journal. */
const RECENT_MS = 2_500

/**
 * Listens to the server's events and keeps every screen current: the journal,
 * the open decision, devices and, at most once a second, the stats.
 */
export function LiveProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [changed, setChanged] = useState<ReadonlySet<string>>(new Set())
  const [added, setAdded] = useState<ReadonlySet<string>>(new Set())

  useEffect(() => {
    const timers = new Set<number>()
    const known = new Set<string>()
    let statsPending = false
    let decisionsPending = false

    const mark = (set: typeof setChanged, id: string) => {
      set((current) => new Set(current).add(id))
      const timer = window.setTimeout(() => {
        timers.delete(timer)
        set((current) => {
          const next = new Set(current)
          next.delete(id)
          return next
        })
      }, RECENT_MS)
      timers.add(timer)
    }
    const markRecent = (id: string) => {
      mark(setChanged, id)
      // The first event about a decision means it has just been made.
      if (!known.has(id)) {
        known.add(id)
        mark(setAdded, id)
      }
    }

    const refreshStats = () => {
      if (statsPending) return
      statsPending = true
      const timer = window.setTimeout(() => {
        timers.delete(timer)
        statsPending = false
        void queryClient.invalidateQueries({ queryKey: ['stats'] })
      }, 1_000)
      timers.add(timer)
    }

    // A sync after an outage sends a burst of events: the lists reload once
    // for all of them, and a reload already running isn't cut short.
    const refreshDecisions = () => {
      if (decisionsPending) return
      decisionsPending = true
      const timer = window.setTimeout(() => {
        timers.delete(timer)
        decisionsPending = false
        void queryClient.invalidateQueries({ queryKey: ['decisions'] }, { cancelRefetch: false })
      }, 400)
      timers.add(timer)
    }

    const unsubscribe = api.subscribe((event) => {
      switch (event.type) {
        case 'decision.upserted':
          refreshDecisions()
          void queryClient.invalidateQueries({ queryKey: ['decision', event.decision.id] })
          markRecent(event.decision.id)
          refreshStats()
          break
        case 'correction.created':
          refreshDecisions()
          void queryClient.invalidateQueries({ queryKey: ['decision', event.correction.decision_id] })
          known.add(event.correction.decision_id)
          markRecent(event.correction.decision_id)
          refreshStats()
          break
        case 'device.updated':
          void queryClient.invalidateQueries({ queryKey: ['devices'] })
          break
        case 'stream.reconnected':
          // Events sent during the break are lost: reload everything shown.
          void queryClient.invalidateQueries()
          break
      }
    })

    return () => {
      unsubscribe()
      for (const timer of timers) clearTimeout(timer)
    }
  }, [queryClient])

  return <LiveContext value={{ changed, added }}>{children}</LiveContext>
}

import { ArrowCounterClockwise, Check, Clock, Lightning, LockSimple } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'

import { api } from '@/api/client'
import type { ActionClass, DecisionView } from '@/api/types'
import { Skeleton } from '@/components/ui/skeleton'
import { useRecentDecisions } from '@/hooks/use-recent-decisions'
import { useRobot } from '@/hooks/use-robot'
import { CLASS_LABEL, humanize } from '@/lib/decision'
import { count } from '@/lib/format'
import { cn } from '@/lib/utils'

// How each class behaves is the product's rule, the same for every robot.
const CLASSES: { key: ActionClass; icon: ReactNode; decides: string; offline: string; waitsOffline: boolean }[] = [
  {
    key: 'reflex',
    icon: <Lightning className="size-4" />,
    decides: 'The robot, instantly',
    offline: 'Always works',
    waitsOffline: false,
  },
  {
    key: 'reversible',
    icon: <ArrowCounterClockwise className="size-4" />,
    decides: 'The robot, checked later',
    offline: 'Keeps working',
    waitsOffline: false,
  },
  {
    key: 'irreversible',
    icon: <LockSimple className="size-4" />,
    decides: 'Only the cloud model',
    offline: 'Waits for network',
    waitsOffline: true,
  },
]

// One grid for the header and the rows; on phones each row stacks, every
// cell with its column's name.
const ROW =
  'grid gap-x-6 gap-y-2 px-4 sm:px-5 lg:grid-cols-[minmax(0,10rem)_minmax(0,1.3fr)_minmax(0,1fr)_minmax(0,1fr)_5rem] lg:items-center'

/** The actions the robots actually took in a class, most frequent first. */
function actionsOf(rows: DecisionView[], actionClass: ActionClass): string[] {
  const seen = new Map<string, number>()
  for (const d of rows) {
    if (d.action_class === actionClass && d.action) seen.set(d.action, (seen.get(d.action) ?? 0) + 1)
  }
  return [...seen.entries()].sort((a, b) => b[1] - a[1]).map(([action]) => action)
}

/** The rules as a table: who decides each kind of action, and what happens offline. */
export function ActionClasses() {
  const robot = useRobot()
  const stats = useQuery({ queryKey: ['stats', null], queryFn: () => api.stats() })
  const recent = useRecentDecisions()
  const s = stats.data
  const waitingNow = robot.queued || s?.by_status.waiting_for_server || 0

  return (
    <section aria-labelledby="rules-title" className="grid gap-6">
      <div className="grid gap-1">
        <h2 id="rules-title" className="text-xl font-semibold tracking-tight">
          Who decides what
        </h2>
        <p className="text-sm text-muted-foreground">The costlier a mistake, the more the cloud decides.</p>
      </div>

      <div role="table" aria-labelledby="rules-title" className="panel overflow-hidden">
        <div role="row" className={cn(ROW, 'hidden h-11 border-b text-sm text-muted-foreground lg:grid')}>
          <span role="columnheader">Class</span>
          <span role="columnheader">Who decides</span>
          <span role="columnheader">Without network</span>
          <span role="columnheader">Outcomes in the demo</span>
          <span role="columnheader" className="text-right">
            Decisions
          </span>
        </div>
        {CLASSES.map((c) => {
          // Only a network cut changes the rules; a switched-off stand doesn't.
          const outage = !robot.online && !robot.idle
          const waiting = c.waitsOffline && outage
          const actions = actionsOf(recent.data ?? [], c.key).slice(0, 3)
          return (
            <div role="row" key={c.key} className={cn(ROW, 'border-b py-4 last:border-b-0')}>
              <span role="rowheader" className="flex items-center gap-2.5 font-medium">
                <span className="text-brand">{c.icon}</span>
                {CLASS_LABEL[c.key]}
              </span>
              <Cell label="Who decides">{c.decides}</Cell>
              <Cell
                label="Without network"
                className={cn(
                  'flex items-center gap-2 transition-colors duration-500',
                  waiting ? 'text-led-amber' : outage ? 'text-led-green' : 'text-muted-foreground',
                )}
              >
                {c.waitsOffline ? (
                  <Clock className="size-4 shrink-0" aria-hidden />
                ) : (
                  <Check className="size-4 shrink-0" aria-hidden />
                )}
                {waiting ? `${count(waitingNow)} waiting now` : outage ? 'Working now' : c.offline}
              </Cell>
              <Cell label="Outcomes in the demo" className="flex flex-wrap gap-1.5">
                {recent.isPending ? (
                  <>
                    <Skeleton className="h-6 w-14 rounded-[4px]" />
                    <Skeleton className="h-6 w-16 rounded-[4px]" />
                  </>
                ) : actions.length ? (
                  actions.map((a) => (
                    <span key={a} className="rounded-[4px] border border-border-strong px-1.5 text-sm leading-6">
                      {humanize(a)}
                    </span>
                  ))
                ) : (
                  <span className="text-faint">–</span>
                )}
              </Cell>
              <Cell label="Decisions" className="readout text-muted-foreground lg:flex lg:justify-end">
                {stats.isPending ? <Skeleton className="h-4 w-8" /> : s ? count(s.by_class[c.key]) : '–'}
              </Cell>
            </div>
          )
        })}
      </div>
    </section>
  )
}

/** A cell of the table; on phones its column's name stands before it. */
function Cell({ label, className, children }: { label: string; className?: string; children: ReactNode }) {
  return (
    <span role="cell" className="grid grid-cols-[7rem_minmax(0,1fr)] items-center gap-3 lg:block">
      <span className="text-xs text-muted-foreground lg:hidden">
        {label}
      </span>
      <span className={className}>{children}</span>
    </span>
  )
}

import { WifiSlash } from '@phosphor-icons/react'
import type { ReactNode } from 'react'

import type { Correction, DecisionView } from '@/api/types'
import { Led } from '@/components/led'
import { humanize, ledOf, modelName, type LedColor } from '@/lib/decision'
import { clockTime, duration, percent } from '@/lib/format'
import { cn } from '@/lib/utils'

/**
 * A sync this much later than the step before means the device had no
 * network: the same window after which the server counts it offline.
 */
const OFFLINE_GAP_MS = 15_000

interface Step {
  /** Epoch ms; null for a step still to come. */
  at: number | null
  /** The light at this step; none for a plain hand-over. */
  light?: LedColor
  title: string
  detail?: ReactNode
  /** The decision reached the server here, so a long wait before means no network. */
  sync?: boolean
}

const actionLight = (action: string | null) => ledOf({ status: 'executed', action })

/** The decision's path as it happened: the robot, the server, the cloud model and back. */
function stepsOf(d: DecisionView, corrections: Correction[]): Step[] {
  const t0 = Date.parse(d.decided_at)
  const local =
    d.local_model &&
    `${d.local_model}, ${d.local_confidence != null ? `${percent(d.local_confidence)} sure` : "didn't recognise it"}`
  const steps: Step[] = []

  if (d.source === 'device') {
    steps.push({
      at: t0,
      light: actionLight(d.action),
      title:
        d.action_class === 'reflex'
          ? `Reflex on the robot: ${humanize(d.action ?? '')}`
          : `Decided on the robot: ${humanize(d.action ?? '')}`,
      detail: [local, d.latency_ms != null && `in ${duration(d.latency_ms)}`].filter(Boolean).join(', '),
    })
  } else {
    steps.push({
      at: t0,
      light: 'amber',
      title: d.proposed_action
        ? `Robot proposed ${humanize(d.proposed_action)}, asked the cloud`
        : 'Robot asked the cloud',
      detail: local || undefined,
    })
  }

  if (d.source === 'server') {
    steps.push({
      at: Date.parse(d.synced_at ?? d.updated_at),
      light: actionLight(d.action),
      title: `Cloud model answered: ${humanize(d.action ?? '')}`,
      sync: true,
      detail: (
        <>
          {[d.model && modelName(d.model), d.confidence != null && `${percent(d.confidence)} sure`, d.latency_ms != null && `in ${duration(d.latency_ms)}`]
            .filter(Boolean)
            .join(', ')}
          {d.reason && <q className="mt-1.5 block text-foreground/90 before:content-none after:content-none">{d.reason}</q>}
        </>
      ),
    })
  }
  if (d.status === 'waiting_for_server') steps.push({ at: null, light: 'amber', title: 'Waiting for the cloud model' })
  if (d.status === 'cancelled') {
    steps.push({ at: Date.parse(d.updated_at), light: 'off', title: 'Cancelled', detail: d.reason ?? undefined })
  }
  if (d.executed_at) {
    steps.push({ at: Date.parse(d.executed_at), light: actionLight(d.action), title: `Robot acted: ${humanize(d.action ?? '')}` })
  }
  if (d.source === 'device') {
    steps.push(
      d.synced_at
        ? { at: Date.parse(d.synced_at), title: 'Reached the server', sync: true }
        : { at: null, title: 'Not synced yet' },
    )
  }

  for (const c of corrections) {
    steps.push({
      at: Date.parse(c.created_at),
      light: 'white',
      title: `Cloud check: corrected to ${humanize(c.action)}`,
      detail: (
        <>
          {modelName(c.model)}
          <q className="mt-1.5 block text-foreground/90 before:content-none after:content-none">{c.reason}</q>
        </>
      ),
    })
    steps.push(
      c.applied_at
        ? { at: Date.parse(c.applied_at), light: actionLight(c.action), title: `Robot fixed it: ${humanize(c.action)}` }
        : { at: null, light: 'white', title: 'Waiting for the robot to fix it' },
    )
  }
  if (d.needs_review && corrections.length === 0) {
    steps.push(
      d.reviewed_at
        ? { at: Date.parse(d.reviewed_at), title: 'Cloud check agreed' }
        : { at: null, light: 'amber', title: 'Waiting for the cloud check' },
    )
  }

  const done = steps.filter((s) => s.at != null).sort((a, b) => a.at! - b.at!)
  return [...done, ...steps.filter((s) => s.at == null)]
}

export function Trace({ decision: d, corrections }: { decision: DecisionView; corrections: Correction[] }) {
  const t0 = Date.parse(d.decided_at)
  const steps = stepsOf(d, corrections)
  const offlineBefore = steps.map((s, i) => {
    const prev = steps[i - 1]
    return s.sync && prev?.at != null && s.at != null && s.at - prev.at > OFFLINE_GAP_MS ? s.at - prev.at : null
  })

  return (
    <ol className="grid">
      {steps.map((s, i) => {
        const last = i === steps.length - 1
        const offline = offlineBefore[i]
        return (
          <li key={i} className="relative grid grid-cols-[10px_minmax(0,1fr)_auto] gap-x-3 pb-4 last:pb-0">
            {!last && (
              <span
                aria-hidden
                className={cn(
                  'absolute top-4 bottom-0.5 left-[3.5px] w-px',
                  offlineBefore[i + 1] ? 'border-l border-dashed border-faint' : 'bg-border-strong',
                  steps[i + 1].at == null && 'border-l border-dotted border-faint bg-transparent',
                )}
              />
            )}
            <span className="flex h-5 items-center">
              {s.light ? (
                <Led color={s.light} />
              ) : (
                <span aria-hidden className="size-2 rounded-full border border-muted-foreground bg-card" />
              )}
            </span>
            <div className="grid min-w-0 gap-0.5 text-sm">
              {offline && (
                <p className="mb-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                  <WifiSlash className="size-4" aria-hidden />
                  Offline for {duration(offline)}, synced when the network came back
                </p>
              )}
              <p className={cn('leading-5', s.at == null && 'text-muted-foreground')}>{s.title}</p>
              {s.detail && <div className="text-xs leading-relaxed text-muted-foreground">{s.detail}</div>}
            </div>
            <span className="readout pt-0.5 text-xs text-muted-foreground">
              {s.at == null ? '' : i === 0 ? clockTime(d.decided_at) : `+${duration(s.at - t0)}`}
            </span>
          </li>
        )
      })}
    </ol>
  )
}

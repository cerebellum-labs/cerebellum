import { Crosshair } from '@phosphor-icons/react'

import type { Decision } from '@/api/types'
import { Led } from '@/components/led'
import { LIGHT, ledOf, STATUS_LABEL, type LedColor } from '@/lib/decision'
import { cn } from '@/lib/utils'

const ORDER: LedColor[] = ['green', 'blue', 'red', 'amber', 'white', 'off']

/** Shown before a decision is picked: how to read the journal. */
export function LightLegend() {
  return (
    <div className="panel grid gap-5 p-5">
      <div className="grid gap-2">
        <Crosshair className="size-5 text-faint" aria-hidden />
        <p className="text-sm text-muted-foreground">
          Pick a decision to see what the robot saw, who decided and how it travelled.
        </p>
        <p className="hidden text-xs text-muted-foreground lg:block">
          <Kbd>↑</Kbd> <Kbd>↓</Kbd> or <Kbd>J</Kbd> <Kbd>K</Kbd> move through the journal, <Kbd>Esc</Kbd> closes.
        </p>
      </div>
      <section aria-labelledby="lights-title" className="grid gap-3 border-t pt-4">
        <h2 id="lights-title" className="text-xs font-medium text-muted-foreground">
          The robot's lights
        </h2>
        <dl className="grid gap-2.5 text-sm">
          {ORDER.map((color) => (
            <div key={color} className="grid grid-cols-[10px_5.5rem_minmax(0,1fr)] items-center gap-3">
              <Led color={color} />
              <dt>{LIGHT[color].name}</dt>
              <dd className="text-muted-foreground">{LIGHT[color].meaning}</dd>
            </div>
          ))}
        </dl>
      </section>
    </div>
  )
}

/**
 * A decision's status with what its light meant, under the outcome where the
 * legend isn't in sight: "Decided on the robot · Green light: a fruit, put in
 * the basket".
 */
export function StatusLine({
  decision,
  className,
}: {
  decision: Pick<Decision, 'status' | 'action'>
  className?: string
}) {
  const light = LIGHT[ledOf(decision)]
  const text = `${STATUS_LABEL[decision.status]} · ${light.lit}: ${light.meaning.charAt(0).toLowerCase()}${light.meaning.slice(1)}`
  return (
    <p className={cn('text-sm text-muted-foreground', className)} title={text}>
      {text}
    </p>
  )
}

function Kbd({ children }: { children: string }) {
  return (
    <kbd className="readout inline-flex min-w-5 items-center justify-center rounded-[4px] border border-border-strong px-1 text-[11px] leading-4 text-foreground">
      {children}
    </kbd>
  )
}

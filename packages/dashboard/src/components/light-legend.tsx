import { Crosshair } from '@phosphor-icons/react'

import { Led } from '@/components/led'
import type { LedColor } from '@/lib/decision'

const LIGHTS: { color: LedColor; name: string; meaning: string }[] = [
  { color: 'green', name: 'Basket', meaning: 'A fruit, put in the basket' },
  { color: 'blue', name: 'Discard', meaning: 'Not a fruit, thrown out' },
  { color: 'red', name: 'Stop', meaning: 'A hand in the zone, the robot froze' },
  { color: 'amber', name: 'Waiting', meaning: 'Not sure, asked the cloud model' },
  { color: 'white', name: 'Corrected', meaning: 'The cloud model fixed a decision' },
  { color: 'off', name: 'Cancelled', meaning: 'The item left before the answer' },
]

/** Shown before a decision is picked: how to read the journal. */
export function LightLegend() {
  return (
    <div className="panel grid gap-5 p-5">
      <div className="grid gap-2">
        <Crosshair className="size-5 text-faint" aria-hidden />
        <p className="text-sm text-muted-foreground">
          Pick a decision to see what the robot saw, who decided and how it travelled.
        </p>
      </div>
      <section aria-labelledby="lights-title" className="grid gap-3 border-t pt-4">
        <h2 id="lights-title" className="text-xs font-medium text-muted-foreground">
          The robot's lights
        </h2>
        <dl className="grid gap-2.5 text-sm">
          {LIGHTS.map((light) => (
            <div key={light.color} className="grid grid-cols-[10px_5.5rem_minmax(0,1fr)] items-center gap-3">
              <Led color={light.color} />
              <dt>{light.name}</dt>
              <dd className="text-muted-foreground">{light.meaning}</dd>
            </div>
          ))}
        </dl>
      </section>
    </div>
  )
}

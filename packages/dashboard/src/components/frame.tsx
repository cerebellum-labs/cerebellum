import { ImageBroken } from '@phosphor-icons/react'

import { api } from '@/api/client'
import { cn } from '@/lib/utils'

/** Viewfinder corners over the inspected frame. */
function Corners() {
  const corner = 'absolute size-3 border-foreground/70'
  return (
    <span aria-hidden className="pointer-events-none absolute inset-2">
      <span className={cn(corner, 'top-0 left-0 border-t border-l')} />
      <span className={cn(corner, 'top-0 right-0 border-t border-r')} />
      <span className={cn(corner, 'bottom-0 left-0 border-b border-l')} />
      <span className={cn(corner, 'right-0 bottom-0 border-r border-b')} />
    </span>
  )
}

/** What the robot's camera saw when it made the decision. */
export function Frame({
  id,
  hasFrame,
  viewfinder,
  className,
}: {
  id: string
  hasFrame: boolean
  /** Frame the picture like a camera's viewfinder, for the large view. */
  viewfinder?: boolean
  className?: string
}) {
  return (
    <div
      className={cn(
        'relative aspect-[4/3] overflow-hidden rounded-md bg-muted ring-1 ring-border ring-inset',
        className,
      )}
    >
      {hasFrame ? (
        <img
          src={api.frameUrl(id)}
          alt="What the robot's camera saw"
          loading="lazy"
          className="size-full object-cover"
        />
      ) : (
        <span className="flex size-full items-center justify-center text-faint">
          <ImageBroken className="size-4" aria-label="No frame" />
        </span>
      )}
      {viewfinder && <Corners />}
    </div>
  )
}

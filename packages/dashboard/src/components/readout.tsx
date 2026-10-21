import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

/** One instrument reading: a plain label over a figure. */
export function Readout({
  label,
  children,
  hint,
  className,
}: {
  label: ReactNode
  children: ReactNode
  hint?: ReactNode
  className?: string
}) {
  return (
    <div className={cn('grid content-start gap-1.5', className)}>
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="readout text-xl leading-none text-foreground">{children}</dd>
      {hint && <dd className="text-xs text-muted-foreground">{hint}</dd>}
    </div>
  )
}

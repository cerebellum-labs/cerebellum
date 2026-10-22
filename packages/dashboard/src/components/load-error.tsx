import { ArrowClockwise } from '@phosphor-icons/react'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

/**
 * What a block shows when its data didn't load: what failed, what to check,
 * and a way to try again. The rest of the page keeps working.
 */
export function LoadError({
  what,
  onRetry,
  compact,
  className,
}: {
  /** What failed to load, as a sentence subject: "The latest decision". */
  what: string
  onRetry: () => void
  /** One line with an inline retry, for a small slot inside a block. */
  compact?: boolean
  className?: string
}) {
  if (compact) {
    return (
      <p role="alert" className={cn('flex flex-wrap items-center gap-x-2 text-sm text-muted-foreground', className)}>
        {what} didn't load.
        <button
          type="button"
          onClick={onRetry}
          className="font-medium text-foreground underline underline-offset-4 hover:text-brand"
        >
          Try again
        </button>
      </p>
    )
  }
  return (
    <div role="alert" className={cn('grid justify-items-start gap-1 text-sm', className)}>
      <p className="font-medium">{what} didn't load.</p>
      <p className="text-muted-foreground">Check that the server is reachable, then try again.</p>
      <Button variant="outline" size="sm" className="mt-3" onClick={onRetry}>
        <ArrowClockwise /> Try again
      </Button>
    </div>
  )
}

import { ArrowRight } from '@phosphor-icons/react'

import type { Decision } from '@/api/types'
import { humanize } from '@/lib/decision'
import { cn } from '@/lib/utils'

// The row's own LED already shows the final action; the text stays plain.
function Action({ action, quiet }: { action: string; quiet?: boolean }) {
  return <span className={cn(quiet && 'text-muted-foreground')}>{humanize(action)}</span>
}

/**
 * What the robot wanted and what it did: "Discard → Basket" when the server
 * changed the robot's mind, "~~Basket~~ → Discard" when a later check
 * corrected it, or one action when neither happened.
 */
export function DecisionOutcome({
  decision,
  className,
}: {
  decision: Decision & { corrected_action?: string | null }
  className?: string
}) {
  const { proposed_action: proposed, action, status, corrected_action: corrected } = decision

  if (status === 'corrected' && action && corrected) {
    return (
      <span className={cn('inline-flex items-center gap-2', className)}>
        <span className="text-muted-foreground line-through decoration-muted-foreground/60">{humanize(action)}</span>
        <ArrowRight className="size-4 text-muted-foreground" aria-label="corrected to" />
        <Action action={corrected} />
      </span>
    )
  }

  if (status === 'waiting_for_server') {
    return (
      <span className={cn('inline-flex items-center gap-2', className)}>
        {proposed ? <Action action={proposed} quiet /> : 'No proposal'}
        <span className="text-muted-foreground">waiting for the server</span>
      </span>
    )
  }
  if (status === 'cancelled') {
    return (
      <span className={cn('inline-flex items-center gap-2 text-muted-foreground', className)}>
        {proposed ? <span className="line-through">{humanize(proposed)}</span> : null}
        cancelled
      </span>
    )
  }
  if (!action) return <span className={className}>No action</span>
  if (proposed && proposed !== action) {
    return (
      <span className={cn('inline-flex items-center gap-2', className)}>
        <Action action={proposed} quiet />
        <ArrowRight className="size-4 text-muted-foreground" aria-label="changed to" />
        <Action action={action} />
      </span>
    )
  }
  return <Action action={action} />
}

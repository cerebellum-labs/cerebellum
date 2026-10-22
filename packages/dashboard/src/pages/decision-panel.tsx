import { X } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'

import { api, ApiError } from '@/api/client'
import { DecisionOutcome } from '@/components/decision-outcome'
import { Trace } from '@/components/decision-trace'
import { Frame } from '@/components/frame'
import { Led } from '@/components/led'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { CLASS_LABEL, humanize, ledOf, STATUS_LABEL } from '@/lib/decision'
import { clockTime, count, fullTime, usd } from '@/lib/format'
import { cn } from '@/lib/utils'

/**
 * Everything about one decision: the frame, the path it took between the
 * robot and the cloud, and the details. `bare` drops the panel's own frame
 * for use inside a sheet.
 */
export function DecisionPanel({ id, bare, onClose }: { id: string; bare?: boolean; onClose?: () => void }) {
  const query = useQuery({ queryKey: ['decision', id], queryFn: () => api.getDecision(id) })

  const box = cn(!bare && 'panel max-h-[calc(100svh-6rem)] overflow-y-auto')

  if (query.isPending) {
    return (
      <div className={cn(box, 'grid gap-4 p-4')} aria-busy aria-label="Loading the decision">
        <Skeleton className="aspect-[4/3] w-full" />
        <Skeleton className="h-5 w-48" />
        <Skeleton className="h-40 w-full" />
      </div>
    )
  }
  if (query.isError) {
    const missing = query.error instanceof ApiError && query.error.status === 404
    return (
      <div className={cn(box, 'grid gap-1 p-4 text-sm')}>
        <p className="font-medium">{missing ? 'This decision no longer exists.' : "The decision didn't load."}</p>
        {!missing && <p className="text-muted-foreground">Check that the server is reachable, then try again.</p>}
      </div>
    )
  }

  const { decision: d, corrections } = query.data
  return (
    <article className={cn(box, 'animate-in duration-200 fade-in-0')} aria-labelledby={`decision-${d.id}`}>
      <div className="sticky top-0 z-10 flex h-11 items-center justify-between border-b bg-card/90 px-4 backdrop-blur">
        <p className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
          <span className="truncate text-foreground">{d.device_name}</span>
          <span className="readout">{clockTime(d.decided_at)}</span>
        </p>
        {onClose && (
          <Button variant="ghost" size="icon-sm" onClick={onClose} aria-label="Close">
            <X />
          </Button>
        )}
      </div>

      <div className="grid gap-5 p-4">
        <Frame id={d.id} hasFrame={d.has_frame} viewfinder className="w-full" />

        <header className="grid gap-1.5">
          <h2 id={`decision-${d.id}`} className="flex items-center gap-2.5 text-xl font-semibold tracking-tight">
            <Led color={ledOf(d)} className="size-3" />
            <DecisionOutcome decision={d} />
          </h2>
          <p className="text-sm text-muted-foreground">{STATUS_LABEL[d.status]}</p>
        </header>

        <Section title="How it was decided" fields={false}>
          <Trace decision={d} corrections={corrections} />
        </Section>

        <Section title="Details">
          <Field label="Task">{humanize(d.task)}</Field>
          <Field label="Options">
            <span className="flex flex-wrap gap-1">
              {d.options.map((o) => (
                <span key={o} className="rounded-sm border border-border-strong px-1.5 text-xs leading-5">
                  {humanize(o)}
                </span>
              ))}
            </span>
          </Field>
          <Field label="Action class">{CLASS_LABEL[d.action_class]}</Field>
          <Field label="Decided at" mono>
            {fullTime(d.decided_at)}
          </Field>
        </Section>

        {(d.input_tokens != null || (d.cost_usd ?? 0) > 0) && (
          <Section title="Cost">
            {d.input_tokens != null && (
              <Field label="Tokens in" mono>
                {count(d.input_tokens)}
              </Field>
            )}
            {d.output_tokens != null && (
              <Field label="Tokens out" mono>
                {count(d.output_tokens)}
              </Field>
            )}
            <Field label="Cost" mono>
              {usd(d.cost_usd ?? 0)}
            </Field>
          </Section>
        )}

        {d.prompt && (
          <Section title="Question for the model" fields={false}>
            <p className="text-sm leading-relaxed text-muted-foreground">{d.prompt}</p>
          </Section>
        )}

        {Object.keys(d.metadata).length > 0 && (
          <details className="group border-t pt-4">
            <summary className="cursor-pointer text-xs font-medium text-muted-foreground transition-colors hover:text-foreground">
              Metadata from the robot
            </summary>
            <pre className="readout mt-2.5 overflow-x-auto rounded-md border bg-background p-3 text-xs leading-relaxed text-muted-foreground">
              {JSON.stringify(d.metadata, null, 2)}
            </pre>
          </details>
        )}
      </div>
    </article>
  )
}

/** A titled block; its children are Fields unless `fields` is false. */
function Section({ title, fields = true, children }: { title: string; fields?: boolean; children: ReactNode }) {
  return (
    <section className="grid gap-3 border-t pt-4">
      <h3 className="text-xs font-medium text-muted-foreground">{title}</h3>
      {fields ? (
        <dl className="grid grid-cols-[minmax(7.5rem,auto)_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm">{children}</dl>
      ) : (
        children
      )}
    </section>
  )
}

function Field({ label, mono, children }: { label: string; mono?: boolean; children: ReactNode }) {
  return (
    <>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className={cn('min-w-0', mono && 'readout')}>{children}</dd>
    </>
  )
}

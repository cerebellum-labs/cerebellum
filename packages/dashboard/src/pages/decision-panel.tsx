import { ArrowSquareOut, X } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'

import { api, ApiError } from '@/api/client'
import type { ModelCall } from '@/api/types'
import { DecisionOutcome } from '@/components/decision-outcome'
import { Trace, TraceSkeleton } from '@/components/decision-trace'
import { Frame } from '@/components/frame'
import { Led } from '@/components/led'
import { StatusLine } from '@/components/light-legend'
import { LoadError } from '@/components/load-error'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { CLASS_LABEL, humanize, ledOf, modelName } from '@/lib/decision'
import { clockTime, count, dayName, duration, fullTime, usd } from '@/lib/format'
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
      <div className={cn(box, 'grid gap-5 p-4')} aria-busy aria-label="Loading the decision">
        <Skeleton className="aspect-[4/3] w-full" />
        <div className="grid gap-2">
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-3.5 w-28" />
        </div>
        <TraceSkeleton />
        <div className="grid gap-3 border-t pt-4">
          {['w-24', 'w-40', 'w-28', 'w-36'].map((width) => (
            <div key={width} className="flex justify-between gap-4">
              <Skeleton className="h-3.5 w-20" />
              <Skeleton className={cn('h-3.5', width)} />
            </div>
          ))}
        </div>
      </div>
    )
  }
  if (query.isError) {
    const missing = query.error instanceof ApiError && query.error.status === 404
    return missing ? (
      <div className={cn(box, 'p-4 text-sm font-medium')}>This decision no longer exists.</div>
    ) : (
      <LoadError what="The decision" onRetry={() => query.refetch()} className={cn(box, 'p-4')} />
    )
  }

  const { decision: d, corrections, calls } = query.data
  const recordUrl = api.decisionUrl(d.id)
  return (
    <article className={cn(box, 'animate-in duration-200 fade-in-0')} aria-labelledby={`decision-${d.id}`}>
      <div className="sticky top-0 z-10 flex h-11 items-center justify-between border-b bg-card/90 px-4 backdrop-blur">
        <p className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
          <span className="truncate text-foreground">{d.device_name}</span>
          {dayName(d.decided_at) !== 'Today' && <span>{dayName(d.decided_at)}</span>}
          <span className="readout">{clockTime(d.decided_at)}</span>
        </p>
        {onClose && (
          <Button variant="ghost" size="icon-sm" onClick={onClose} aria-label="Close" className="max-lg:size-10">
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
          <StatusLine decision={d} />
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

        {(d.input_tokens != null || (d.cloud_cost_usd ?? d.cost_usd ?? 0) > 0) && (
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
            {d.cost_usd != null && (
              <Field label="Answer" mono>
                {usd(d.cost_usd)}
              </Field>
            )}
            {/* Retries, failed calls and the review come on top of the answer. */}
            {d.cloud_cost_usd != null && d.cloud_cost_usd !== (d.cost_usd ?? 0) && (
              <Field label={d.cost_usd == null ? 'Cloud check' : 'All cloud calls'} mono>
                {usd(d.cloud_cost_usd)}
              </Field>
            )}
          </Section>
        )}

        {d.prompt && (
          <Section title="Question for the model" fields={false}>
            <p className="text-sm leading-relaxed text-muted-foreground">{d.prompt}</p>
          </Section>
        )}

        {calls && calls.length > 0 && (
          <Section title="Cloud model log" fields={false}>
            <ol className="grid gap-4">
              {calls.map((c) => (
                <li key={c.id} className="grid gap-1.5 text-xs">
                  <p className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
                    <span className="text-sm">{CALL_LABEL[c.purpose]}</span>
                    <span className="readout text-muted-foreground">
                      {clockTime(c.created_at)} · {duration(c.latency_ms)} · {usd(c.cost_usd)}
                    </span>
                  </p>
                  <p className="text-muted-foreground">
                    {modelName(c.model)}
                    {c.input_tokens != null && `, ${count(c.input_tokens)} tokens in, ${count(c.output_tokens ?? 0)} out`}
                  </p>
                  {/* The reply exactly as the model sent it, or why the call failed. */}
                  <pre
                    className={cn(
                      'readout rounded-md border bg-background p-2.5 leading-relaxed break-words whitespace-pre-wrap',
                      c.error ? 'text-destructive' : 'text-muted-foreground',
                    )}
                  >
                    {c.error ?? pretty(c.output)}
                  </pre>
                </li>
              ))}
            </ol>
          </Section>
        )}

        {/* For anyone checking the data is real: the record as the API sends
            it, and links to open it and the frame straight from the API. */}
        <details className="group border-t pt-4">
          <summary className="cursor-pointer text-xs font-medium text-muted-foreground transition-colors hover:text-foreground">
            Raw record from the API
          </summary>
          <div className="mt-2.5 grid gap-3">
            {recordUrl && (
              <p className="flex flex-wrap gap-x-4 gap-y-1 text-xs">
                <a href={recordUrl} target="_blank" rel="noreferrer" className={LINK}>
                  Open as JSON <ArrowSquareOut className="size-3.5" aria-hidden />
                </a>
                {d.has_frame && (
                  <a href={api.frameUrl(d.id)} target="_blank" rel="noreferrer" className={LINK}>
                    Open the frame <ArrowSquareOut className="size-3.5" aria-hidden />
                  </a>
                )}
              </p>
            )}
            <pre className="readout overflow-x-auto rounded-md border bg-background p-3 text-xs leading-relaxed text-muted-foreground">
              {JSON.stringify(d, null, 2)}
            </pre>
          </div>
        </details>
      </div>
    </article>
  )
}

/** A titled block; its children are Fields unless `fields` is false. */
const CALL_LABEL: Record<ModelCall['purpose'], string> = {
  decide: 'Asked to decide',
  review: "Asked to check the robot's call",
}

const LINK = 'inline-flex items-center gap-1 font-medium underline-offset-4 hover:text-brand hover:underline'

/** A JSON reply laid out to read; anything else as it came. */
function pretty(output: string | null): string {
  if (!output) return 'No reply'
  try {
    return JSON.stringify(JSON.parse(output), null, 2)
  } catch {
    return output
  }
}

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

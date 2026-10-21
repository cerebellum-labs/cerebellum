import { ArrowRight } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'

import { api } from '@/api/client'
import { ActionClasses } from '@/components/action-classes'
import { DecisionOutcome } from '@/components/decision-outcome'
import { Trace } from '@/components/decision-trace'
import { Frame } from '@/components/frame'
import { Led } from '@/components/led'
import { SystemMap } from '@/components/system-map'
import { Skeleton } from '@/components/ui/skeleton'
import { useFigures } from '@/hooks/use-figures'
import { useNow } from '@/hooks/use-now'
import { useRecentDecisions } from '@/hooks/use-recent-decisions'
import { ledOf, STATUS_LABEL } from '@/lib/decision'
import { count, percent, timeAgo } from '@/lib/format'

/**
 * The front page: what Cerebellum is, a real decision as it happens, the
 * system running live and the rules it follows.
 */
export function OverviewPage() {
  return (
    <div className="grid gap-16 sm:gap-20">
      <header className="grid items-start gap-10 pt-2 sm:pt-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,26rem)] lg:gap-12 xl:grid-cols-[minmax(0,1fr)_minmax(0,28rem)] xl:gap-16">
        <div className="grid content-start gap-6 lg:pt-6">
          <h1 className="max-w-[18ch] text-4xl leading-[1.05] font-semibold tracking-tighter text-balance sm:text-5xl xl:text-6xl">
            Robots that decide on their own and ask the cloud when it matters
          </h1>
          <p className="max-w-[52ch] text-lg leading-relaxed text-pretty text-muted-foreground">
            Cerebellum syncs a robot's local model with a cloud model, and keeps the robot working offline.
          </p>
          <Figures />
          <Link
            to="/journal"
            className="flex w-fit items-center gap-2 border-b border-brand pb-0.5 font-medium transition-colors hover:text-brand"
          >
            Open the journal <ArrowRight className="size-4" aria-hidden />
          </Link>
        </div>
        <LatestDecision />
      </header>

      <SystemMap />
      <ActionClasses />
    </div>
  )
}

/** What it gains over asking the cloud every time, said in one sentence. */
function Figures() {
  const f = useFigures()
  if (f === undefined) return <Skeleton className="h-12 w-full max-w-[52ch]" />
  if (f === null) return null
  const n = (value: string) => <span className="readout font-medium text-foreground">{value}</span>
  return (
    <p className="max-w-[52ch] text-base leading-relaxed text-pretty text-muted-foreground">
      Over the last {n(count(f.decided))} decisions it answered {n(`${f.faster.toFixed(1)}×`)} faster and spent{' '}
      {n(percent(f.saved))} less than asking the cloud every time, and caught {n(count(f.mistakes))} of the robot's
      mistakes.
    </p>
  )
}

/** The newest decision with its frame and the path it took. Live events swap it as they come. */
function LatestDecision() {
  const recent = useRecentDecisions()
  const now = useNow(5_000)
  const latest = recent.data?.[0]
  const detail = useQuery({
    queryKey: ['decision', latest?.id],
    queryFn: () => api.getDecision(latest!.id),
    enabled: Boolean(latest),
  })

  if (recent.isPending) return <Skeleton className="h-[34rem] w-full rounded-lg" />
  if (!latest) {
    return (
      <section className="panel grid gap-1 p-5 text-sm">
        <p className="font-medium">No decisions yet.</p>
        <p className="text-muted-foreground">The newest one shows up here as soon as a robot reports it.</p>
      </section>
    )
  }

  return (
    <section aria-labelledby="latest-title" className="panel overflow-hidden">
      <div className="flex h-12 items-center justify-between gap-3 border-b px-4">
        <h2 id="latest-title" className="text-sm font-medium">
          Latest decision
        </h2>
        <p className="truncate text-sm text-muted-foreground">
          {latest.device_name}, {timeAgo(latest.decided_at, now)}
        </p>
      </div>
      <div key={latest.id} className="grid animate-in gap-5 p-4 duration-300 fade-in-0">
        <Link to={`/journal?decision=${latest.id}`} aria-label="Open this decision in the journal">
          <Frame id={latest.id} hasFrame={latest.has_frame} viewfinder className="w-full" />
        </Link>
        <div className="grid gap-1">
          <p className="flex items-center gap-2.5 text-xl font-semibold tracking-tight">
            <Led color={ledOf(latest)} className="size-3" />
            <DecisionOutcome decision={latest} />
          </p>
          <p className="text-sm text-muted-foreground">{STATUS_LABEL[latest.status]}</p>
        </div>
        {detail.data ? (
          <Trace decision={detail.data.decision} corrections={detail.data.corrections} />
        ) : (
          <Skeleton className="h-28 w-full" />
        )}
      </div>
    </section>
  )
}

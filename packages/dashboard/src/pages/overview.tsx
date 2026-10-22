import { ArrowRight, Cloud, Cpu, GithubLogo, WifiSlash } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router'

import { api } from '@/api/client'
import type { Decision } from '@/api/types'
import { ActionClasses } from '@/components/action-classes'
import { DecisionOutcome } from '@/components/decision-outcome'
import { Trace, TraceSkeleton } from '@/components/decision-trace'
import { Frame } from '@/components/frame'
import { Led } from '@/components/led'
import { LoadError } from '@/components/load-error'
import { SystemMap } from '@/components/system-map'
import { Skeleton } from '@/components/ui/skeleton'
import { useFigures } from '@/hooks/use-figures'
import { useNow } from '@/hooks/use-now'
import { useRecentDecisions } from '@/hooks/use-recent-decisions'
import { CLOUD_PROVIDER, LIGHT, ledOf, STATUS_LABEL } from '@/lib/decision'
import { count, duration, timeAgo } from '@/lib/format'
import { cn } from '@/lib/utils'

/**
 * The front page: what Cerebellum is, a real decision as it happens, the
 * system running live and the rules it follows.
 */
export function OverviewPage() {
  return (
    <div className="grid gap-14 sm:gap-16">
      <header className="grid items-start gap-10 pt-2 sm:pt-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,26rem)] lg:gap-12 xl:grid-cols-[minmax(0,1fr)_minmax(0,28rem)] xl:gap-16">
        <div className="grid max-w-[36rem] content-start gap-7 lg:pt-4">
          {/* Named after the part of the brain that runs quick moves without
              asking the cortex: the headline says that in plain words. */}
          <div className="grid gap-3">
            <p className="text-sm font-medium text-brand">Open-source layer for robots</p>
            <h1 className="max-w-[18ch] text-3xl leading-[1.08] font-semibold tracking-tighter text-balance sm:text-4xl xl:text-5xl">
              Fast reflexes on the robot. A big brain in the cloud.
            </h1>
            {/* The problem, so the three lines below read as its answer. */}
            <p className="text-base leading-relaxed text-pretty text-muted-foreground sm:text-lg">
              A robot can't wait two seconds for a cloud model on every move, or freeze when the Wi‑Fi drops.
            </p>
          </div>
          <HowItWorks />
          <Figures />
          {/* Watch it work first; for builders, the code is one step away. */}
          <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
            <Link
              to="/journal"
              className="flex w-fit items-center gap-2 border-b border-brand pb-0.5 font-medium transition-colors hover:text-brand"
            >
              See every decision live <ArrowRight className="size-4" aria-hidden />
            </Link>
            <a
              href={SOURCE_URL}
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
            >
              <GithubLogo className="size-4" aria-hidden /> Get the code
            </a>
          </div>
        </div>
        <LatestDecision />
      </header>

      <SystemMap />
      <ActionClasses />
      <OpenSource />
    </div>
  )
}

const SOURCE_URL = 'https://github.com/cerebellum-labs/cerebellum'

/** The page's last word: what a visitor can do with it next. */
function OpenSource() {
  return (
    <footer className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 border-t pt-6 text-sm text-muted-foreground">
      <p>Cerebellum is open source under the MIT license. Put the agent on your own robot.</p>
      <a
        href={SOURCE_URL}
        target="_blank"
        rel="noreferrer"
        className="flex items-center gap-2 font-medium text-foreground transition-colors hover:text-brand"
      >
        <GithubLogo className="size-4" aria-hidden /> Get the code on GitHub
        <ArrowRight className="size-4" aria-hidden />
      </a>
    </footer>
  )
}

// What Cerebellum gives a robot, in three lines: the outcome first, then how.
// The icons are the ones the live system uses for the same parts.
const HOW: { icon: ReactNode; title: string; text: string }[] = [
  { icon: <Cpu />, title: 'Acts in milliseconds', text: "with a small model on the robot for what it's sure of" },
  {
    icon: <Cloud />,
    title: `Asks Nemotron on ${CLOUD_PROVIDER}`,
    text: "before anything that can't be undone, and to double-check unsure calls",
  },
  { icon: <WifiSlash />, title: 'Keeps working offline', text: 'and syncs everything once the network is back' },
]

function HowItWorks() {
  return (
    <ul aria-label="How it works" className="grid gap-2.5">
      {HOW.map((step) => (
        <li key={step.title} className="flex items-baseline gap-3 text-base">
          <span aria-hidden className="translate-y-0.5 text-brand [&>svg]:size-4">
            {step.icon}
          </span>
          <span>
            <span className="font-medium">{step.title}</span>{' '}
            <span className="text-muted-foreground">{step.text}</span>
          </span>
        </li>
      ))}
    </ul>
  )
}

/**
 * What Cerebellum gains over asking the cloud every time, side by side, from
 * the demo robot's latest decisions: the contrast reads at a glance.
 */
function Figures() {
  const recent = useRecentDecisions()
  const f = useFigures()
  // The latest decision next to it says what failed.
  if (recent.isError) return null
  if (f === undefined) {
    return (
      <div className="grid gap-3 py-1" aria-busy aria-label="Loading the comparison">
        <Skeleton className="h-3 w-2/3" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-full" />
      </div>
    )
  }
  if (f === null) return null
  const perThousand = (total: number) => `$${((total / f.decided) * 1000).toFixed(2)}`
  // Figures in the readout face; the last row is words.
  const rows: { label: string; cloud: string; ours: string; figure: boolean }[] = [
    { label: 'Typical answer', cloud: duration(f.cloudTypical), ours: duration(f.typical), figure: true },
    {
      label: 'Cost of 1,000 decisions',
      cloud: perThousand(f.cloudOnly),
      ours: perThousand(f.spent),
      figure: true,
    },
    { label: 'When Wi\u2011Fi drops', cloud: 'Robot stops', ours: 'Keeps working', figure: false },
  ]
  return (
    <figure className="grid gap-2.5">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-xs text-muted-foreground">
            <th scope="col" className="pb-2 text-left font-normal">
              <span className="sr-only">Measure</span>
            </th>
            <th scope="col" className="pb-2 pl-4 text-right font-normal">
              Cloud only
            </th>
            <th scope="col" className="pb-2 pl-4 text-right font-medium text-brand">
              Cerebellum
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.label} className="border-t">
              <th scope="row" className="py-2 text-left font-normal text-muted-foreground">
                {row.label}
              </th>
              <td className={cn('py-2 pl-4 text-right whitespace-nowrap text-muted-foreground', row.figure && 'readout')}>
                {row.cloud}
              </td>
              <td className={cn('py-2 pl-4 text-right font-medium whitespace-nowrap', row.figure && 'readout')}>
                {row.ours}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <figcaption className="text-xs text-muted-foreground">
        Live from the demo robot's last {count(f.decided)} decisions
        {f.mistakes ? (
          <>
            , where the cloud{' '}
            <Link
              to="/journal?show=changed"
              className="underline decoration-border-strong underline-offset-4 transition-colors hover:text-foreground hover:decoration-brand"
            >
              caught {count(f.mistakes)} of its {f.mistakes === 1 ? 'mistake' : 'mistakes'}
            </Link>
          </>
        ) : null}
        .
      </figcaption>
    </figure>
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

  if (recent.isPending) return <LatestDecisionSkeleton />
  if (recent.isError) {
    return <LoadError what="The latest decision" onRetry={() => recent.refetch()} className="panel p-5" />
  }
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
        <h2 id="latest-title" className="shrink-0 text-sm font-medium">
          Live from the demo robot
        </h2>
        <p className="truncate text-sm text-muted-foreground">
          {latest.device_name}, {timeAgo(latest.decided_at, now)}
        </p>
      </div>
      {/* The frame sets the top row's height and the path gets a fixed one,
          so the panel keeps its size from one decision to the next and the
          live system below stays put. A longer path fades out at the
          bottom; the journal has all of it. */}
      <div key={latest.id} className="grid animate-in gap-4 p-4 duration-300 fade-in-0">
        <div className="grid grid-cols-[7.5rem_minmax(0,1fr)] items-center gap-4 sm:grid-cols-[10.5rem_minmax(0,1fr)]">
          <Link to={`/journal?decision=${latest.id}`} aria-label="Open this decision in the journal">
            <Frame id={latest.id} hasFrame={latest.has_frame} className="w-full" />
          </Link>
          <div className="grid min-w-0 gap-1">
            <p className="flex items-start gap-2 text-lg leading-snug font-semibold tracking-tight">
              <Led color={ledOf(latest)} className="mt-2" />
              <DecisionOutcome decision={latest} className="flex-wrap gap-x-2 gap-y-0" />
            </p>
            <p className="pl-[18px] text-sm text-muted-foreground">{subtitleOf(latest)}</p>
          </div>
        </div>
        <PathBox>
          {detail.data ? (
            <Trace decision={detail.data.decision} corrections={detail.data.corrections} brief />
          ) : detail.isError ? (
            <LoadError compact what="How it was decided" onRetry={() => detail.refetch()} />
          ) : (
            <TraceSkeleton />
          )}
        </PathBox>
      </div>
      <Link
        to={`/journal?decision=${latest.id}`}
        className="flex h-11 items-center justify-between border-t px-4 text-sm text-muted-foreground transition-colors hover:text-foreground"
      >
        The whole story in the journal
        <ArrowRight className="size-4" aria-hidden />
      </Link>
    </section>
  )
}

/**
 * The path's fixed-height slot. Room for a cloud decision's path; a longer
 * one, like a correction's, fades out at the bottom, and only then.
 */
function PathBox({ children }: { children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null)
  const [overflows, setOverflows] = useState(false)
  useEffect(() => {
    const box = ref.current
    const content = box?.firstElementChild
    if (!box || !content) return
    const check = () => setOverflows(box.scrollHeight > box.clientHeight + 1)
    const observer = new ResizeObserver(check)
    observer.observe(content)
    return () => observer.disconnect()
  }, [children])
  return (
    <div ref={ref} className={cn('h-36 overflow-hidden', overflows && 'mask-b-from-75%')}>
      {children}
    </div>
  )
}

/**
 * The line under the outcome. Where the outcome already says "asking the
 * cloud" or "cancelled", it says what the robot's light showed instead.
 */
function subtitleOf(d: Pick<Decision, 'status'>): string {
  if (d.status === 'waiting_for_server') return `${LIGHT.amber.lit}: not sure`
  if (d.status === 'cancelled') return `${LIGHT.off.lit}: gave up waiting`
  return STATUS_LABEL[d.status]
}

/** The latest decision's panel while it loads, in the same places. */
function LatestDecisionSkeleton() {
  return (
    <section className="panel overflow-hidden" aria-busy aria-label="Loading the latest decision">
      <div className="flex h-12 items-center justify-between gap-3 border-b px-4">
        <Skeleton className="h-3.5 w-28" />
        <Skeleton className="h-3.5 w-32" />
      </div>
      <div className="grid gap-4 p-4">
        <div className="grid grid-cols-[7.5rem_minmax(0,1fr)] items-center gap-4 sm:grid-cols-[10.5rem_minmax(0,1fr)]">
          <Skeleton className="aspect-[4/3] w-full" />
          <div className="grid gap-2">
            <Skeleton className="h-5 w-32" />
            <Skeleton className="h-3.5 w-24" />
          </div>
        </div>
        <div className="h-36">
          <TraceSkeleton />
        </div>
      </div>
      <div className="flex h-11 items-center border-t px-4">
        <Skeleton className="h-3.5 w-48" />
      </div>
    </section>
  )
}

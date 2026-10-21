import { Cloud, Cpu } from '@phosphor-icons/react'
import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { useSearchParams } from 'react-router'

import { api } from '@/api/client'
import type { DecisionStatus, DecisionView } from '@/api/types'
import { useLive } from '@/app/context'
import { DecisionOutcome } from '@/components/decision-outcome'
import { DeviceSelect } from '@/components/device-select'
import { Frame } from '@/components/frame'
import { Led } from '@/components/led'
import { LightLegend } from '@/components/light-legend'
import { PageHeader } from '@/components/page-header'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { useMediaQuery } from '@/hooks/use-media-query'
import { ledOf, modelName, SOURCE_LABEL, STATUSES, STATUS_LABEL } from '@/lib/decision'
import { clockTime, count, duration, percent, usd } from '@/lib/format'
import { cn } from '@/lib/utils'
import { DecisionPanel } from './decision-panel'

const ALL = 'all'

// One grid for the header and every row, sized by the table's own width (it
// shares the screen with the inspector). Narrow: the light, the frame, the
// outcome and the time. Medium adds who decided and how long it took; wide
// adds confidence and cost.
const COLUMNS = cn(
  'grid items-center gap-x-3 px-3 sm:px-4',
  'grid-cols-[10px_64px_minmax(0,1fr)_auto]',
  '@2xl:grid-cols-[10px_72px_64px_minmax(0,1fr)_minmax(0,11rem)_5rem] @2xl:gap-x-4',
  '@4xl:grid-cols-[10px_72px_72px_minmax(0,1fr)_minmax(0,12rem)_4rem_5rem_5.5rem]',
)

export function JournalPage() {
  const [params, setParams] = useSearchParams()
  const deviceId = params.get('device') ?? undefined
  const status = (params.get('status') as DecisionStatus | null) ?? undefined
  const selected = params.get('decision')
  const wide = useMediaQuery('(min-width: 1024px)')

  const devices = useQuery({ queryKey: ['devices'], queryFn: () => api.listDevices() })
  const journal = useInfiniteQuery({
    queryKey: ['decisions', { deviceId, status }],
    queryFn: ({ pageParam }) => api.listDecisions({ device_id: deviceId, status, before: pageParam ?? undefined }),
    initialPageParam: null as string | null,
    getNextPageParam: (page) => page.next_before,
  })
  const rows = journal.data?.pages.flatMap((page) => page.decisions) ?? []

  function setParam(key: string, value: string | null) {
    setParams(
      (current) => {
        const next = new URLSearchParams(current)
        if (value) next.set(key, value)
        else next.delete(key)
        return next
      },
      { replace: key === 'decision' },
    )
  }

  const filtered = Boolean(deviceId || status)
  const closeDecision = () => setParam('decision', null)

  let body: ReactNode
  if (journal.isPending) {
    body = <JournalSkeleton />
  } else if (journal.isError) {
    body = (
      <Notice title="The journal didn't load.">
        <p>Check that the server is reachable, then try again.</p>
        <Button variant="outline" size="sm" className="mt-4" onClick={() => journal.refetch()}>
          Try again
        </Button>
      </Notice>
    )
  } else if (rows.length === 0) {
    body = filtered ? (
      <Notice title="No decisions match these filters.">
        <Button variant="outline" size="sm" className="mt-3" onClick={() => setParams({})}>
          Show all decisions
        </Button>
      </Notice>
    ) : (
      <Notice title="No decisions yet.">
        <p>Start the agent on a device: its decisions appear here as soon as it reports one.</p>
      </Notice>
    )
  } else {
    body = (
      <>
        <div
          aria-hidden
          className={cn(
            COLUMNS,
            'sticky top-14 z-10 hidden h-9 border-b bg-card/90 text-xs text-muted-foreground backdrop-blur @2xl:grid',
          )}
        >
          <span />
          <span>Time</span>
          <span>Frame</span>
          <span>Outcome</span>
          <span>Decided</span>
          <span className="hidden text-right @4xl:block">Sure</span>
          <span className="text-right">Took</span>
          <span className="hidden text-right @4xl:block">Cost</span>
        </div>
        <ol className="divide-y">
          {rows.map((d) => (
            <DecisionRow
              key={d.id}
              decision={d}
              showDevice={!deviceId && (devices.data?.length ?? 0) > 1}
              selected={d.id === selected}
              onSelect={() => setParam('decision', d.id)}
            />
          ))}
        </ol>
        <div className="flex h-11 items-center justify-between border-t px-3 text-xs text-muted-foreground sm:px-4">
          <span className="readout">{count(rows.length)} shown</span>
          {journal.hasNextPage && (
            <Button
              variant="ghost"
              size="sm"
              disabled={journal.isFetchingNextPage}
              onClick={() => journal.fetchNextPage()}
            >
              {journal.isFetchingNextPage ? 'Loading…' : 'Show older'}
            </Button>
          )}
        </div>
      </>
    )
  }

  return (
    <div className="grid gap-6">
      <PageHeader
        id="journal-title"
        title="Journal"
        description="Every decision with its frame, who made it and how long it took."
      >
        <DeviceSelect value={deviceId} onChange={(id) => setParam('device', id)} />
      </PageHeader>


      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(360px,420px)]">
        <section aria-labelledby="decisions-title" className="panel @container min-w-0 overflow-clip">
          <div className="flex h-12 items-center gap-3 border-b px-3 sm:px-4">
            <h2 id="decisions-title" className="text-sm font-medium">
              Decisions
            </h2>
            <LiveBadge />
            <Select value={status ?? ALL} onValueChange={(v) => setParam('status', v === ALL ? null : v)}>
              <SelectTrigger size="sm" className="ml-auto w-44" aria-label="Status">
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" align="end">
                <SelectItem value={ALL}>All statuses</SelectItem>
                {STATUSES.map((s) => (
                  <SelectItem key={s} value={s}>
                    {STATUS_LABEL[s]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {body}
        </section>

        {wide ? (
          <aside aria-label="Selected decision" className="sticky top-[4.5rem]">
            {selected ? <DecisionPanel key={selected} id={selected} onClose={closeDecision} /> : <LightLegend />}
          </aside>
        ) : (
          <Sheet open={Boolean(selected)} onOpenChange={(open) => !open && closeDecision()}>
            <SheetContent
              showCloseButton={false}
              className="gap-0 overflow-y-auto bg-card p-0 data-[side=right]:w-full data-[side=right]:sm:max-w-md"
            >
              <SheetHeader className="sr-only">
                <SheetTitle>Decision</SheetTitle>
                <SheetDescription>What the robot saw, who decided and why.</SheetDescription>
              </SheetHeader>
              {selected && <DecisionPanel id={selected} bare onClose={closeDecision} />}
            </SheetContent>
          </Sheet>
        )}
      </div>
    </div>
  )
}

/** New decisions stream in while the page is open. */
function LiveBadge() {
  return (
    <span className="flex items-center gap-1.5 rounded-[4px] border border-border-strong px-2 py-0.5 text-xs text-muted-foreground">
      <span aria-hidden className="relative flex size-2">
        <span className="absolute inline-flex size-full animate-ping rounded-full bg-led-green opacity-70" />
        <span className="relative size-2 rounded-full bg-led-green" />
      </span>
      Live
    </span>
  )
}

function Notice({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="p-6 text-sm">
      <p className="font-medium">{title}</p>
      <div className="mt-1 text-muted-foreground">{children}</div>
    </div>
  )
}

function DecisionRow({
  decision: d,
  showDevice,
  selected,
  onSelect,
}: {
  decision: DecisionView
  showDevice: boolean
  selected: boolean
  onSelect: () => void
}) {
  const live = useLive()
  const added = live.added.has(d.id)
  const changed = live.changed.has(d.id)
  const dash = <span className="text-faint">–</span>
  const SourceIcon = d.source === 'server' ? Cloud : Cpu

  return (
    <li className={cn(added && 'animate-in duration-500 fade-in-0 slide-in-from-top-2')}>
      <button
        type="button"
        onClick={onSelect}
        aria-current={selected || undefined}
        className={cn(
          COLUMNS,
          'w-full py-2.5 text-left text-sm transition-colors duration-1000',
          'hover:bg-white/[0.03] focus-visible:relative focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring',
          changed && 'bg-white/[0.06] duration-0',
          selected && 'bg-brand/[0.08] shadow-[inset_3px_0_0_var(--brand)]',
        )}
      >
        <Led color={ledOf(d)} />
        <time
          dateTime={d.decided_at}
          className="readout order-last self-start pt-0.5 text-xs text-muted-foreground @2xl:order-none @2xl:self-center @2xl:pt-0"
        >
          {clockTime(d.decided_at)}
        </time>
        <Frame id={d.id} hasFrame={d.has_frame} className="w-full rounded-[5px]" />
        <span className="grid min-w-0 gap-0.5">
          <span className="truncate font-medium">
            <DecisionOutcome decision={d} />
          </span>
          <span className="flex min-w-0 gap-x-3 truncate text-xs text-muted-foreground">
            <span className={cn(d.status === 'waiting_for_server' && 'text-led-amber')}>{STATUS_LABEL[d.status]}</span>
            {showDevice && <span className="truncate">{d.device_name}</span>}
            {d.latency_ms != null && <span className="readout @2xl:hidden">{duration(d.latency_ms)}</span>}
          </span>
        </span>
        <span className="hidden min-w-0 items-start gap-2 @2xl:flex">
          {d.source ? (
            <>
              <SourceIcon className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
              <span className="grid min-w-0 gap-0.5">
                <span className="truncate">{SOURCE_LABEL[d.source]}</span>
                {d.model && (
                  <span className="truncate text-xs text-muted-foreground">{modelName(d.model)}</span>
                )}
              </span>
            </>
          ) : (
            dash
          )}
        </span>
        <span className="readout hidden text-right @4xl:block">
          {d.confidence != null ? percent(d.confidence) : dash}
        </span>
        <span className="readout hidden text-right @2xl:block">
          {d.latency_ms != null ? duration(d.latency_ms) : dash}
        </span>
        <span className="readout hidden text-right @4xl:block">{d.cost_usd ? usd(d.cost_usd) : dash}</span>
      </button>
    </li>
  )
}

function JournalSkeleton() {
  return (
    <div className="divide-y" aria-busy aria-label="Loading the journal">
      {Array.from({ length: 8 }, (_, i) => (
        <div key={i} className={cn(COLUMNS, 'py-2.5')}>
          <Skeleton className="size-2 rounded-full" />
          <Skeleton className="hidden h-3 w-14 @2xl:block" />
          <Skeleton className="aspect-[4/3] w-full" />
          <div className="grid gap-2">
            <Skeleton className="h-3.5 w-36" />
            <Skeleton className="h-3 w-52 max-w-full" />
          </div>
        </div>
      ))}
    </div>
  )
}

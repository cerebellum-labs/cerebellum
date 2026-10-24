import { Cloud, Cpu } from '@phosphor-icons/react'
import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { Fragment, useEffect, useRef, type ReactNode } from 'react'
import { useNavigate, useSearchParams } from 'react-router'

import { api, type Connection } from '@/api/client'
import type { DecisionQuery, DecisionStatus, DecisionView } from '@/api/types'
import { useLive } from '@/app/context'
import { DecisionOutcome } from '@/components/decision-outcome'
import { DeviceSelect } from '@/components/device-select'
import { Frame } from '@/components/frame'
import { Led } from '@/components/led'
import { LightLegend } from '@/components/light-legend'
import { LoadError } from '@/components/load-error'
import { PageHeader } from '@/components/page-header'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { useConnection } from '@/hooks/use-connection'
import { useMediaQuery } from '@/hooks/use-media-query'
import { usePageTitle } from '@/hooks/use-page-title'
import { LIGHT, ledOf, modelName, SOURCE_LABEL, STATUS_LABEL } from '@/lib/decision'
import { clockTime, count, dayName, duration, percent, usd } from '@/lib/format'
import { cn } from '@/lib/utils'
import { DecisionPanel } from './decision-panel'

const ALL = 'all'

/**
 * The journal's filter, by who decided and what came of it; `?show=` keeps
 * it in the link. "Changed" is every time the cloud disagreed with the robot.
 */
const SHOW = {
  device: { label: 'Decided on the robot', query: { source: 'device' } },
  server: { label: 'Decided in the cloud', query: { source: 'server' } },
  changed: { label: 'Changed by the cloud', query: { changed: true } },
  waiting: { label: 'Asking the cloud', query: { status: 'waiting_for_server' } },
  cancelled: { label: 'Cancelled', query: { status: 'cancelled' } },
} satisfies Record<string, { label: string; query: DecisionQuery }>

type Show = keyof typeof SHOW
const isShow = (value: string | null): value is Show => value != null && value in SHOW

// One grid for the header and every row, sized by the table's own width (it
// shares the screen with the inspector). Narrow: the light, the frame and the
// outcome, with the time under it. Medium adds who decided and how long it took; wide
// adds confidence and cost.
const COLUMNS = cn(
  'grid items-center gap-x-3 px-3 sm:px-4',
  'grid-cols-[10px_64px_minmax(0,1fr)] @sm:grid-cols-[10px_80px_minmax(0,1fr)]',
  '@2xl:grid-cols-[10px_72px_64px_minmax(0,1fr)_minmax(0,11rem)_5rem] @2xl:gap-x-4',
  '@4xl:grid-cols-[10px_72px_72px_minmax(0,1fr)_minmax(0,12rem)_4rem_5rem_5.5rem]',
)

export function JournalPage() {
  const [params, setParams] = useSearchParams()
  const deviceId = params.get('device') ?? undefined
  const showParam = params.get('show')
  const show = isShow(showParam) ? showParam : undefined
  const selected = params.get('decision')
  // The side panel needs room: below 1280px it would squeeze the list into its narrow layout.
  const wide = useMediaQuery('(min-width: 1280px)')
  usePageTitle('Decision journal')

  const devices = useQuery({ queryKey: ['devices'], queryFn: () => api.listDevices() })
  const journal = useInfiniteQuery({
    queryKey: ['decisions', { deviceId, show }],
    queryFn: ({ pageParam }) =>
      api.listDecisions({ ...(show && SHOW[show].query), device_id: deviceId, before: pageParam ?? undefined }),
    initialPageParam: null as string | null,
    getNextPageParam: (page) => page.next_before,
  })
  // New decisions show on top the moment they arrive: watching them come in
  // is the point of the journal.
  const rows = journal.data?.pages.flatMap((page) => page.decisions) ?? []

  function setParam(key: string, value: string | null, replace = false) {
    setParams(
      (current) => {
        const next = new URLSearchParams(current)
        if (value) next.set(key, value)
        else next.delete(key)
        return next
      },
      { replace },
    )
  }

  // Opening a decision from the list is a step Back undoes, so on a phone
  // Back closes the sheet instead of leaving the journal; moving between
  // decisions or closing one adds no steps of its own.
  const navigate = useNavigate()
  const pushed = useRef(false)
  useEffect(() => {
    if (!selected) pushed.current = false
  }, [selected])
  function openDecision(id: string) {
    if (!selected) pushed.current = true
    setParam('decision', id, Boolean(selected))
  }
  function closeDecision() {
    if (pushed.current) void navigate(-1)
    else setParam('decision', null, true)
  }

  const filtered = Boolean(deviceId || show)


  // ↑/↓ or j/k move through the journal, Esc closes the open decision.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey) return
      const target = e.target as HTMLElement
      if (target.closest('input, textarea, select, [role="combobox"], [role="listbox"], [role="dialog"], [contenteditable]')) {
        return
      }
      if (e.key === 'Escape' && selected) {
        closeDecision()
        return
      }
      const arrow = e.key === 'ArrowDown' || e.key === 'ArrowUp'
      // Arrows move through the list only from inside it; elsewhere they
      // scroll the page as usual. j and k work anywhere.
      if (arrow && !target.closest('[data-decisions]')) return
      const step = e.key === 'ArrowDown' || e.key === 'j' ? 1 : e.key === 'ArrowUp' || e.key === 'k' ? -1 : 0
      if (!step || rows.length === 0) return
      e.preventDefault()
      const at = rows.findIndex((d) => d.id === selected)
      const next = rows[at === -1 ? 0 : Math.min(Math.max(at + step, 0), rows.length - 1)]
      openDecision(next.id)
      const row = document.getElementById(rowId(next.id))
      row?.focus({ preventScroll: true })
      row?.scrollIntoView({ block: 'nearest' })
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  // A failed "Show older" puts the query in its error state with the
  // pages already loaded: those stay, with a retry below them.
  const olderFailed = journal.isFetchNextPageError
  const fetchOlder = () => void journal.fetchNextPage()

  let body: ReactNode
  if (journal.isPending) {
    body = <JournalSkeleton />
  } else if (journal.isError && !olderFailed) {
    body = <LoadError what="The journal" onRetry={() => journal.refetch()} className="p-6" />
  } else if (rows.length === 0) {
    body = filtered ? (
      <Notice title="No decisions match these filters.">
        <Button variant="outline" size="sm" className="mt-3" onClick={() => setParams({})}>
          Show all decisions
        </Button>
      </Notice>
    ) : (
      <Notice title="No decisions yet.">
        <p>Start the agent on a robot: its decisions appear here as soon as it reports one.</p>
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
          {rows.map((d, i) => {
            // A row only shows the time of day: a new day starts under a name.
            const dayOf = dayName(d.decided_at)
            const newDay = i === 0 || dayName(rows[i - 1].decided_at) !== dayOf
            return (
              <Fragment key={d.id}>
                {newDay && (
                  <li className="bg-background/40 px-3 py-1.5 text-xs font-medium text-muted-foreground sm:px-4">
                    {dayOf}
                  </li>
                )}
                <DecisionRow
                  decision={d}
                  showDevice={!deviceId && (devices.data?.length ?? 0) > 1}
                  selected={d.id === selected}
                  onSelect={() => openDecision(d.id)}
                />
              </Fragment>
            )
          })}
        </ol>
        <div className="flex min-h-11 flex-wrap items-center justify-between gap-x-3 border-t px-3 text-xs text-muted-foreground sm:px-4">
          <span className="readout">{count(rows.length)} shown</span>
          {olderFailed ? (
            <LoadError compact what="Older decisions" onRetry={fetchOlder} className="text-xs" />
          ) : journal.hasNextPage && (
            <Button
              variant="ghost"
              size="sm"
              disabled={journal.isFetchingNextPage}
              onClick={fetchOlder}
              className="max-sm:h-10"
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
        title="Decision journal"
        description="Every decision the demo robot made, live: what the camera saw, who decided and how long it took."
      >
        <DeviceSelect value={deviceId} onChange={(id) => setParam('device', id)} />
        <Select value={show ?? ALL} onValueChange={(v) => setParam('show', v === ALL ? null : v)}>
          <SelectTrigger className="w-full sm:w-52" aria-label="Show">
            <SelectValue />
          </SelectTrigger>
          <SelectContent position="popper" align="end">
            <SelectItem value={ALL}>All decisions</SelectItem>
            {(Object.keys(SHOW) as Show[]).map((key) => (
              <SelectItem key={key} value={key}>
                {SHOW[key].label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </PageHeader>

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(360px,420px)]">
        <section
          aria-labelledby="decisions-title"
          data-decisions
          className="panel @container min-w-0 overflow-clip"
        >
          <div className="flex h-12 items-center gap-3 border-b px-3 sm:px-4">
            <h2 id="decisions-title" className="text-sm font-medium">
              Decisions
            </h2>
            <LiveBadge />
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

const LIVE_LABEL: Record<Connection, string> = {
  open: 'Live',
  connecting: 'Connecting',
  reconnecting: 'Reconnecting',
}

/**
 * Whether new decisions stream in right now: green while the event stream is
 * open, amber while it opens or comes back after a break.
 */
function LiveBadge() {
  const connection = useConnection()
  const live = connection === 'open'
  return (
    <span
      role="status"
      title={live ? 'New decisions appear as they happen.' : 'New decisions appear once the connection is back.'}
      className="flex items-center gap-1.5 rounded-[4px] border border-border-strong px-2 py-0.5 text-xs text-muted-foreground"
    >
      <span
        aria-hidden
        className={cn('size-2 rounded-full', live ? 'bg-led-green' : 'animate-pulse bg-led-amber')}
      />
      {LIVE_LABEL[connection]}
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
  const light = ledOf(d)
  const sure = d.source === 'server' ? 'The cloud model' : "The robot's model"

  return (
    <li className={cn(added && 'animate-in duration-500 fade-in-0 slide-in-from-top-2')}>
      <button
        id={rowId(d.id)}
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
        <Led color={light} title={`${LIGHT[light].lit}: ${LIGHT[light].meaning}`} />
        <time dateTime={d.decided_at} className="readout hidden text-xs text-muted-foreground @2xl:block">
          {clockTime(d.decided_at)}
        </time>
        <Frame id={d.id} hasFrame={d.has_frame} decorative className="w-full rounded-[5px]" />
        <span className="grid min-w-0 gap-0.5">
          <span className="truncate font-medium">
            <DecisionOutcome decision={d} />
          </span>
          {/* Done is most decisions: only a status that says something more
              is written out. Narrow, the time and how long it took join it. */}
          <span className="flex min-w-0 gap-x-3 truncate text-xs text-muted-foreground">
            {/* Narrow, the Decided column is gone: an icon keeps who decided. */}
            {d.source && (
              <SourceIcon
                className="size-3.5 shrink-0 self-center @2xl:hidden"
                aria-label={SOURCE_LABEL[d.source]}
                role="img"
              />
            )}
            <span className="readout shrink-0 @2xl:hidden">{clockTime(d.decided_at)}</span>
            {d.latency_ms != null && <span className="readout shrink-0 @2xl:hidden">{duration(d.latency_ms)}</span>}
            {/* The outcome already says "asking the cloud" or "cancelled". */}
            {!QUIET_STATUSES.has(d.status) && <span className="truncate">{STATUS_LABEL[d.status]}</span>}
            {/* Too narrow for both: the device gives way, the filter has it. */}
            {showDevice && <span className="hidden truncate @sm:inline">{d.device_name}</span>}
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
        <span
          className="readout hidden text-right @4xl:block"
          title={d.confidence != null ? `${sure} was ${percent(d.confidence)} sure` : undefined}
        >
          <span className="sr-only">{sure} was </span>
          {d.confidence != null ? percent(d.confidence) : dash}
          <span className="sr-only"> sure</span>
        </span>
        <span className="readout hidden text-right @2xl:block">
          <span className="sr-only">Took </span>
          {d.latency_ms != null ? duration(d.latency_ms) : dash}
        </span>
        <span className="readout hidden text-right @4xl:block">
          {/* A decision on the robot costs nothing: left blank, not dashed. */}
          {d.cost_usd ? (
            <>
              <span className="sr-only">Cost </span>
              {usd(d.cost_usd)}
            </>
          ) : null}
        </span>
      </button>
    </li>
  )
}

/** Statuses a row doesn't write out: done is most decisions, the others its outcome says. */
const QUIET_STATUSES = new Set<DecisionStatus>(['executed', 'waiting_for_server', 'cancelled'])

const rowId = (id: string) => `decision-row-${id}`

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

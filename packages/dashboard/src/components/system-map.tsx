import {
  Brain,
  Cloud,
  HardDrives,
  LockSimple,
  Scissors,
  VideoCamera,
  WifiHigh,
  WifiSlash,
} from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { Link } from 'react-router'

import { api, simulator } from '@/api/client'
import type { DashboardEvent } from '@/api/types'
import { Led } from '@/components/led'
import { LoadError } from '@/components/load-error'
import { Mark } from '@/components/mark'
import { Readout } from '@/components/readout'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useNow } from '@/hooks/use-now'
import { useRecentDecisions } from '@/hooks/use-recent-decisions'
import { useRobot } from '@/hooks/use-robot'
import { CLOUD_PROVIDER, ledOf, modelName, type LedColor } from '@/lib/decision'
import { count, duration, percent, timeAgo, usd } from '@/lib/format'
import { cn } from '@/lib/utils'

/** Camera to agent on the robot, robot to server over the network, server to the cloud model. */
type WireName = 'local' | 'network' | 'cloud'

interface Comet {
  key: number
  wire: WireName
  /** Travels back towards the robot. */
  back: boolean
  light: LedColor
  delay: number
}

type Hop = Omit<Comet, 'key'>

/** Time between one wire and the next for the same decision. */
const HOP_MS = 650

const COLOR: Record<LedColor, string> = {
  green: 'var(--led-green)',
  blue: 'var(--led-blue)',
  amber: 'var(--led-amber)',
  red: 'var(--led-red)',
  white: 'var(--led-white)',
  off: '#6b6b6b',
}

/** Which wires a server event lights up, in the colour of the decision's light. */
function hopsOf(event: DashboardEvent, seen: Set<string>, local: boolean): Hop[] {
  if (event.type === 'correction.created') {
    return [
      { wire: 'cloud', back: true, light: 'white', delay: 0 },
      { wire: 'network', back: true, light: 'white', delay: HOP_MS },
    ]
  }
  if (event.type !== 'decision.upserted') return []
  const d = event.decision
  const first = !seen.has(d.id)
  seen.add(d.id)
  const light = ledOf(d)
  // Without a simulator nothing reports the robot's own step, so the first
  // news of a decision also lights the wire on the robot.
  const lead: Hop[] = first && !local ? [{ wire: 'local', back: false, light, delay: 0 }] : []
  const after = lead.length ? HOP_MS : 0

  switch (d.status) {
    case 'waiting_for_server':
      return [
        ...lead,
        { wire: 'network', back: false, light, delay: after },
        { wire: 'cloud', back: false, light, delay: after + HOP_MS },
      ]
    case 'decided_on_server':
      return [
        { wire: 'cloud', back: true, light, delay: 0 },
        { wire: 'network', back: true, light, delay: HOP_MS },
      ]
    case 'decided_on_device':
    case 'cancelled':
      return [...lead, { wire: 'network', back: false, light, delay: after }]
    case 'executed':
      // A decision that reaches the server already done; later updates of a
      // known decision travel nowhere new.
      return first ? [...lead, { wire: 'network', back: false, light, delay: after }] : []
    default:
      return []
  }
}

/**
 * Cerebellum drawn live: the robots with their camera, local model and the
 * agent; the server and the cloud model; the network in between. Every
 * decision travels across in the colour of the robot's light. With the demo's
 * simulator the network can be cut to show what happens offline.
 */
export function SystemMap() {
  const robot = useRobot()
  const now = useNow(1_000)
  const stats = useQuery({ queryKey: ['stats', null], queryFn: () => api.stats() })
  const devices = useQuery({ queryKey: ['devices'], queryFn: () => api.listDevices() })
  const recent = useRecentDecisions()

  const [comets, setComets] = useState<Comet[]>([])
  // The last sync after the network came back; it stays until the next cut.
  const [synced, setSynced] = useState<number | null>(null)
  const nextKey = useRef(0)

  useEffect(() => {
    const add = (hops: Hop[]) =>
      hops.length && setComets((current) => [...current, ...hops.map((hop) => ({ ...hop, key: nextKey.current++ }))])
    const seen = new Set<string>()
    const offServer = api.subscribe((event) => add(hopsOf(event, seen, Boolean(simulator))))
    const offRobot = simulator?.subscribe((event) => {
      if (event.type === 'decided') add([{ wire: 'local', back: false, light: event.light, delay: 0 }])
      if (event.type === 'synced' && event.count > 0) setSynced(event.count)
    })
    return () => {
      offServer()
      offRobot?.()
    }
  }, [])

  const drop = (key: number) => setComets((current) => current.filter((c) => c.key !== key))
  const on = (wire: WireName) => comets.filter((c) => c.wire === wire)

  const s = stats.data
  const decided = s ? s.by_source.device + s.by_source.server : 0
  const total = devices.data?.length ?? 0
  const online = devices.data?.filter((d) => d.online).length ?? 0
  const lastSeen = devices.data?.reduce<string | null>(
    (latest, d) => (d.last_seen_at && (!latest || d.last_seen_at > latest) ? d.last_seen_at : latest),
    null,
  )
  const rows = recent.data ?? []
  const localModel = rows.find((d) => d.local_model)?.local_model
  const cloudModel = rows.find((d) => d.source === 'server')?.model
  const waiting = s?.by_status.waiting_for_server ?? 0
  const showSynced = synced !== null && robot.online
  // A figure still on its way holds its place instead of reading as zero; one
  // the server doesn't know (no answers yet, an agent that doesn't report
  // it) leaves its readout out rather than showing a dash.
  const loading = <Skeleton className="h-5 w-12" />
  const reading = (pending: boolean, label: string, value: ReactNode | null) =>
    pending ? (
      <Readout label={label}>{loading}</Readout>
    ) : value == null ? null : (
      <Readout label={label}>{value}</Readout>
    )
  const ofStats = (label: string, value: ReactNode | null) => reading(stats.isPending, label, value)
  const ofDevices = (label: string, value: ReactNode | null) => reading(devices.isPending, label, value)
  const failed = stats.isError || devices.isError
  const retry = () => {
    if (stats.isError) void stats.refetch()
    if (devices.isError) void devices.refetch()
  }

  // Null when all is well: the header and the agent already say how many
  // robots are online, so the note only shows for news.
  let status: ReactNode = null
  // What a screen reader hears: the same news without the ticking clock and
  // counters, so it speaks once per change instead of every second.
  let announce = ''
  if (devices.isPending) {
    // Nothing yet: the readouts below hold their places.
  } else if (failed) {
    status = <LoadError compact what="The system's state" onRetry={retry} />
  } else if (devices.data && total === 0) {
    status = (
      <>
        <span className="font-medium">No robots yet.</span>{' '}
        <span className="text-muted-foreground">
          <Link to="/devices" className="underline underline-offset-4 hover:text-foreground">
            Add one
          </Link>{' '}
          and its decisions show up here.
        </span>
      </>
    )
  } else if (!robot.online) {
    announce = 'Offline. The robots keep deciding on their own.'
    status = (
      <>
        <span className="font-medium">
          Offline
          {robot.offlineSince ? (
            <span className="readout"> {duration(Math.max(0, now - robot.offlineSince))}</span>
          ) : null}
          .
        </span>{' '}
        <span className="text-muted-foreground">
          {robot.simulated
            ? `The robot keeps deciding: ${count(robot.unsynced)} made, ${count(robot.queued)} waiting in its queue.`
            : 'The robots keep deciding on their own and sync once the network is back.'}
        </span>
      </>
    )
  } else if (showSynced) {
    const syncedNote = `${count(synced)} offline ${synced === 1 ? 'decision' : 'decisions'} synced.`
    announce = `Back online. ${syncedNote}`
    status = (
      <>
        <span className="font-medium">Back online.</span>{' '}
        <span className="text-muted-foreground">
          {syncedNote}{' '}
          <Link to="/journal" className="underline underline-offset-4 hover:text-foreground">
            See them in the journal
          </Link>
        </span>
      </>
    )
  } else {
    announce = `${online} of ${total} ${total === 1 ? 'robot' : 'robots'} online.`
  }

  return (
    <section aria-labelledby="system-title">
      <div className="grid gap-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="grid max-w-[70ch] gap-1">
            <h2 id="system-title" className="text-xl font-semibold tracking-tight">
              Live system
            </h2>
            <p className="text-sm text-muted-foreground">Each dot is a decision happening now.</p>
          </div>
          {robot.simulated && (
            <Button
              variant={robot.online ? 'outline' : 'default'}
              onClick={() => simulator?.setOnline(!robot.online)}
              className="max-sm:w-full"
            >
              {robot.online ? <Scissors /> : <WifiHigh />}
              {robot.online ? 'Cut the network' : 'Restore the network'}
            </Button>
          )}
        </div>

        <p role="status" aria-live="polite" className="sr-only">
          {announce}
        </p>
        {status && (
          <div
            className={cn(
              'flex items-start gap-2.5 rounded-md border px-3 py-2.5 text-sm',
              robot.online || total === 0 ? 'border-border bg-card' : 'border-led-amber/30 bg-led-amber/[0.06]',
            )}
          >
            <Led color={failed || total === 0 ? 'off' : robot.online ? 'green' : 'amber'} className="mt-1.5" />
            <div className="min-w-0">{status}</div>
          </div>
        )}

        {/* One grid on wide screens: the robot on the left, running down from
            the camera to the agent; the cloud on the right, running up from
            the server to the model; the network between agent and server. On
            small screens everything stacks in one column. */}
        <div className="grid lg:grid-cols-[minmax(0,1fr)_minmax(7rem,11rem)_minmax(0,1fr)]">
          <ZoneFrame column="lg:col-start-1" />
          <ZoneFrame column="lg:col-start-3" />

          <Zone>
            <ZoneLabel className="lg:col-start-1">{total > 1 ? 'On each robot' : 'On the robot'}</ZoneLabel>
            <Node
              className="lg:col-start-1 lg:row-start-2"
              icon={<VideoCamera className="size-4" />}
              title="Camera and local model"
              subtitle={localModel ?? 'Local model'}
            >
              {ofStats('Answers in', s?.avg_latency_ms.device != null ? duration(s.avg_latency_ms.device) : null)}
              {ofStats('Decides alone', decided && s ? percent(s.by_source.device / decided) : null)}
            </Node>
            <Wire className="lg:col-start-1 lg:row-start-3" label="Task and frame" comets={on('local')} onDone={drop} />
            <Node
              className="lg:col-start-1 lg:row-start-4"
              icon={<Mark className="bg-transparent p-0 [&>span]:size-1.5" />}
              title="Cerebellum agent"
              subtitle="Rules, queue, journal"
              brand
              light={robot.queued ? 'amber' : undefined}
              lightLabel={robot.queued ? `${count(robot.queued)} queued` : undefined}
            >
              {ofDevices('Online', devices.data ? `${online}/${total}` : null)}
              {ofDevices('Last sync', lastSeen ? timeAgo(lastSeen, now) : null)}
            </Node>
            <ZoneNote className="lg:col-start-1">
              <LockSimple className="mt-px size-4 shrink-0" aria-hidden />
              <span>Video never leaves the robot, only single frames.</span>
            </ZoneNote>
          </Zone>

          <Wire
            network
            className="lg:col-start-2 lg:row-start-4"
            label={robot.online || total === 0 ? 'Journal and requests' : 'No network'}
            comets={on('network')}
            onDone={drop}
            broken={!robot.online && total > 0}
          />

          <Zone>
            <ZoneLabel className="lg:col-start-3">In the cloud</ZoneLabel>
            <Node
              className="lg:col-start-3 lg:row-start-4"
              icon={<HardDrives className="size-4" />}
              title="Cerebellum server"
              subtitle="Journal of every robot"
              brand
            >
              {ofStats('Decisions', s ? count(s.total) : null)}
              {ofStats('Corrections', s ? count(s.corrections) : null)}
            </Node>
            <Wire
              flip
              className="lg:col-start-3 lg:row-start-3"
              label="Hard cases"
              comets={on('cloud')}
              onDone={drop}
            />
            <Node
              className="lg:col-start-3 lg:row-start-2"
              icon={<Brain className="size-4" />}
              title="Cloud model"
              subtitle={`${cloudModel ? modelName(cloudModel) : 'Vision language model'} on ${CLOUD_PROVIDER}`}
              light={waiting ? 'amber' : undefined}
              lightLabel={waiting ? `${waiting} thinking` : undefined}
            >
              {ofStats('Answers in', s?.avg_latency_ms.server != null ? duration(s.avg_latency_ms.server) : null)}
              {ofStats('Spent', s ? usd(s.cost_usd) : null)}
            </Node>
            <ZoneNote className="lg:col-start-3">
              <Cloud className="mt-px size-4 shrink-0" aria-hidden />
              <span>Served by {CLOUD_PROVIDER} Token Factory.</span>
            </ZoneNote>
          </Zone>
        </div>
      </div>
    </section>
  )
}

/** The dashed outline of a zone on wide screens, behind its parts. */
function ZoneFrame({ column }: { column: string }) {
  return (
    <div
      aria-hidden
      className={cn(
        'hidden rounded-xl border border-dashed border-border-strong bg-card/40 lg:row-span-5 lg:row-start-1 lg:block',
        column,
      )}
    />
  )
}

/** A zone's parts: framed on small screens, laid into the shared grid on wide ones. */
function Zone({ children }: { children: ReactNode }) {
  return (
    <div className="grid rounded-xl border border-dashed border-border-strong bg-card/40 p-3 sm:p-4 lg:contents">
      {children}
    </div>
  )
}

function ZoneLabel({ className, children }: { className: string; children: ReactNode }) {
  return (
    <p className={cn('pb-3 text-xs font-medium text-muted-foreground lg:row-start-1 lg:px-5 lg:pt-4', className)}>
      {children}
    </p>
  )
}

function ZoneNote({ className, children }: { className: string; children: ReactNode }) {
  return (
    <p
      className={cn(
        'flex items-start gap-2 pt-4 text-xs leading-relaxed text-muted-foreground lg:row-start-5 lg:px-5 lg:pb-5',
        className,
      )}
    >
      {children}
    </p>
  )
}

function Node({
  className,
  icon,
  title,
  subtitle,
  brand,
  light,
  lightLabel,
  children,
}: {
  className?: string
  icon: ReactNode
  title: string
  subtitle: string
  /** One of Cerebellum's own parts. */
  brand?: boolean
  light?: LedColor
  lightLabel?: string
  children: ReactNode
}) {
  return (
    <div
      className={cn(
        'relative z-[1] grid content-between gap-4 rounded-lg border bg-card p-3.5 sm:p-4 lg:mx-5',
        brand
          ? 'border-brand bg-card'
          : 'border-border-strong',
        className,
      )}
    >
      <div className="flex items-start gap-3">
        <span
          className={cn(
            'flex size-8 shrink-0 items-center justify-center rounded-md border',
            brand ? 'border-transparent bg-brand text-brand-foreground' : 'border-border-strong bg-background',
          )}
        >
          {icon}
        </span>
        <div className="grid min-w-0 flex-1 gap-0.5">
          <h3 className="truncate text-sm font-medium">{title}</h3>
          <p className="truncate text-sm text-muted-foreground">{subtitle}</p>
        </div>
        {light && (
          <span className="flex shrink-0 items-center gap-1.5 text-xs text-muted-foreground">
            <Led color={light} />
            {lightLabel}
          </span>
        )}
      </div>
      <dl className="grid grid-cols-2 gap-3">{children}</dl>
    </div>
  )
}

function Wire({
  className,
  label,
  comets,
  onDone,
  network,
  flip,
  broken,
}: {
  className: string
  label: string
  comets: Comet[]
  onDone: (key: number) => void
  /** The network between robot and cloud: runs across on wide screens. */
  network?: boolean
  /** Runs upwards on wide screens. */
  flip?: boolean
  broken?: boolean
}) {
  return (
    <div
      className={cn(
        'relative h-16',
        network && 'lg:h-auto',
        flip && 'wire-flip-lg',
        className,
      )}
    >
      <span
        aria-hidden
        className={cn(
          'absolute inset-y-0 left-1/2 border-l',
          network && 'lg:inset-x-0 lg:inset-y-auto lg:top-1/2 lg:left-0 lg:border-t lg:border-l-0',
          broken ? 'border-dashed border-led-amber/60' : 'border-border-strong',
        )}
      />
      <span
        className={cn(
          'absolute top-1/2 left-[calc(50%+14px)] -translate-y-1/2 text-xs whitespace-nowrap',
          network &&
            'lg:top-auto lg:bottom-[calc(50%+16px)] lg:left-1/2 lg:-translate-x-1/2 lg:translate-y-0',
          broken ? 'text-led-amber' : 'text-muted-foreground',
        )}
      >
        {label}
      </span>
      {broken && (
        <span
          aria-hidden
          className="absolute top-1/2 left-1/2 flex size-7 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border border-led-amber/40 bg-background text-led-amber"
        >
          <WifiSlash className="size-4" />
        </span>
      )}
      {comets.map((c) => (
        <span
          key={c.key}
          aria-hidden
          data-back={c.back || undefined}
          className={cn('comet', network && 'comet-lg')}
          style={{ '--c': COLOR[c.light], animationDelay: `${c.delay}ms` } as CSSProperties}
          onAnimationEnd={() => onDone(c.key)}
        />
      ))}
    </div>
  )
}

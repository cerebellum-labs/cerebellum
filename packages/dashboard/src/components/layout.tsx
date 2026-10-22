import { GithubLogo, SignIn, SignOut, Trophy, WifiHigh } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { Link, NavLink, Outlet, useLocation } from 'react-router'

import { api, simulator } from '@/api/client'
import { useAdmin } from '@/app/context'
import { Led } from '@/components/led'
import { Mark } from '@/components/mark'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useNow } from '@/hooks/use-now'
import { useRobot } from '@/hooks/use-robot'
import { count, duration } from '@/lib/format'
import { cn } from '@/lib/utils'

const NAV = [
  { to: '/', label: 'Overview', end: true },
  { to: '/journal', label: 'Journal', end: false },
  { to: '/robots', label: 'Robots', end: false },
]

export function Layout() {
  const admin = useAdmin()
  const { pathname } = useLocation()
  // A new page starts at its top; a query change (?decision=) keeps the place.
  useEffect(() => {
    window.scrollTo(0, 0)
  }, [pathname])
  const devices = useQuery({ queryKey: ['devices'], queryFn: () => api.listDevices() })
  const online = devices.data?.filter((d) => d.online).length ?? 0
  const total = devices.data?.length ?? 0

  return (
    <div className="min-h-svh">
      <HackathonBanner />
      <header className="sticky top-0 z-20 border-b bg-background/85 backdrop-blur-md">
        <div className="mx-auto flex h-14 max-w-[1600px] items-center gap-3 px-4 sm:gap-6 sm:px-6">
          <div className="flex items-center gap-2.5">
            <Link to="/" className="flex items-center gap-2.5 text-[15px] font-semibold tracking-tight">
              <Mark />
              <span className="hidden sm:inline">Cerebellum</span>
            </Link>
          </div>

          <nav aria-label="Sections" className="flex h-full items-stretch">
            {NAV.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  cn(
                    'relative flex items-center px-2 text-sm text-muted-foreground transition-colors hover:text-foreground sm:px-3',
                    'after:absolute after:inset-x-2 after:-bottom-px after:h-px sm:after:inset-x-3',
                    'focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring',
                    isActive && 'text-foreground after:h-0.5 after:bg-brand',
                  )
                }
              >
                {item.label}
              </NavLink>
            ))}
          </nav>

          <div className="ml-auto flex items-center gap-4">
            <OfflineNotice />
            {devices.isPending && <Skeleton aria-hidden className="hidden h-4 w-20 md:block" />}
            {devices.data && (
              <Link
                to="/robots"
                className="hidden items-center gap-2 text-sm text-muted-foreground transition-colors hover:text-foreground md:flex"
              >
                <Led color={online ? 'green' : 'off'} />
                <span>
                  <span className="readout text-foreground">
                    {online}/{total}
                  </span>{' '}
                  online
                </span>
              </Link>
            )}
            {admin.signedIn ? (
              <Button variant="outline" size="sm" onClick={() => void admin.signOut()}>
                <SignOut /> <span className="max-sm:sr-only">Sign out</span>
              </Button>
            ) : (
              <Button variant="outline" size="sm" onClick={admin.signIn}>
                <SignIn /> <span className="max-sm:sr-only">Sign in</span>
              </Button>
            )}
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-[1600px] px-4 py-6 sm:px-6 sm:py-8">
        <Outlet />
      </main>
    </div>
  )
}

/**
 * While the robots are off the network, every page says so; the demo's
 * simulator can also bring the network back from here.
 */
function OfflineNotice() {
  const robot = useRobot()
  if (robot.online) return null
  return (
    <div className="flex items-center gap-2">
      <span className="flex items-center gap-2 rounded-full border border-led-amber/40 bg-led-amber/[0.08] px-2.5 py-1 text-xs text-led-amber">
        <Led color="amber" />
        <span>
          Offline
          {robot.offlineSince ? <OfflineFor since={robot.offlineSince} /> : null}
          {robot.queued ? <span className="max-sm:hidden"> · {count(robot.queued)} queued</span> : null}
        </span>
      </span>
      {robot.simulated && (
        <Button size="sm" onClick={() => simulator?.setOnline(true)}>
          <WifiHigh /> <span className="max-sm:sr-only">Restore</span>
        </Button>
      )}
    </div>
  )
}

function OfflineFor({ since }: { since: number }) {
  const now = useNow(1_000)
  return <span className="readout max-sm:hidden"> {duration(Math.max(0, now - since))}</span>
}

/** The page is a hackathon demo, not a product someone runs: say so first, like a label on the hardware. */
function HackathonBanner() {
  return (
    <div className="bg-brand text-brand-foreground">
      <div className="mx-auto flex min-h-11 max-w-[1600px] items-center gap-3 px-4 py-2 text-sm sm:px-6">
        <Trophy weight="fill" className="size-4 shrink-0" aria-hidden />
        <p className="min-w-0 flex-1">
          {/* Says what the visitor is looking at: the product itself, wired
              to a test stand, not a mock-up. */}
          <span className="font-semibold">Live demo</span>
          <span className="hidden sm:inline">
            : the real Cerebellum dashboard, {simulator ? 'running a simulated robot' : 'connected to a test robot'}
          </span>
          <span className="hidden xl:inline"> · Nebius × NVIDIA Global AI Hackathon</span>
        </p>
        <a
          href="https://github.com/cerebellum-labs/cerebellum"
          target="_blank"
          rel="noreferrer"
          className="flex shrink-0 items-center gap-1.5 font-medium underline-offset-4 hover:underline"
        >
          <GithubLogo weight="fill" className="size-4" aria-hidden />
          <span className="max-sm:sr-only">Source</span>
        </a>
      </div>
    </div>
  )
}

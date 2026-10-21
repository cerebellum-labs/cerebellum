import { GithubLogo, SignIn, SignOut, Trophy } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import { Link, NavLink, Outlet } from 'react-router'

import { api } from '@/api/client'
import { useAdmin } from '@/app/context'
import { Led } from '@/components/led'
import { Mark } from '@/components/mark'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const NAV = [
  { to: '/', label: 'Overview', end: true },
  { to: '/journal', label: 'Journal', end: false },
  { to: '/devices', label: 'Devices', end: false },
]

export function Layout() {
  const admin = useAdmin()
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
            {devices.data && (
              <Link
                to="/devices"
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

/** The page is a hackathon demo, not a product someone runs: say so first, like a label on the hardware. */
function HackathonBanner() {
  return (
    <div className="bg-brand text-brand-foreground">
      <div className="mx-auto flex min-h-11 max-w-[1600px] items-center gap-3 px-4 py-2 text-sm sm:px-6">
        <Trophy weight="fill" className="size-4 shrink-0" aria-hidden />
        <p className="min-w-0 flex-1">
          <span className="font-semibold">Hackathon demo</span>
          <span className="hidden sm:inline">, Physical AI track</span>
          <span className="hidden md:inline">. Nemotron on Nebius Token Factory.</span>
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

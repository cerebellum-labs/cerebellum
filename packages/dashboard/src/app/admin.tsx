import { useCallback, useMemo, useState, type FormEvent, type ReactNode } from 'react'

import { api, ApiError, isMock } from '@/api/client'
import { AdminContext, type Admin } from '@/app/context'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

// Reading is open to everyone; adding devices, new tokens and deleting need
// the dashboard password. The server keeps the session in a cookie the page
// can't read, so the page remembers that it signed in and forgets it on 401.
const SIGNED_IN_KEY = 'cerebellum.dashboard.signed-in'

function readSignedIn(): boolean {
  try {
    return sessionStorage.getItem(SIGNED_IN_KEY) === '1'
  } catch {
    return false
  }
}

export function AdminProvider({ children }: { children: ReactNode }) {
  const [signedIn, setSignedIn] = useState(readSignedIn)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [pending, setPending] = useState<(() => void) | null>(null)

  const remember = useCallback((value: boolean) => {
    setSignedIn(value)
    try {
      if (value) sessionStorage.setItem(SIGNED_IN_KEY, '1')
      else sessionStorage.removeItem(SIGNED_IN_KEY)
    } catch {
      // Private mode without storage: the session lasts until reload.
    }
  }, [])

  const admin = useMemo<Admin>(
    () => ({
      signedIn,
      whenSignedIn: (action) => {
        if (signedIn) return action()
        setPending(() => action)
        setDialogOpen(true)
      },
      signIn: () => setDialogOpen(true),
      signOut: async () => {
        await api.logout()
        remember(false)
      },
      sessionExpired: (then) => {
        remember(false)
        setPending(() => then ?? null)
        setDialogOpen(true)
      },
    }),
    [signedIn, remember],
  )

  return (
    <AdminContext value={admin}>
      {children}
      <SignInDialog
        open={dialogOpen}
        onOpenChange={(open) => {
          setDialogOpen(open)
          if (!open) setPending(null)
        }}
        onSignedIn={() => {
          remember(true)
          setDialogOpen(false)
          pending?.()
          setPending(null)
        }}
      />
    </AdminContext>
  )
}

function SignInDialog({
  open,
  onOpenChange,
  onSignedIn,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSignedIn: () => void
}) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await api.login(password)
      setPassword('')
      onSignedIn()
    } catch (err) {
      setError(err instanceof ApiError && err.status === 401 ? 'Wrong password.' : 'Could not reach the server. Try again.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setError(null)
        onOpenChange(next)
      }}
    >
      <DialogContent className="sm:max-w-sm">
        <form onSubmit={submit} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>Sign in to manage robots</DialogTitle>
            <DialogDescription>
              Anyone can watch the demo. Adding robots, issuing tokens and deleting robots need the dashboard
              password.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-2">
            <Label htmlFor="admin-password">Password</Label>
            <Input
              id="admin-password"
              type="password"
              autoComplete="current-password"
              autoFocus
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              aria-invalid={error ? true : undefined}
              aria-describedby={error ? 'admin-password-error' : undefined}
            />
            {error && (
              <p id="admin-password-error" className="text-sm text-destructive">
                {error}
              </p>
            )}
            {isMock && <p className="text-sm text-muted-foreground">Demo data: any password signs in.</p>}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={busy || !password}>
              {busy ? 'Signing in…' : 'Sign in'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowRight, Check, Copy, Key, Plus, SignIn, Trash } from '@phosphor-icons/react'
import { useState, type FormEvent } from 'react'
import { Link } from 'react-router'

import { api, ApiError } from '@/api/client'
import type { DeviceView } from '@/api/types'
import { useAdmin } from '@/app/context'
import { Led } from '@/components/led'
import { LoadError } from '@/components/load-error'
import { PageHeader } from '@/components/page-header'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
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
import { Skeleton } from '@/components/ui/skeleton'
import { useNow } from '@/hooks/use-now'
import { usePageTitle } from '@/hooks/use-page-title'
import { day, fullTime, timeAgo } from '@/lib/format'
import { cn } from '@/lib/utils'

// The header and every row share one grid; below desktop width a row stacks under its name.
const COLUMNS = cn(
  'grid items-center gap-x-4 gap-y-2 px-4 sm:px-5',
  'grid-cols-[10px_minmax(0,1fr)]',
  'lg:grid-cols-[10px_minmax(0,1fr)_minmax(0,14rem)_5rem_6rem_9rem]',
)

type Confirm = { kind: 'rotate' | 'delete'; device: DeviceView }
type Issued = { device: DeviceView; token: string; isNew: boolean }

export function DevicesPage() {
  const admin = useAdmin()
  const queryClient = useQueryClient()
  const now = useNow()
  usePageTitle('Robots')
  const devices = useQuery({ queryKey: ['devices'], queryFn: () => api.listDevices() })

  const [adding, setAdding] = useState(false)
  const [confirm, setConfirm] = useState<Confirm | null>(null)
  const [issued, setIssued] = useState<Issued | null>(null)

  const rotate = useMutation({
    mutationFn: (device: DeviceView) => api.rotateToken(device.id),
    onSuccess: (token, device) => setIssued({ device, token, isNew: false }),
  })
  const remove = useMutation({
    mutationFn: (device: DeviceView) => api.deleteDevice(device.id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['devices'] }),
  })

  /** Runs a confirmed action; a lost session asks for the password and retries. */
  function run(next: Confirm) {
    const mutation = next.kind === 'rotate' ? rotate : remove
    mutation.mutate(next.device, {
      onError: (err) => {
        if (err instanceof ApiError && err.status === 401) admin.sessionExpired(() => run(next))
      },
    })
    setConfirm(null)
  }

  const shownConfirm = useLastShown(confirm)

  const actionError = [rotate.error, remove.error].find((e) => e && !(e instanceof ApiError && e.status === 401))

  return (
    <section aria-labelledby="devices-title" className="grid gap-6">
      <PageHeader
        id="devices-title"
        title="Robots"
        description="Every robot in a fleet syncs to this one server with its own token. Online means it synced in the last 15 seconds."
      >
        {/* Visitors watch; managing robots is for the signed-in owner. */}
        {admin.signedIn ? (
          <Button onClick={() => setAdding(true)}>
            <Plus /> Add robot
          </Button>
        ) : (
          <Button variant="outline" onClick={admin.signIn}>
            <SignIn /> Sign in to manage
          </Button>
        )}
      </PageHeader>

      {actionError && (
        <p role="alert" className="text-sm text-destructive">
          {actionError.message}
        </p>
      )}

      {devices.isPending ? (
        <DevicesSkeleton />
      ) : devices.isError ? (
        <LoadError what="The robots" onRetry={() => devices.refetch()} className="panel p-6" />
      ) : devices.data.length === 0 ? (
        <div className="panel p-6 text-sm">
          <p className="font-medium">No robots yet.</p>
          <p className="mt-1 text-muted-foreground">Add one to get the token its agent signs requests with.</p>
        </div>
      ) : (
        <div className="panel overflow-clip">
          <div aria-hidden className={cn(COLUMNS, 'hidden h-9 border-b text-xs text-muted-foreground lg:grid')}>
            <span />
            <span>Robot</span>
            <span>Status</span>
            <span>Agent</span>
            <span>Added</span>
            <span />
          </div>
          <ul className="divide-y">
            {devices.data.map((device) => (
              <li key={device.id} className={cn(COLUMNS, 'py-3.5 text-sm')}>
                <Led color={device.online ? 'green' : 'off'} className="max-lg:mt-1.5 max-lg:self-start" />
                <div className="grid min-w-0 gap-1">
                  <Link to={`/journal?device=${device.id}`} className="w-fit font-medium hover:underline">
                    {device.name}
                  </Link>
                  <code className="readout text-xs break-all text-muted-foreground">{device.key}</code>
                  {/* What the agent reports about the robot: "camera: usb · board: Pi 5". */}
                  {Object.keys(device.metadata).length > 0 && (
                    <p className="text-xs text-muted-foreground">
                      {Object.entries(device.metadata)
                        .map(([k, v]) => `${k}: ${typeof v === 'string' ? v : JSON.stringify(v)}`)
                        .join('\u00a0· ')}
                    </p>
                  )}
                </div>
                <p className={cn('max-lg:col-start-2', device.online ? 'text-foreground' : 'text-muted-foreground')}>
                  {device.online
                    ? 'Online'
                    : device.last_seen_at
                      ? `Last seen ${timeAgo(device.last_seen_at, now)}`
                      : 'Never connected: start its agent with the token'}
                </p>
                <p className="readout text-muted-foreground max-lg:hidden">{device.agent_version ?? '–'}</p>
                <p className="text-muted-foreground max-lg:hidden" title={fullTime(device.created_at)}>
                  {day(device.created_at)}
                </p>
                {/* Deleting is rare: a quiet icon that turns red on hover, not
                    a red button on every row. */}
                <div className="flex items-center gap-1 max-lg:col-start-2 lg:justify-end">
                  {!admin.signedIn ? (
                    <Link
                      to={`/journal?device=${device.id}`}
                      className="flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground max-sm:min-h-10"
                    >
                      See decisions <ArrowRight className="size-4" aria-hidden />
                    </Link>
                  ) : (
                    <>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => admin.whenSignedIn(() => setConfirm({ kind: 'rotate', device }))}
                      >
                        <Key /> New token
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        // A full-size target for a finger on phones.
                        className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive max-sm:size-10"
                        aria-label={`Delete ${device.name}`}
                        title="Delete"
                        onClick={() => admin.whenSignedIn(() => setConfirm({ kind: 'delete', device }))}
                      >
                        <Trash />
                      </Button>
                    </>
                  )}
                </div>
              </li>
            ))}
          </ul>
        </div>
      )}

      <AddDeviceDialog
        open={adding}
        onOpenChange={setAdding}
        onAdded={(device, token) => {
          setAdding(false)
          setIssued({ device, token, isNew: true })
        }}
      />

      <AlertDialog open={Boolean(confirm)} onOpenChange={(open) => !open && setConfirm(null)}>
        <AlertDialogContent>
          {shownConfirm?.kind === 'rotate' ? (
            <AlertDialogHeader>
              <AlertDialogTitle>Issue a new token for {shownConfirm.device.name}?</AlertDialogTitle>
              <AlertDialogDescription>
                The current token stops working at once, so the robot goes offline until you put the new one on it.
              </AlertDialogDescription>
            </AlertDialogHeader>
          ) : (
            <AlertDialogHeader>
              <AlertDialogTitle>Delete {shownConfirm?.device.name}?</AlertDialogTitle>
              <AlertDialogDescription>
                Its token stops working and the robot can no longer sync. Its decisions stay in the journal.
              </AlertDialogDescription>
            </AlertDialogHeader>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel>Keep it</AlertDialogCancel>
            <AlertDialogAction
              variant={shownConfirm?.kind === 'delete' ? 'destructive' : 'default'}
              onClick={() => confirm && run(confirm)}
            >
              {shownConfirm?.kind === 'rotate' ? 'Issue new token' : 'Delete robot'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <TokenDialog issued={issued} onClose={() => setIssued(null)} />
    </section>
  )
}

/**
 * A device key as the server takes it, from anything typed: lowercase
 * letters and digits with single dashes, like "Bench arm 1" → "bench-arm-1".
 * A trailing dash stays while typing; the key sent drops it.
 */
function toKey(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+/, '')
    .slice(0, 64)
}

function AddDeviceDialog({
  open,
  onOpenChange,
  onAdded,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onAdded: (device: DeviceView, token: string) => void
}) {
  const admin = useAdmin()
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  // Follows the name until the key is edited by hand.
  const [typedKey, setTypedKey] = useState<string | null>(null)
  const shownKey = typedKey ?? toKey(name)
  const key = shownKey.replace(/-+$/, '')

  const create = useMutation({
    mutationFn: () => api.createDevice({ key, name: name.trim() }),
    onSuccess: ({ device, token }) => {
      void queryClient.invalidateQueries({ queryKey: ['devices'] })
      setName('')
      setTypedKey(null)
      onAdded(device, token)
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 401) admin.sessionExpired(() => onOpenChange(true))
    },
  })

  function submit(event: FormEvent) {
    event.preventDefault()
    create.mutate()
  }

  const error = create.error && !(create.error instanceof ApiError && create.error.status === 401) ? create.error : null

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) create.reset()
        onOpenChange(next)
      }}
    >
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>Add a robot</DialogTitle>
            <DialogDescription>You get its token once, right after adding it.</DialogDescription>
          </DialogHeader>
          <div className="grid gap-2">
            <Label htmlFor="device-name">Name</Label>
            <Input
              id="device-name"
              placeholder="e.g. Fruit sorter"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="device-key">Key</Label>
            <Input
              id="device-key"
              placeholder="e.g. fruit-sorter-2"
              value={shownKey}
              onChange={(e) => setTypedKey(toKey(e.target.value))}
              className="font-mono"
              aria-invalid={error ? true : undefined}
              aria-describedby="device-key-hint"
            />
            <p id="device-key-hint" className={error ? 'text-sm text-destructive' : 'text-sm text-muted-foreground'}>
              {error ? error.message : 'Made from the name; unique, with lowercase letters, digits and dashes.'}
            </p>
          </div>
          <DialogFooter>
            <Button type="submit" disabled={create.isPending || !key}>
              {create.isPending ? 'Adding…' : 'Add robot'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

/** Where the agent reaches the API: the API origin the dashboard itself uses. */
const API_URL = import.meta.env.VITE_API_URL || window.location.origin

function TokenDialog({ issued, onClose }: { issued: Issued | null; onClose: () => void }) {
  const [copied, setCopied] = useState(false)
  // A browser that refuses the clipboard: the lines are there to select.
  const [copyFailed, setCopyFailed] = useState(false)
  const shown = useLastShown(issued)
  const env = shown ? `CEREBELLUM_URL=${API_URL}\nCEREBELLUM_TOKEN=${shown.token}` : ''
  const guarded = !copied && !copyFailed

  async function copy() {
    try {
      await navigator.clipboard.writeText(env)
      setCopied(true)
    } catch {
      setCopyFailed(true)
    }
  }
  function close() {
    setCopied(false)
    setCopyFailed(false)
    onClose()
  }

  return (
    <Dialog open={Boolean(issued)} onOpenChange={(open) => !open && close()}>
      <DialogContent
        className="sm:max-w-lg"
        // The token is shown once: a stray click or Esc must not lose it
        // before it is copied. Done and the close button still close.
        onInteractOutside={(e) => guarded && e.preventDefault()}
        onEscapeKeyDown={(e) => guarded && e.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle>
            {shown?.isNew ? `${shown.device.name} is added` : `New token for ${shown?.device.name}`}
          </DialogTitle>
          <DialogDescription>
            Put these lines in the robot's <code className="readout text-foreground">.env</code> and start its agent. The
            token won't be shown again.
          </DialogDescription>
        </DialogHeader>
        <pre className="readout rounded-md border bg-background p-3 text-xs leading-relaxed break-all whitespace-pre-wrap text-foreground select-all">
          {env}
        </pre>
        {copyFailed && (
          <p role="alert" className="text-sm text-destructive">
            Couldn't copy. Select the lines above and copy them yourself.
          </p>
        )}
        <DialogFooter className="sm:justify-between">
          {/* Copying comes first; once it's done, closing does. Copy is last
              so it sits on top on phones and on the right on wide screens. */}
          <Button variant={copied ? 'default' : 'outline'} onClick={close}>
            Done
          </Button>
          <Button variant={copied ? 'outline' : 'default'} onClick={copy}>
            {copied ? <Check /> : <Copy />}
            {copied ? 'Copied' : 'Copy both lines'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** The last value that wasn't null, so a closing dialog keeps its text while it fades out. */
function useLastShown<T>(value: T | null): T | null {
  const [last, setLast] = useState(value)
  if (value !== null && value !== last) setLast(value)
  return value ?? last
}

/** The device list's rows while it loads, in the same columns. */
function DevicesSkeleton() {
  return (
    <div className="panel overflow-clip" aria-busy aria-label="Loading the robots">
      <div aria-hidden className={cn(COLUMNS, 'hidden h-9 border-b lg:grid')}>
        <span />
        <Skeleton className="h-3 w-12" />
        <Skeleton className="h-3 w-10" />
        <Skeleton className="h-3 w-10" />
        <Skeleton className="h-3 w-10" />
        <span />
      </div>
      <ul className="divide-y" aria-hidden>
        {['w-32', 'w-40'].map((width) => (
          <li key={width} className={cn(COLUMNS, 'py-3.5')}>
            <Skeleton className="size-2.5 rounded-full max-lg:mt-1.5 max-lg:self-start" />
            <div className="grid gap-2">
              <Skeleton className={cn('h-4', width)} />
              <Skeleton className="h-3 w-24" />
            </div>
            <Skeleton className="h-4 w-24 max-lg:col-start-2" />
            <Skeleton className="h-4 w-12 max-lg:hidden" />
            <Skeleton className="h-4 w-14 max-lg:hidden" />
            <div className="flex gap-1 max-lg:col-start-2 lg:justify-end">
              <Skeleton className="h-8 w-28" />
              <Skeleton className="size-8" />
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}

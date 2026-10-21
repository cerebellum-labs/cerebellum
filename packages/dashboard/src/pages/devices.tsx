import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Copy, Key, Plus, Trash } from '@phosphor-icons/react'
import { useState, type FormEvent } from 'react'
import { Link } from 'react-router'

import { api, ApiError } from '@/api/client'
import type { DeviceView } from '@/api/types'
import { useAdmin } from '@/app/context'
import { Led } from '@/components/led'
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
import { fullTime, timeAgo } from '@/lib/format'
import { cn } from '@/lib/utils'

// The header and every row share one grid; below desktop width a row stacks under its name.
const COLUMNS = cn(
  'grid items-center gap-x-4 gap-y-2 px-4 sm:px-5',
  'grid-cols-[10px_minmax(0,1fr)]',
  'lg:grid-cols-[10px_minmax(0,1.4fr)_minmax(0,1fr)_5rem_9rem_13rem]',
)

type Confirm = { kind: 'rotate' | 'delete'; device: DeviceView }
type Issued = { device: DeviceView; token: string; isNew: boolean }

export function DevicesPage() {
  const admin = useAdmin()
  const queryClient = useQueryClient()
  const now = useNow()
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

  const actionError = [rotate.error, remove.error].find((e) => e && !(e instanceof ApiError && e.status === 401))

  return (
    <section aria-labelledby="devices-title" className="grid gap-6">
      <PageHeader
        id="devices-title"
        title="Devices"
        description="Each robot syncs with its own token. Online means it synced in the last 15 seconds."
      >
        <Button onClick={() => admin.whenSignedIn(() => setAdding(true))}>
          <Plus /> Add device
        </Button>
      </PageHeader>

      {actionError && (
        <p role="alert" className="text-sm text-destructive">
          {actionError.message}
        </p>
      )}

      {devices.isPending ? (
        <Skeleton className="h-40 w-full rounded-lg" />
      ) : devices.isError ? (
        <div className="panel p-6 text-sm">
          <p className="font-medium">The devices didn't load.</p>
          <Button variant="outline" size="sm" className="mt-4" onClick={() => devices.refetch()}>
            Try again
          </Button>
        </div>
      ) : devices.data.length === 0 ? (
        <div className="panel p-6 text-sm">
          <p className="font-medium">No devices yet.</p>
          <p className="mt-1 text-muted-foreground">Add one to get the token its agent signs requests with.</p>
        </div>
      ) : (
        <div className="panel overflow-clip">
          <div aria-hidden className={cn(COLUMNS, 'hidden h-9 border-b text-xs text-muted-foreground lg:grid')}>
            <span />
            <span>Device</span>
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
                  {Object.keys(device.metadata).length > 0 && (
                    <p className="readout text-xs break-words text-muted-foreground">
                      {Object.entries(device.metadata)
                        .map(([k, v]) => `${k}=${typeof v === 'string' ? v : JSON.stringify(v)}`)
                        .join('  ')}
                    </p>
                  )}
                </div>
                <p className={cn('max-lg:col-start-2', device.online ? 'text-foreground' : 'text-muted-foreground')}>
                  {device.online
                    ? 'Online'
                    : device.last_seen_at
                      ? `Last seen ${timeAgo(device.last_seen_at, now)}`
                      : 'Never connected'}
                </p>
                <p className="readout text-muted-foreground max-lg:hidden">{device.agent_version ?? '–'}</p>
                <p className="readout text-muted-foreground max-lg:hidden">{fullTime(device.created_at)}</p>
                <div className="flex gap-2 max-lg:col-start-2 lg:justify-end">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => admin.whenSignedIn(() => setConfirm({ kind: 'rotate', device }))}
                  >
                    <Key /> New token
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="text-destructive hover:text-destructive"
                    onClick={() => admin.whenSignedIn(() => setConfirm({ kind: 'delete', device }))}
                  >
                    <Trash /> Delete
                  </Button>
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
          {confirm?.kind === 'rotate' ? (
            <AlertDialogHeader>
              <AlertDialogTitle>Issue a new token for {confirm.device.name}?</AlertDialogTitle>
              <AlertDialogDescription>
                The current token stops working at once, so the robot goes offline until you put the new one on it.
              </AlertDialogDescription>
            </AlertDialogHeader>
          ) : (
            <AlertDialogHeader>
              <AlertDialogTitle>Delete {confirm?.device.name}?</AlertDialogTitle>
              <AlertDialogDescription>
                Its token stops working and the robot can no longer sync. Its decisions stay in the journal.
              </AlertDialogDescription>
            </AlertDialogHeader>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel>Keep it</AlertDialogCancel>
            <AlertDialogAction
              variant={confirm?.kind === 'delete' ? 'destructive' : 'default'}
              onClick={() => confirm && run(confirm)}
            >
              {confirm?.kind === 'rotate' ? 'Issue new token' : 'Delete device'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <TokenDialog issued={issued} onClose={() => setIssued(null)} />
    </section>
  )
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
  const [key, setKey] = useState('')

  const create = useMutation({
    mutationFn: () => api.createDevice({ key: key.trim(), name: name.trim() }),
    onSuccess: ({ device, token }) => {
      void queryClient.invalidateQueries({ queryKey: ['devices'] })
      setName('')
      setKey('')
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
            <DialogTitle>Add a device</DialogTitle>
            <DialogDescription>You get its token once, right after adding it.</DialogDescription>
          </DialogHeader>
          <div className="grid gap-2">
            <Label htmlFor="device-name">Name</Label>
            <Input
              id="device-name"
              placeholder="Fruit sorter"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="device-key">Key</Label>
            <Input
              id="device-key"
              placeholder="fruit-sorter-2"
              value={key}
              onChange={(e) => setKey(e.target.value.toLowerCase())}
              aria-invalid={error ? true : undefined}
              aria-describedby="device-key-hint"
            />
            <p id="device-key-hint" className={error ? 'text-sm text-destructive' : 'text-sm text-muted-foreground'}>
              {error ? error.message : 'Unique and readable: lowercase letters, digits and dashes.'}
            </p>
          </div>
          <DialogFooter>
            <Button type="submit" disabled={create.isPending || !key.trim()}>
              {create.isPending ? 'Adding…' : 'Add device'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function TokenDialog({ issued, onClose }: { issued: Issued | null; onClose: () => void }) {
  const [copied, setCopied] = useState(false)

  async function copy() {
    if (!issued) return
    await navigator.clipboard.writeText(issued.token)
    setCopied(true)
  }

  return (
    <Dialog
      open={Boolean(issued)}
      onOpenChange={(open) => {
        if (!open) {
          setCopied(false)
          onClose()
        }
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {issued?.isNew ? `${issued.device.name} is added` : `New token for ${issued?.device.name}`}
          </DialogTitle>
          <DialogDescription>
            Copy the token now: it won't be shown again. Put it on the robot as CEREBELLUM_TOKEN in its .env file.
          </DialogDescription>
        </DialogHeader>
        <div className="flex gap-2">
          <Input readOnly value={issued?.token ?? ''} aria-label="Device token" className="font-mono text-xs" onFocus={(e) => e.target.select()} />
          <Button variant="outline" onClick={copy}>
            {copied ? <Check /> : <Copy />}
            {copied ? 'Copied' : 'Copy'}
          </Button>
        </div>
        <DialogFooter>
          <Button
            onClick={() => {
              setCopied(false)
              onClose()
            }}
          >
            Done
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

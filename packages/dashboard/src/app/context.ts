// Contexts and their hooks, apart from the provider components in admin.tsx
// and live.tsx, so that fast refresh can reload those files on their own.
import { createContext, useContext } from 'react'

export interface Admin {
  signedIn: boolean
  /** Runs `action` now if signed in, otherwise after a successful sign-in. */
  whenSignedIn: (action: () => void) => void
  signIn: () => void
  signOut: () => Promise<void>
  /** Call when the server answered 401: the session is gone. */
  sessionExpired: (then?: () => void) => void
}

export const AdminContext = createContext<Admin | null>(null)

export function useAdmin(): Admin {
  const admin = useContext(AdminContext)
  if (!admin) throw new Error('useAdmin needs an AdminProvider')
  return admin
}

export interface Live {
  /** Decisions that changed in the last few seconds. */
  changed: ReadonlySet<string>
  /** Decisions that first appeared in the last few seconds. */
  added: ReadonlySet<string>
}

export const LiveContext = createContext<Live>({ changed: new Set(), added: new Set() })

export function useLive(): Live {
  return useContext(LiveContext)
}

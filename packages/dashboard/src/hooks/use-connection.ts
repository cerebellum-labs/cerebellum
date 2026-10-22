import { useSyncExternalStore } from 'react'

import { api, type Connection } from '@/api/client'

/** The state of the server's event stream, live. */
export function useConnection(): Connection {
  return useSyncExternalStore(api.onConnection, api.connection)
}

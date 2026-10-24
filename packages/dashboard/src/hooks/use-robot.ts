import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'

import { api, simulator, type RobotState } from '@/api/client'
import { useNow } from '@/hooks/use-now'

export interface Robot extends RobotState {
  /** The demo can cut and restore the network. */
  simulated: boolean
  /**
   * Quiet for longer than a network cut lasts: the robots are switched off,
   * not deciding offline. Only a real server can tell; the simulator never is.
   */
  idle: boolean
}

/** How long a robot can be out of touch before it counts as switched off. */
const IDLE_MS = 5 * 60_000

/**
 * The robot's link to the server. The demo's simulator knows what happens on
 * the robot itself; a real server only knows whether the robot is in touch.
 */
export function useRobot(): Robot {
  const devices = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.listDevices(),
    // A robot that drops off the network changes nothing on the server, so
    // no event says so: the list is asked again to see it go offline.
    refetchInterval: simulator ? false : 5_000,
  })
  const [state, setState] = useState<RobotState | null>(() => simulator?.state() ?? null)
  // Ticks so a quiet robot turns idle without waiting for other news.
  const now = useNow(30_000)

  useEffect(() => simulator?.subscribe((event) => event.type === 'state' && setState(event.state)), [])

  if (state) return { ...state, simulated: true, idle: false }
  const list = devices.data ?? []
  // No robots is not an outage.
  const online = list.length === 0 || list.some((d) => d.online)
  // Offline since the last robot was last heard from.
  const lastSeen = Math.max(0, ...list.map((d) => (d.last_seen_at ? Date.parse(d.last_seen_at) : 0)))
  const idle = !online && (!lastSeen || now - lastSeen > IDLE_MS)
  return {
    online,
    queued: 0,
    unsynced: 0,
    simulated: false,
    idle,
    offlineSince: online || !lastSeen ? null : lastSeen,
  }
}

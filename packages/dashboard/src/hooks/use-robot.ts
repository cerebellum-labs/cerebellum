import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'

import { api, simulator, type RobotState } from '@/api/client'

export interface Robot extends RobotState {
  /** The demo can cut and restore the network. */
  simulated: boolean
}

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

  useEffect(() => simulator?.subscribe((event) => event.type === 'state' && setState(event.state)), [])

  if (state) return { ...state, simulated: true }
  const list = devices.data ?? []
  // No robots is not an outage.
  const online = list.length === 0 || list.some((d) => d.online)
  // Offline since the last robot was last heard from.
  const lastSeen = Math.max(0, ...list.map((d) => (d.last_seen_at ? Date.parse(d.last_seen_at) : 0)))
  return { online, queued: 0, unsynced: 0, simulated: false, offlineSince: online || !lastSeen ? null : lastSeen }
}

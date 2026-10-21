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
  const devices = useQuery({ queryKey: ['devices'], queryFn: () => api.listDevices() })
  const [state, setState] = useState<RobotState | null>(() => simulator?.state() ?? null)

  useEffect(() => simulator?.subscribe((event) => event.type === 'state' && setState(event.state)), [])

  if (state) return { ...state, simulated: true }
  const online = devices.data?.some((d) => d.online) ?? true
  return { online, queued: 0, unsynced: 0, simulated: false, offlineSince: null }
}

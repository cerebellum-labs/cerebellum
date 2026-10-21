import { useQuery } from '@tanstack/react-query'

import { api } from '@/api/client'

/** How many of the latest decisions the overview reads its figures from. */
const RECENT_LIMIT = 200

/** The latest decisions of all devices, newest first. Live events keep them fresh. */
export function useRecentDecisions() {
  return useQuery({
    queryKey: ['decisions', { recent: RECENT_LIMIT }],
    queryFn: () => api.listDecisions({ limit: RECENT_LIMIT }),
    select: (page) => page.decisions,
  })
}

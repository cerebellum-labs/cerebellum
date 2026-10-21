import { useEffect, useState } from 'react'

/** The current time, refreshed every `interval` ms, for "2 min ago" labels. */
export function useNow(interval = 10_000): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), interval)
    return () => clearInterval(timer)
  }, [interval])
  return now
}

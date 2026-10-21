const relative = new Intl.RelativeTimeFormat('en', { numeric: 'auto', style: 'short' })
const clock = new Intl.DateTimeFormat('en', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
const dateTime = new Intl.DateTimeFormat('en', {
  month: 'short',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
})

/** "12 sec. ago", "4 min. ago", "2 hr. ago". */
export function timeAgo(iso: string, now = Date.now()): string {
  // A moment in the future is a clock a little ahead: it just happened.
  const seconds = Math.min(0, Math.round((Date.parse(iso) - now) / 1000))
  if (Math.abs(seconds) < 60) return relative.format(seconds, 'second')
  const minutes = Math.round(seconds / 60)
  if (Math.abs(minutes) < 60) return relative.format(minutes, 'minute')
  const hours = Math.round(minutes / 60)
  if (Math.abs(hours) < 48) return relative.format(hours, 'hour')
  return relative.format(Math.round(hours / 24), 'day')
}

/** Local wall-clock time of the event, "14:03:11". */
export function clockTime(iso: string): string {
  return clock.format(new Date(iso))
}

export function fullTime(iso: string): string {
  return dateTime.format(new Date(iso))
}

/** "36 ms", "1.8 s". */
export function duration(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)} ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`
  const minutes = Math.floor(ms / 60_000)
  const seconds = Math.round((ms % 60_000) / 1000)
  return seconds ? `${minutes} min ${seconds} s` : `${minutes} min`
}

/** Model calls cost fractions of a cent, so small amounts keep their digits. */
export function usd(amount: number): string {
  if (amount === 0) return '$0'
  if (amount < 0.01) return `$${amount.toPrecision(2)}`
  return `$${amount.toFixed(amount < 1 ? 3 : 2)}`
}

export function percent(value: number): string {
  return `${Math.round(value * 100)}%`
}

export function count(value: number): string {
  return value.toLocaleString('en')
}

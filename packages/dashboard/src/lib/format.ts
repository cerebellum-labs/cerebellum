const relative = new Intl.RelativeTimeFormat('en', { numeric: 'auto', style: 'short' })
const clock = new Intl.DateTimeFormat('en', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
const dayOnly = new Intl.DateTimeFormat('en', { month: 'short', day: 'numeric' })
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

/** The day alone, "Oct 19": when a device was added. */
export function day(iso: string): string {
  return dayOnly.format(new Date(iso))
}

/** The local calendar day of a moment: "Today", "Yesterday" or "Oct 22". */
export function dayName(iso: string, now = Date.now()): string {
  const start = (t: number) => new Date(t).setHours(0, 0, 0, 0)
  const days = Math.round((start(now) - start(Date.parse(iso))) / 86_400_000)
  if (days === 0) return 'Today'
  if (days === 1) return 'Yesterday'
  return day(iso)
}

export function fullTime(iso: string): string {
  return dateTime.format(new Date(iso))
}

/**
 * "36 ms", "1.8 s", "4 min 12 s", "1 hr 32 min", "3 d 5 hr": the two largest
 * units, and seconds only while they still matter (under ten minutes).
 */
export function duration(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)} ms`
  // Below a minute that still reads as one once rounded to tenths.
  if (ms < 59_950) return `${(ms / 1000).toFixed(1)} s`
  const totalSeconds = Math.round(ms / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)
  const pair = (big: number, bigUnit: string, small: number, smallUnit: string) =>
    small ? `${big} ${bigUnit} ${small} ${smallUnit}` : `${big} ${bigUnit}`
  if (days) return pair(days, 'd', hours % 24, 'hr')
  if (hours) return pair(hours, 'hr', minutes % 60, 'min')
  return minutes < 10 ? pair(minutes, 'min', totalSeconds % 60, 's') : `${minutes} min`
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

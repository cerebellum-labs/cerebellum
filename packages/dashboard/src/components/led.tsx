import type { ComponentProps } from 'react'

import type { LedColor } from '@/lib/decision'
import { cn } from '@/lib/utils'

// A lit LED gets a faint halo of its own colour, an unlit one sits flat.
const FILL: Record<LedColor, string> = {
  green: 'bg-led-green shadow-[0_0_8px_color-mix(in_oklab,var(--led-green)_70%,transparent)]',
  blue: 'bg-led-blue shadow-[0_0_8px_color-mix(in_oklab,var(--led-blue)_70%,transparent)]',
  amber: 'led-waiting bg-led-amber shadow-[0_0_8px_color-mix(in_oklab,var(--led-amber)_70%,transparent)]',
  red: 'bg-led-red shadow-[0_0_8px_color-mix(in_oklab,var(--led-red)_70%,transparent)]',
  white: 'bg-led-white shadow-[0_0_8px_color-mix(in_oklab,var(--led-white)_50%,transparent)]',
  off: 'bg-led-off',
}

/** One of the sorter's LEDs. Decorative: the status is always written next to it. */
export function Led({ color, className, ...props }: { color: LedColor } & ComponentProps<'span'>) {
  return <span aria-hidden className={cn('inline-block size-2.5 shrink-0 rounded-full', FILL[color], className)} {...props} />
}

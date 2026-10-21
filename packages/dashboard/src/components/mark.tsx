import { cn } from '@/lib/utils'

/** The sorter's four lights as the product mark, in one colour. */
export function Mark({ className }: { className?: string }) {
  return (
    <span aria-hidden className={cn('grid shrink-0 grid-cols-2 gap-[3px] rounded-[5px] bg-brand p-[5px]', className)}>
      <span className="size-1.5 rounded-full bg-brand-foreground" />
      <span className="size-1.5 rounded-full bg-brand-foreground" />
      <span className="size-1.5 rounded-full bg-brand-foreground" />
      <span className="size-1.5 rounded-full bg-brand-foreground" />
    </span>
  )
}

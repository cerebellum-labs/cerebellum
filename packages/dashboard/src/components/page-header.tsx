import type { ReactNode } from 'react'

/** A page's title, one line on what it shows, and its controls on the right. */
export function PageHeader({
  id,
  title,
  description,
  children,
}: {
  id: string
  title: string
  description: ReactNode
  children?: ReactNode
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-x-6 gap-y-4">
      <div className="grid gap-1.5">
        <h1 id={id} className="text-2xl font-semibold tracking-tight">
          {title}
        </h1>
        <p className="max-w-[68ch] text-sm text-muted-foreground">{description}</p>
      </div>
      {children && <div className="grid w-full grid-cols-2 gap-2 sm:flex sm:w-auto">{children}</div>}
    </div>
  )
}

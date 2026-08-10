import type { ReactNode } from 'react'

/** Низ экрана: пояснение и главный CTA прижаты к нижнему краю (гипотеза H4). */
export function BoardFooter({ children }: { children: ReactNode }) {
  return <div className="mt-auto flex flex-col gap-2.5 pt-2">{children}</div>
}

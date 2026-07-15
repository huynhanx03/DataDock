import type { HTMLAttributes, ReactNode } from 'react'
import { Box } from 'lucide-react'
import { cn } from '@/shared/lib/cn'

type EmptyStateProps = HTMLAttributes<HTMLDivElement> & {
  icon?: ReactNode
  title: ReactNode
  description?: ReactNode
  actions?: ReactNode
  compact?: boolean
}

export function EmptyState({ icon, title, description, actions, compact = false, className, ...props }: EmptyStateProps) {
  return (
    <div
      data-slot="empty-state"
      className={cn('grid place-items-center px-6 text-center', compact ? 'min-h-40 py-8' : 'min-h-72 py-12', className)}
      {...props}
    >
      <div className="flex max-w-md flex-col items-center">
        <div className={cn('grid place-items-center rounded-xl border border-border bg-accent text-primary shadow-control [&_svg]:size-5', compact ? 'size-9' : 'size-11')}>
          {icon ?? <Box />}
        </div>
        <h2 className={cn('font-semibold tracking-[-0.015em] text-foreground', compact ? 'mt-3 text-sm' : 'mt-4 text-base')}>{title}</h2>
        {description ? <p className="mt-1.5 text-[length:var(--font-size-ui)] leading-5 text-muted-foreground">{description}</p> : null}
        {actions ? <div className="mt-4 flex flex-wrap items-center justify-center gap-2">{actions}</div> : null}
      </div>
    </div>
  )
}

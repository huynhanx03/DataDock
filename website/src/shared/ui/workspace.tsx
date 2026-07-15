import type { HTMLAttributes, ReactNode } from 'react'
import { cn } from '@/shared/lib/cn'

type WorkspacePageProps = HTMLAttributes<HTMLElement>

export function WorkspacePage({ className, ...props }: WorkspacePageProps) {
  return (
    <section
      data-slot="workspace-page"
      className={cn('flex h-full min-h-0 min-w-0 flex-col overflow-hidden bg-background', className)}
      {...props}
    />
  )
}

type WorkspaceHeaderProps = Omit<HTMLAttributes<HTMLElement>, 'title'> & {
  icon?: ReactNode
  eyebrow?: ReactNode
  title: ReactNode
  description?: ReactNode
  meta?: ReactNode
  actions?: ReactNode
  compact?: boolean
}

export function WorkspaceHeader({
  icon,
  eyebrow,
  title,
  description,
  meta,
  actions,
  compact = false,
  className,
  ...props
}: WorkspaceHeaderProps) {
  return (
    <header
      data-slot="workspace-header"
      className={cn(
        'flex shrink-0 flex-wrap items-center justify-between gap-4 border-b border-border bg-background/95 px-5 backdrop-blur lg:px-6',
        compact ? 'min-h-16 py-3' : 'min-h-[5.25rem] py-4',
        className,
      )}
      {...props}
    >
      <div className="flex min-w-0 items-center gap-3.5">
        {icon ? (
          <div className="grid size-10 shrink-0 place-items-center rounded-xl border border-primary/25 bg-accent text-primary shadow-control [&_svg]:size-[1.125rem]">
            {icon}
          </div>
        ) : null}
        <div className="min-w-0">
          {eyebrow ? (
            <div className="mb-0.5 flex items-center gap-1.5 text-[length:var(--font-size-meta)] font-semibold tracking-[0.08em] text-primary uppercase">
              {eyebrow}
            </div>
          ) : null}
          <div className="flex min-w-0 flex-wrap items-baseline gap-x-2.5 gap-y-0.5">
            <h1 className={cn('truncate font-semibold tracking-[-0.03em] text-foreground', compact ? 'text-lg leading-6' : 'text-[1.35rem] leading-7')}>
              {title}
            </h1>
            {meta}
          </div>
          {description ? <div className="mt-0.5 truncate text-[length:var(--font-size-ui)] text-muted-foreground">{description}</div> : null}
        </div>
      </div>
      {actions ? <div className="ml-auto flex shrink-0 flex-wrap items-center justify-end gap-2">{actions}</div> : null}
    </header>
  )
}

export function WorkspaceToolbar({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      data-slot="workspace-toolbar"
      className={cn('flex min-h-[var(--toolbar-height)] shrink-0 flex-wrap items-center gap-2 border-b border-border bg-surface/55 px-4 py-1.5 lg:px-5', className)}
      {...props}
    />
  )
}

type MasterDetailLayoutProps = HTMLAttributes<HTMLDivElement> & {
  master: ReactNode
  detail: ReactNode
  masterClassName?: string
  detailClassName?: string
}

export function MasterDetailLayout({
  master,
  detail,
  masterClassName,
  detailClassName,
  className,
  ...props
}: MasterDetailLayoutProps) {
  return (
    <div
      data-slot="master-detail-layout"
      className={cn('grid min-h-0 min-w-0 flex-1 grid-cols-[minmax(15rem,18rem)_minmax(0,1fr)] overflow-hidden max-lg:grid-cols-[minmax(13rem,15rem)_minmax(0,1fr)] max-md:grid-cols-1', className)}
      {...props}
    >
      <aside className={cn('min-h-0 min-w-0 overflow-hidden border-r border-border bg-surface/35 max-md:max-h-[42%] max-md:border-r-0 max-md:border-b', masterClassName)}>
        {master}
      </aside>
      <main className={cn('min-h-0 min-w-0 overflow-hidden bg-background', detailClassName)}>{detail}</main>
    </div>
  )
}

type WorkspacePanelProps = HTMLAttributes<HTMLDivElement> & {
  title?: ReactNode
  description?: ReactNode
  actions?: ReactNode
  noPadding?: boolean
}

export function WorkspacePanel({ title, description, actions, noPadding = false, className, children, ...props }: WorkspacePanelProps) {
  return (
    <div data-slot="workspace-panel" className={cn('overflow-hidden rounded-xl border border-border bg-surface shadow-control', className)} {...props}>
      {title || description || actions ? (
        <div className="flex min-h-12 items-center justify-between gap-3 border-b border-border px-4 py-2.5">
          <div className="min-w-0">
            {title ? <h2 className="truncate text-[length:var(--font-size-ui)] font-semibold text-foreground">{title}</h2> : null}
            {description ? <p className="mt-0.5 truncate text-[length:var(--font-size-meta)] text-muted-foreground">{description}</p> : null}
          </div>
          {actions ? <div className="flex shrink-0 items-center gap-1.5">{actions}</div> : null}
        </div>
      ) : null}
      <div className={cn(!noPadding && 'p-4')}>{children}</div>
    </div>
  )
}

type SectionHeadingProps = HTMLAttributes<HTMLDivElement> & {
  title: ReactNode
  description?: ReactNode
  actions?: ReactNode
}

export function SectionHeading({ title, description, actions, className, ...props }: SectionHeadingProps) {
  return (
    <div className={cn('flex min-h-10 items-start justify-between gap-4', className)} {...props}>
      <div className="min-w-0">
        <h2 className="text-sm font-semibold tracking-[-0.01em] text-foreground">{title}</h2>
        {description ? <p className="mt-0.5 text-[length:var(--font-size-meta)] leading-5 text-muted-foreground">{description}</p> : null}
      </div>
      {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
    </div>
  )
}

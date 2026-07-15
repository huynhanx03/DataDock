import type { HTMLAttributes, ReactNode } from 'react'
import { cn } from '@/shared/lib/cn'

type PropertyFieldProps = Omit<HTMLAttributes<HTMLDivElement>, 'title'> & {
  label: ReactNode
  htmlFor?: string
  description?: ReactNode
  error?: ReactNode
  required?: boolean
  inline?: boolean
}

export function PropertyField({ label, htmlFor, description, error, required, inline = false, className, children, ...props }: PropertyFieldProps) {
  return (
    <div
      data-slot="property-field"
      className={cn(inline ? 'grid grid-cols-[minmax(9rem,0.42fr)_minmax(0,1fr)] items-start gap-4' : 'grid gap-1.5', className)}
      {...props}
    >
      <div className="min-w-0">
        <label htmlFor={htmlFor} className="text-[length:var(--font-size-ui)] font-medium text-foreground">
          {label}
          {required ? <span className="ml-1 text-destructive" aria-hidden="true">*</span> : null}
          {required ? <span className="sr-only"> required</span> : null}
        </label>
        {inline && description ? <p className="mt-0.5 text-[length:var(--font-size-meta)] leading-4 text-muted-foreground">{description}</p> : null}
      </div>
      <div className="min-w-0">
        {children}
        {!inline && description ? <p className="mt-1 text-[length:var(--font-size-meta)] leading-4 text-muted-foreground">{description}</p> : null}
        {error ? <p role="alert" className="mt-1 text-[length:var(--font-size-meta)] leading-4 text-destructive">{error}</p> : null}
      </div>
    </div>
  )
}

export function PropertyGrid({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div data-slot="property-grid" className={cn('grid grid-cols-1 gap-4 lg:grid-cols-2', className)} {...props} />
}

import { forwardRef, type InputHTMLAttributes, type SelectHTMLAttributes, type TextareaHTMLAttributes } from 'react'
import { Check, ChevronDown } from 'lucide-react'
import { cn } from '@/shared/lib/cn'

export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(
  ({ className, children, ...props }, ref) => (
    <span className="relative block min-w-0">
      <select
        ref={ref}
        data-slot="select"
        className={cn('h-[var(--control-height)] w-full min-w-0 appearance-none rounded-md border border-input bg-surface px-2.5 pr-8 text-[length:var(--font-size-ui)] text-foreground shadow-control outline-none transition-[border-color,box-shadow] focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/20 disabled:pointer-events-none disabled:bg-muted/60 disabled:opacity-60', className)}
        {...props}
      >
        {children}
      </select>
      <ChevronDown className="pointer-events-none absolute top-1/2 right-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
    </span>
  ),
)

Select.displayName = 'Select'

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  ({ className, ...props }, ref) => (
    <textarea
      ref={ref}
      data-slot="textarea"
      className={cn('min-h-24 w-full min-w-0 resize-y rounded-md border border-input bg-surface px-2.5 py-2 text-[length:var(--font-size-ui)] leading-5 text-foreground shadow-control outline-none transition-[border-color,box-shadow] placeholder:text-muted-foreground/75 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/20 disabled:pointer-events-none disabled:bg-muted/60 disabled:opacity-60', className)}
      {...props}
    />
  ),
)

Textarea.displayName = 'Textarea'

type CheckboxProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'size'> & {
  label?: string
}

export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(({ className, label, ...props }, ref) => (
  <label className={cn('inline-flex min-w-0 items-center gap-2 text-[length:var(--font-size-ui)] text-foreground', props.disabled && 'opacity-55', className)}>
    <span className="relative grid size-4 shrink-0 place-items-center">
      <input ref={ref} type="checkbox" className="peer absolute inset-0 m-0 appearance-none rounded border border-input bg-surface shadow-control outline-none focus-visible:ring-3 focus-visible:ring-ring/25 checked:border-primary checked:bg-primary" {...props} />
      <Check className="pointer-events-none size-3 text-primary-foreground opacity-0 peer-checked:opacity-100" />
    </span>
    {label ? <span className="truncate">{label}</span> : null}
  </label>
))

Checkbox.displayName = 'Checkbox'

type SwitchProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'size'> & {
  label?: string
  description?: string
}

export const Switch = forwardRef<HTMLInputElement, SwitchProps>(({ className, label, description, ...props }, ref) => (
  <label className={cn('flex min-w-0 items-center gap-3', props.disabled && 'opacity-55', className)}>
    <span className="relative inline-flex h-5 w-9 shrink-0">
      <input ref={ref} type="checkbox" role="switch" className="peer absolute inset-0 m-0 appearance-none rounded-full border border-input bg-muted outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/25 checked:border-primary checked:bg-primary" {...props} />
      <span className="pointer-events-none absolute top-0.5 left-0.5 size-4 rounded-full bg-surface shadow-sm transition-transform peer-checked:translate-x-4 peer-checked:bg-primary-foreground" />
    </span>
    {label || description ? <span className="min-w-0"><span className="block text-[length:var(--font-size-ui)] font-medium text-foreground">{label}</span>{description ? <span className="mt-0.5 block text-[length:var(--font-size-meta)] leading-4 text-muted-foreground">{description}</span> : null}</span> : null}
  </label>
))

Switch.displayName = 'Switch'

import { forwardRef, type InputHTMLAttributes } from 'react'
import { cn } from '@/shared/lib/cn'

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  invalid?: boolean
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, invalid, type, ...props }, ref) => (
    <input
      ref={ref}
      type={type}
      {...props}
      data-slot="input"
      aria-invalid={invalid || props['aria-invalid'] || undefined}
      className={cn(
        'h-[var(--control-height)] w-full min-w-0 rounded-md border border-input bg-surface px-2.5 py-1 text-[length:var(--font-size-ui)] text-foreground shadow-control outline-none transition-[border-color,box-shadow,background-color] placeholder:text-muted-foreground/75 selection:bg-primary/25 disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-muted/60 disabled:opacity-60 file:mr-3 file:border-0 file:bg-transparent file:text-[length:var(--font-size-ui)] file:font-medium file:text-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/20 aria-invalid:border-destructive aria-invalid:ring-destructive/15',
        className,
      )}
    />
  ),
)

Input.displayName = 'Input'

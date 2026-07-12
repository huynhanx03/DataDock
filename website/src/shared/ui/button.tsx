import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/shared/lib/cn'

export const buttonVariants = cva(
  'inline-flex shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-md border border-transparent font-medium transition-[color,background-color,border-color,box-shadow,transform] duration-150 outline-none select-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/25 disabled:pointer-events-none disabled:opacity-45 active:translate-y-px [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*="size-"])]:size-4',
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground shadow-control hover:bg-primary/90',
        secondary: 'border-border bg-secondary text-secondary-foreground shadow-control hover:border-border-strong hover:bg-secondary/75',
        outline: 'border-border bg-surface text-foreground shadow-control hover:border-border-strong hover:bg-accent hover:text-accent-foreground',
        ghost: 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
        subtle: 'bg-accent/70 text-accent-foreground hover:bg-accent',
        destructive: 'bg-destructive text-destructive-foreground shadow-control hover:bg-destructive/90',
        link: 'h-auto rounded-none p-0 text-primary underline-offset-4 hover:underline active:translate-y-0',
      },
      size: {
        xs: 'h-[var(--control-height-xs)] gap-1.5 px-2.5 text-[length:var(--font-size-ui)] [&_svg:not([class*="size-"])]:size-3.5',
        sm: 'h-[var(--control-height-sm)] gap-1.5 px-3 text-[length:var(--font-size-ui)]',
        default: 'h-[var(--control-height)] px-3.5 text-[length:var(--font-size-ui)]',
        lg: 'h-[var(--control-height-lg)] px-4 text-[length:var(--font-size-ui)]',
        'icon-xs': 'size-[var(--control-height-xs)] p-0 [&_svg:not([class*="size-"])]:size-3.5',
        'icon-sm': 'size-[var(--control-height-sm)] p-0',
        icon: 'size-[var(--control-height)] p-0',
        'icon-lg': 'size-[var(--control-height-lg)] p-0 [&_svg:not([class*="size-"])]:size-[1.125rem]',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)

export interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ asChild = false, className, size, variant, ...props }, ref) => {
    const Component = asChild ? Slot : 'button'

    return (
      <Component
        ref={ref}
        data-slot="button"
        className={cn(buttonVariants({ size, variant }), className)}
        {...props}
      />
    )
  },
)

Button.displayName = 'Button'

export interface IconButtonProps
  extends Omit<ButtonProps, 'aria-label' | 'children' | 'size'> {
  label: string
  children: ReactNode
  size?: 'icon-xs' | 'icon-sm' | 'icon' | 'icon-lg'
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ children, label, size = 'icon-sm', variant = 'ghost', ...props }, ref) => (
    <Button
      ref={ref}
      aria-label={label}
      size={size}
      variant={variant}
      {...props}
    >
      {children}
      <span className="sr-only">{label}</span>
    </Button>
  ),
)

IconButton.displayName = 'IconButton'

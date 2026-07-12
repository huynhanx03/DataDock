import { type ComponentPropsWithoutRef } from 'react'
import * as TooltipPrimitive from '@radix-ui/react-tooltip'
import { cn } from '@/shared/lib/cn'

export function TooltipProvider({
  delayDuration = 350,
  skipDelayDuration = 100,
  ...props
}: ComponentPropsWithoutRef<typeof TooltipPrimitive.Provider>) {
  return (
    <TooltipPrimitive.Provider
      delayDuration={delayDuration}
      skipDelayDuration={skipDelayDuration}
      {...props}
    />
  )
}

export const Tooltip = TooltipPrimitive.Root
export const TooltipTrigger = TooltipPrimitive.Trigger

export function TooltipContent({
  children,
  className,
  sideOffset = 7,
  ...props
}: ComponentPropsWithoutRef<typeof TooltipPrimitive.Content>) {
  return (
    <TooltipPrimitive.Portal>
      <TooltipPrimitive.Content
        data-slot="tooltip-content"
        sideOffset={sideOffset}
        className={cn(
          'z-70 max-w-72 rounded-md border border-border-strong bg-popover px-2.5 py-1.5 text-xs leading-4 text-popover-foreground opacity-0 shadow-overlay transition-[opacity,transform] duration-100 data-[state=delayed-open]:opacity-100 data-[state=delayed-open]:data-[side=bottom]:translate-y-1 data-[state=delayed-open]:data-[side=left]:-translate-x-1 data-[state=delayed-open]:data-[side=right]:translate-x-1 data-[state=delayed-open]:data-[side=top]:-translate-y-1',
          className,
        )}
        {...props}
      >
        {children}
        <TooltipPrimitive.Arrow className="fill-popover" />
      </TooltipPrimitive.Content>
    </TooltipPrimitive.Portal>
  )
}

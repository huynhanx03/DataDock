import { type ComponentPropsWithoutRef } from 'react'
import { GripVertical } from 'lucide-react'
import * as ResizablePrimitive from 'react-resizable-panels'
import { cn } from '@/shared/lib/cn'

export function ResizablePanelGroup({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof ResizablePrimitive.Group>) {
  return (
    <ResizablePrimitive.Group
      data-slot="resizable-panel-group"
      className={cn('flex size-full data-[panel-group-direction=vertical]:flex-col', className)}
      {...props}
    />
  )
}

export const ResizablePanel = ResizablePrimitive.Panel

export interface ResizableHandleProps
  extends ComponentPropsWithoutRef<typeof ResizablePrimitive.Separator> {
  withHandle?: boolean
}

export function ResizableHandle({
  className,
  withHandle = false,
  ...props
}: ResizableHandleProps) {
  return (
    <ResizablePrimitive.Separator
      data-slot="resizable-handle"
      className={cn(
        'group relative z-10 flex h-full w-px shrink-0 items-center justify-center bg-border outline-none transition-colors after:absolute after:inset-y-0 after:left-1/2 after:w-3 after:-translate-x-1/2 hover:bg-primary/65 focus-visible:bg-primary focus-visible:ring-1 focus-visible:ring-ring aria-[orientation=horizontal]:h-px aria-[orientation=horizontal]:w-full aria-[orientation=horizontal]:after:inset-x-0 aria-[orientation=horizontal]:after:top-1/2 aria-[orientation=horizontal]:after:h-3 aria-[orientation=horizontal]:after:w-full aria-[orientation=horizontal]:after:-translate-x-0 aria-[orientation=horizontal]:after:-translate-y-1/2',
        className,
      )}
      {...props}
    >
      {withHandle ? (
        <span className="relative z-10 flex h-7 w-3.5 items-center justify-center rounded-sm border border-border bg-surface-elevated text-muted-foreground shadow-control group-aria-[orientation=horizontal]:h-3.5 group-aria-[orientation=horizontal]:w-7">
          <GripVertical className="size-3 group-aria-[orientation=horizontal]:rotate-90" />
        </span>
      ) : null}
    </ResizablePrimitive.Separator>
  )
}

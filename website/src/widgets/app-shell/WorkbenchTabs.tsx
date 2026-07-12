import { Plus, X } from 'lucide-react'
import { cn } from '@/shared/lib/cn'
import { IconButton } from '@/shared/ui'
import type { WorkbenchTab } from './shell-types'

type WorkbenchTabsProps = {
  tabs: WorkbenchTab[]
  activeTabId: string
  onSelect: (tabId: string) => void
  onClose: (tabId: string) => void
  onNewQuery: () => void
}

export function WorkbenchTabs({ tabs, activeTabId, onSelect, onClose, onNewQuery }: WorkbenchTabsProps) {
  return (
    <div className="flex h-[var(--toolbar-height)] shrink-0 items-stretch border-b border-border bg-surface/80" role="tablist" aria-label="Open workbench tabs">
      <div className="min-w-0 flex-1 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
        <div className="flex h-full min-w-max items-stretch">
          {tabs.map((tab) => {
            const Icon = tab.icon
            const active = tab.id === activeTabId

            return (
              <div
                key={tab.id}
                className={cn(
                  'group relative flex h-full min-w-32 max-w-56 items-center border-r border-border text-[length:var(--font-size-ui)] text-muted-foreground transition-colors hover:bg-accent/55 hover:text-foreground',
                  active && 'bg-background text-foreground',
                )}
              >
                {active ? <span className="absolute inset-x-0 top-0 h-0.5 bg-primary" /> : null}
                <button
                  type="button"
                  role="tab"
                  aria-selected={active}
                  onClick={() => onSelect(tab.id)}
                  className="flex h-full min-w-0 flex-1 items-center gap-2 px-3.5 outline-none focus-visible:z-10 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/60"
                >
                  <Icon className={cn('size-3.5 shrink-0', active && 'text-primary')} strokeWidth={1.8} />
                  <span className="min-w-0 flex-1 truncate text-left font-medium">{tab.label}</span>
                  {tab.dirty ? <span className="size-1.5 shrink-0 rounded-full bg-warning-foreground" aria-label="Unsaved changes" /> : null}
                </button>
                {tab.closable ? (
                  <button
                    type="button"
                    aria-label={`Close ${tab.label}`}
                    onClick={(event) => {
                      event.stopPropagation()
                      onClose(tab.id)
                    }}
                    className="mr-2 grid size-5 shrink-0 place-items-center rounded text-muted-foreground opacity-60 outline-none transition-[opacity,background-color,color] hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring group-hover:opacity-100"
                  >
                    <X className="size-3" />
                  </button>
                ) : null}
              </div>
            )
          })}
        </div>
      </div>
      <div className="flex shrink-0 items-center border-l border-border px-1.5">
        <IconButton label="New SQL query" size="icon-xs" onClick={onNewQuery}>
          <Plus />
        </IconButton>
      </div>
    </div>
  )
}

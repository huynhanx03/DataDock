import {
  Activity,
  Blocks,
  Braces,
  Cable,
  Clock3,
  Command,
  Database,
  FolderHeart,
  Settings,
  Sparkles,
} from 'lucide-react'
import { APP_CONFIG } from '@/shared/config/constants'
import { APP_ROUTES } from '@/shared/config/routes'
import { cn } from '@/shared/lib/cn'
import {
  IconButton,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/shared/ui'
import type { ActivityId, ActivityItem } from './shell-types'

const PRIMARY_ACTIVITIES: ActivityItem[] = [
  { id: 'explorer', label: 'Database explorer', icon: Blocks, route: APP_ROUTES.home },
  { id: 'connections', label: 'Connection manager', icon: Cable, route: APP_ROUTES.connections },
  { id: 'query', label: 'SQL workspace', icon: Braces, route: APP_ROUTES.query },
  { id: 'history', label: 'Query history', icon: Clock3, route: APP_ROUTES.history },
  { id: 'saved', label: 'Saved queries', icon: FolderHeart, route: APP_ROUTES.savedQueries },
  { id: 'operations', label: 'Operations', icon: Activity, route: APP_ROUTES.operations },
]

type ActivityRailProps = {
  active: ActivityId
  onSelect: (activity: ActivityItem) => void
  onOpenCommandPalette: () => void
}

function RailButton({
  active,
  item,
  onSelect,
}: {
  active: boolean
  item: ActivityItem
  onSelect: (item: ActivityItem) => void
}) {
  const Icon = item.icon

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={item.label}
          aria-pressed={active}
          onClick={() => onSelect(item)}
          className={cn(
            'group relative grid size-10 place-items-center rounded-lg text-muted-foreground outline-none transition-[color,background-color,box-shadow] duration-150 hover:bg-accent hover:text-accent-foreground focus-visible:ring-2 focus-visible:ring-ring/50',
            active && 'bg-accent text-accent-foreground shadow-[inset_0_0_0_1px_color-mix(in_oklab,var(--primary)_20%,transparent)]',
          )}
        >
          <span
            className={cn(
              'absolute top-2 bottom-2 left-[-0.5rem] w-0.5 rounded-full bg-primary opacity-0 transition-opacity',
              active && 'opacity-100',
            )}
          />
          <Icon className="size-[1.125rem]" strokeWidth={1.8} />
        </button>
      </TooltipTrigger>
      <TooltipContent side="right">
        <span>{item.label}</span>
        {item.shortcut ? <kbd className="ml-2 text-muted-foreground">{item.shortcut}</kbd> : null}
      </TooltipContent>
    </Tooltip>
  )
}

export function ActivityRail({ active, onSelect, onOpenCommandPalette }: ActivityRailProps) {
  const settingsItem: ActivityItem = {
    id: 'settings',
    label: 'Settings',
    icon: Settings,
    route: APP_ROUTES.settings,
  }

  return (
    <aside className="flex h-full w-14 shrink-0 flex-col items-center border-r border-border bg-surface/95 py-2.5" aria-label="Primary navigation">
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            className="relative mb-4 grid size-10 place-items-center overflow-hidden rounded-xl border border-primary/30 bg-[radial-gradient(circle_at_30%_20%,color-mix(in_oklab,var(--primary)_95%,white),color-mix(in_oklab,var(--primary)_62%,black)_65%)] text-primary-foreground shadow-[0_8px_22px_color-mix(in_oklab,var(--primary)_24%,transparent)] outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
            aria-label={`${APP_CONFIG.name} home`}
            onClick={() => onSelect(PRIMARY_ACTIVITIES[0])}
          >
            <Database className="size-5" strokeWidth={2.1} />
            <Sparkles className="absolute top-1.5 right-1.5 size-2.5 opacity-85" />
          </button>
        </TooltipTrigger>
        <TooltipContent side="right">{APP_CONFIG.name}</TooltipContent>
      </Tooltip>

      <nav className="flex flex-col items-center gap-1" aria-label="Workbench">
        {PRIMARY_ACTIVITIES.map((item) => (
          <RailButton
            key={item.id}
            item={item}
            active={active === item.id}
            onSelect={onSelect}
          />
        ))}
      </nav>

      <div className="mt-auto flex flex-col items-center gap-1">
        <Tooltip>
          <TooltipTrigger asChild>
            <IconButton label="Open command palette" size="icon-lg" onClick={onOpenCommandPalette}>
              <Command className="size-[1.125rem]" />
            </IconButton>
          </TooltipTrigger>
          <TooltipContent side="right">
            Command palette <kbd className="ml-2 text-muted-foreground">⌘K</kbd>
          </TooltipContent>
        </Tooltip>
        <RailButton item={settingsItem} active={active === 'settings'} onSelect={onSelect} />
        <div className="my-1 h-px w-7 bg-border" />
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              aria-label="Local workspace profile"
              className="grid size-9 place-items-center rounded-full border border-primary/35 bg-accent font-mono text-xs font-semibold text-accent-foreground shadow-control outline-none transition-colors hover:bg-primary/15 focus-visible:ring-2 focus-visible:ring-ring/50"
            >
              DD
            </button>
          </TooltipTrigger>
          <TooltipContent side="right">Local self-hosted workspace</TooltipContent>
        </Tooltip>
      </div>
    </aside>
  )
}

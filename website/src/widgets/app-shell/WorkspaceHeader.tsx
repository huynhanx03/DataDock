import { Check, ChevronsUpDown, MoreHorizontal, Plus, Search } from 'lucide-react'
import type { Workspace } from '@/entities/workspace'
import { APP_CONFIG } from '@/shared/config/constants'
import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  IconButton,
} from '@/shared/ui'

type WorkspaceHeaderProps = {
  workspaces: Workspace[]
  activeWorkspace?: Workspace
  connectionCount: number
  onWorkspaceChange: (workspaceId: string) => void
  onNewConnection: () => void
  onOpenCommandPalette: () => void
}

export function WorkspaceHeader({
  workspaces,
  activeWorkspace,
  connectionCount,
  onWorkspaceChange,
  onNewConnection,
  onOpenCommandPalette,
}: WorkspaceHeaderProps) {
  return (
    <div className="border-b border-border bg-surface/90 px-3 pb-3 pt-3.5 backdrop-blur-xl">
      <div className="flex items-center gap-2">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" className="h-[var(--control-height)] min-w-0 flex-1 justify-start px-2 hover:bg-accent">
              <span
                className="grid size-6 shrink-0 place-items-center rounded-md text-[length:var(--font-size-meta)] font-bold text-white shadow-control"
                style={{ backgroundColor: activeWorkspace?.color || 'var(--primary)' }}
              >
                {(activeWorkspace?.name || APP_CONFIG.name).slice(0, 1).toUpperCase()}
              </span>
              <span className="min-w-0 flex-1 truncate text-left text-[length:var(--font-size-ui)] font-semibold text-foreground">
                {activeWorkspace?.name || 'Choose workspace'}
              </span>
              <ChevronsUpDown className="size-3.5 text-muted-foreground" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-64">
            <DropdownMenuLabel>Workspaces</DropdownMenuLabel>
            {workspaces.map((workspace) => (
              <DropdownMenuItem key={workspace.id} onSelect={() => onWorkspaceChange(workspace.id)}>
                <span
                  className="grid size-6 place-items-center rounded-md text-[length:var(--font-size-meta)] font-bold text-white"
                  style={{ backgroundColor: workspace.color }}
                >
                  {workspace.name.slice(0, 1).toUpperCase()}
                </span>
                <span className="min-w-0 flex-1 truncate">{workspace.name}</span>
                {workspace.id === activeWorkspace?.id ? <Check className="text-primary" /> : null}
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem>
              <Plus />
              Create workspace
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <IconButton label="Workspace options" variant="ghost">
          <MoreHorizontal />
        </IconButton>
      </div>

      <div className="mt-3 grid grid-cols-[1fr_auto] gap-2">
        <Button size="sm" className="justify-start shadow-[0_7px_22px_color-mix(in_oklab,var(--primary)_18%,transparent)]" onClick={onNewConnection}>
          <Plus />
          New connection
        </Button>
        <IconButton label="Open command palette" variant="outline" onClick={onOpenCommandPalette}>
          <Search />
        </IconButton>
      </div>

      <div className="mt-3 flex items-center justify-between px-0.5 text-[length:var(--font-size-meta)] text-muted-foreground">
        <span>{connectionCount} saved connections</span>
        <kbd className="rounded border border-border bg-muted/70 px-1.5 py-0.5 font-mono text-[length:var(--font-size-meta)]">⌘K</kbd>
      </div>
    </div>
  )
}

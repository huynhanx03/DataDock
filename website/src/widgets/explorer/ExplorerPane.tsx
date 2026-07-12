import { useState } from 'react'
import { ChevronDown, Database, RefreshCw, Star } from 'lucide-react'
import type { Connection } from '@/entities/connection'
import type { CatalogTree, DatabaseObject } from '@/entities/database-object'
import type { Workspace } from '@/entities/workspace'
import { cn } from '@/shared/lib/cn'
import { IconButton, Skeleton } from '@/shared/ui'
import { WorkspaceHeader } from '@/widgets/app-shell/WorkspaceHeader'
import { ExplorerSearch } from './ExplorerSearch'
import { ObjectTree } from './ObjectTree'

type ExplorerPaneProps = {
  workspaces: Workspace[]
  activeWorkspace?: Workspace
  connections: Connection[]
  activeConnectionId?: string
  catalog?: CatalogTree
  catalogLoading?: boolean
  selectedObjectId?: string
  onWorkspaceChange: (workspaceId: string) => void
  onNewConnection: () => void
  onOpenCommandPalette: () => void
  onConnectionSelect: (connection: Connection) => void
  onObjectSelect: (object: DatabaseObject) => void
  onOpenObject: (object: DatabaseObject) => void
  onRefresh: () => void
}

export function ExplorerPane({
  workspaces,
  activeWorkspace,
  connections,
  activeConnectionId,
  catalog,
  catalogLoading,
  selectedObjectId,
  onWorkspaceChange,
  onNewConnection,
  onOpenCommandPalette,
  onConnectionSelect,
  onObjectSelect,
  onOpenObject,
  onRefresh,
}: ExplorerPaneProps) {
  const [search, setSearch] = useState('')

  return (
    <section className="flex h-full min-h-0 flex-col bg-surface/80" aria-label="Database explorer">
      <WorkspaceHeader
        workspaces={workspaces}
        activeWorkspace={activeWorkspace}
        connectionCount={connections.length}
        onWorkspaceChange={onWorkspaceChange}
        onNewConnection={onNewConnection}
        onOpenCommandPalette={onOpenCommandPalette}
      />
      <ExplorerSearch value={search} onChange={setSearch} />
      <div className="flex h-[var(--toolbar-height)] shrink-0 items-center gap-2 px-3 text-[length:var(--font-size-meta)] font-semibold tracking-[0.04em] text-muted-foreground uppercase">
        <ChevronDown className="size-3.5" />
        <span className="flex-1">Connections</span>
        <span className="font-mono text-[length:var(--font-size-meta)] font-medium tracking-normal">{connections.length}</span>
        <IconButton label="Refresh database explorer" size="icon-xs" onClick={onRefresh}>
          <RefreshCw />
        </IconButton>
      </div>
      <div className="min-h-0 flex-1">
        {catalogLoading ? (
          <div className="px-3 py-2">
            {Array.from({ length: 10 }, (_, index) => (
              <div key={index} className="flex h-[var(--tree-row-height)] items-center gap-2" style={{ paddingLeft: `${Math.min(index, 4) * 10}px` }}>
                <Skeleton className="size-4 shrink-0" />
                <Skeleton className={cn('h-4', index % 3 === 0 ? 'w-36' : 'w-24')} />
              </div>
            ))}
          </div>
        ) : connections.length ? (
          <ObjectTree
            connections={connections}
            activeConnectionId={activeConnectionId}
            catalog={catalog}
            search={search}
            selectedObjectId={selectedObjectId}
            onConnectionSelect={onConnectionSelect}
            onObjectSelect={onObjectSelect}
            onOpenObject={onOpenObject}
            onRefresh={onRefresh}
          />
        ) : (
          <div className="grid h-full place-items-center px-6 text-center">
            <div>
              <div className="mx-auto grid size-11 place-items-center rounded-xl border border-border bg-muted/50 text-muted-foreground">
                <Database className="size-5" />
              </div>
              <p className="mt-3 text-[length:var(--font-size-ui)] font-medium text-foreground">No connections yet</p>
              <p className="mt-1 text-[length:var(--font-size-meta)] leading-5 text-muted-foreground">Create one or open the demo workspace.</p>
            </div>
          </div>
        )}
      </div>
      <div className="flex h-[var(--tree-row-height)] shrink-0 items-center gap-2 border-t border-border px-3 text-[length:var(--font-size-meta)] text-muted-foreground">
        <Star className="size-3.5 text-amber-400" />
        <span className="truncate">Favorites are pinned first</span>
        <span className="ml-auto size-1.5 rounded-full bg-emerald-400" />
      </div>
    </section>
  )
}

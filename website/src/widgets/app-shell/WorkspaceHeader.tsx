import { useState } from 'react'
import { Check, ChevronsDown, ChevronsUp, ChevronsUpDown, MoreHorizontal, Pencil, Plus, Search, Settings2, Trash2 } from 'lucide-react'
import type { CreateWorkspaceInput, UpdateWorkspaceInput, Workspace } from '@/entities/workspace'
import { WORKSPACE_COLORS, WorkspaceMark } from '@/features/workspaces/WorkspaceMark'
import { WorkspaceFormDialog } from '@/features/workspaces/WorkspaceFormDialog'
import { WorkspaceManagerDialog } from '@/features/workspaces/WorkspaceManagerDialog'
import { APP_CONFIG } from '@/shared/config/constants'
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
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
  onCreateWorkspace: (input: CreateWorkspaceInput) => Promise<void>
  onUpdateWorkspace: (workspaceId: string, input: UpdateWorkspaceInput) => Promise<void>
  onDeleteWorkspace: (workspaceId: string) => Promise<void>
  onReorderWorkspaces: (ids: string[]) => Promise<void>
  onNewConnection: () => void
  onOpenCommandPalette: () => void
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'The workspace could not be deleted.'
}

export function WorkspaceHeader({
  workspaces,
  activeWorkspace,
  connectionCount,
  onWorkspaceChange,
  onCreateWorkspace,
  onUpdateWorkspace,
  onDeleteWorkspace,
  onReorderWorkspaces,
  onNewConnection,
  onOpenCommandPalette,
}: WorkspaceHeaderProps) {
  const [createOpen, setCreateOpen] = useState(false)
  const [managerOpen, setManagerOpen] = useState(false)
  const [editWorkspace, setEditWorkspace] = useState<Workspace>()
  const [deleteWorkspace, setDeleteWorkspace] = useState<Workspace>()
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')
  const defaultColor = WORKSPACE_COLORS[workspaces.length % WORKSPACE_COLORS.length]

  function openCreate() {
    setManagerOpen(false)
    setCreateOpen(true)
  }

  function openEdit(workspace: Workspace) {
    setManagerOpen(false)
    setEditWorkspace(workspace)
  }

  function openDelete(workspace: Workspace) {
    setManagerOpen(false)
    setDeleteError('')
    setDeleteWorkspace(workspace)
  }

  async function confirmDelete() {
    if (!deleteWorkspace) return
    setDeleting(true)
    setDeleteError('')
    try {
      await onDeleteWorkspace(deleteWorkspace.id)
      setDeleteWorkspace(undefined)
    } catch (error) {
      setDeleteError(errorMessage(error))
    } finally {
      setDeleting(false)
    }
  }

  async function toggleCollapsed(workspace: Workspace) {
    await onUpdateWorkspace(workspace.id, { name: workspace.name, icon: workspace.icon, color: workspace.color, collapsed: !workspace.collapsed })
  }

  return (
    <>
      <div className="border-b border-border bg-surface/90 px-3 pb-3 pt-3.5 backdrop-blur-xl">
        <div className="flex items-center gap-2">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" className="h-[var(--control-height)] min-w-0 flex-1 justify-start px-2 hover:bg-accent">
                {activeWorkspace ? <WorkspaceMark workspace={activeWorkspace} className="size-6 rounded-md" /> : <span className="grid size-6 shrink-0 place-items-center rounded-md bg-primary text-[length:var(--font-size-meta)] font-bold text-primary-foreground shadow-control">D</span>}
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
                  <WorkspaceMark workspace={workspace} className="size-6 rounded-md" />
                  <span className="min-w-0 flex-1 truncate">{workspace.name}</span>
                  {workspace.collapsed ? <ChevronsUp className="size-3.5 text-muted-foreground" /> : null}
                  {workspace.id === activeWorkspace?.id ? <Check className="text-primary" /> : null}
                </DropdownMenuItem>
              ))}
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={openCreate}>
                <Plus />
                Create workspace
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => setManagerOpen(true)}>
                <Settings2 />
                Manage workspaces
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <IconButton label="Workspace options" variant="ghost">
                <MoreHorizontal />
              </IconButton>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-52">
              <DropdownMenuLabel>{activeWorkspace?.name ?? APP_CONFIG.name}</DropdownMenuLabel>
              <DropdownMenuItem disabled={!activeWorkspace} onSelect={() => activeWorkspace && openEdit(activeWorkspace)}><Pencil />Edit appearance</DropdownMenuItem>
              <DropdownMenuItem disabled={!activeWorkspace} onSelect={() => { if (activeWorkspace) void toggleCollapsed(activeWorkspace).catch(() => undefined) }}>{activeWorkspace?.collapsed ? <ChevronsDown /> : <ChevronsUp />}{activeWorkspace?.collapsed ? 'Expand explorer' : 'Collapse explorer'}</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => setManagerOpen(true)}><Settings2 />Manage and reorder</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem disabled={!activeWorkspace} tone="destructive" onSelect={() => activeWorkspace && openDelete(activeWorkspace)}><Trash2 />Delete workspace</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <div className="mt-3 grid grid-cols-[1fr_auto] gap-2">
          <Button size="sm" disabled={!activeWorkspace} className="justify-start shadow-[0_7px_22px_color-mix(in_oklab,var(--primary)_18%,transparent)]" onClick={onNewConnection}>
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

      <WorkspaceFormDialog open={createOpen} mode="create" defaultColor={defaultColor} onOpenChange={setCreateOpen} onSubmit={onCreateWorkspace} />
      <WorkspaceFormDialog open={Boolean(editWorkspace)} mode="edit" workspace={editWorkspace} onOpenChange={(open) => { if (!open) setEditWorkspace(undefined) }} onSubmit={async (input) => { if (!editWorkspace) return; await onUpdateWorkspace(editWorkspace.id, { ...input, collapsed: editWorkspace.collapsed }) }} />
      <WorkspaceManagerDialog open={managerOpen} workspaces={workspaces} activeWorkspaceId={activeWorkspace?.id} onOpenChange={setManagerOpen} onWorkspaceChange={(id) => { setManagerOpen(false); onWorkspaceChange(id) }} onCreate={openCreate} onEdit={openEdit} onDelete={openDelete} onToggleCollapsed={toggleCollapsed} onReorder={onReorderWorkspaces} />
      <Dialog open={Boolean(deleteWorkspace)} onOpenChange={(open) => { if (!open && !deleting) { setDeleteWorkspace(undefined); setDeleteError('') } }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Delete {deleteWorkspace?.name}?</DialogTitle>
            <DialogDescription>This permanently removes the workspace and its saved connection profiles. Database contents are never changed.</DialogDescription>
          </DialogHeader>
          {deleteError ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-ui)] text-destructive">{deleteError}</div> : null}
          <DialogFooter>
            <Button variant="outline" disabled={deleting} onClick={() => setDeleteWorkspace(undefined)}>Cancel</Button>
            <Button variant="destructive" disabled={deleting} onClick={() => void confirmDelete()}>{deleting ? 'Deleting…' : 'Delete workspace'}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

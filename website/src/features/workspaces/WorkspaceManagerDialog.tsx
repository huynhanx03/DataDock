import { useEffect, useState, type CSSProperties } from 'react'
import { closestCenter, DndContext, KeyboardSensor, PointerSensor, useSensor, useSensors, type DragEndEvent } from '@dnd-kit/core'
import { arrayMove, SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { Check, ChevronsDown, ChevronsUp, GripVertical, Pencil, Plus, Trash2 } from 'lucide-react'
import type { Workspace } from '@/entities/workspace'
import { cn } from '@/shared/lib/cn'
import { Badge, Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, IconButton } from '@/shared/ui'
import { WorkspaceMark } from './WorkspaceMark'

type WorkspaceManagerDialogProps = {
  open: boolean
  workspaces: Workspace[]
  activeWorkspaceId?: string
  onOpenChange: (open: boolean) => void
  onWorkspaceChange: (workspaceId: string) => void
  onCreate: () => void
  onEdit: (workspace: Workspace) => void
  onDelete: (workspace: Workspace) => void
  onToggleCollapsed: (workspace: Workspace) => Promise<void>
  onReorder: (ids: string[]) => Promise<void>
}

type SortableWorkspaceRowProps = {
  workspace: Workspace
  active: boolean
  busy: boolean
  onSelect: () => void
  onEdit: () => void
  onDelete: () => void
  onToggleCollapsed: () => void
}

function SortableWorkspaceRow({ workspace, active, busy, onSelect, onEdit, onDelete, onToggleCollapsed }: SortableWorkspaceRowProps) {
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({ id: workspace.id, disabled: busy })
  const style: CSSProperties = { transform: CSS.Transform.toString(transform), transition, zIndex: isDragging ? 2 : undefined }

  return (
    <div ref={setNodeRef} style={style} className={cn('relative flex items-center gap-1.5 rounded-xl border bg-surface p-2 shadow-control transition-[border-color,background-color,opacity,box-shadow]', active ? 'border-primary/45 bg-accent/30' : 'border-border', isDragging && 'opacity-80 shadow-overlay')}>
      <IconButton ref={setActivatorNodeRef} label={`Move ${workspace.name}`} size="icon-xs" className="cursor-grab touch-none active:cursor-grabbing" {...attributes} {...listeners}><GripVertical /></IconButton>
      <button type="button" disabled={busy} onClick={onSelect} className="flex min-w-0 flex-1 items-center gap-2 rounded-lg px-1.5 py-1 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring/25 disabled:opacity-60">
        <WorkspaceMark workspace={workspace} />
        <span className="min-w-0 flex-1">
          <span className="flex items-center gap-1.5"><span className="truncate text-[length:var(--font-size-ui)] font-semibold text-foreground">{workspace.name}</span>{active ? <Check className="size-3.5 shrink-0 text-primary" /> : null}</span>
          <span className="mt-0.5 block text-[length:var(--font-size-meta)] text-muted-foreground">Position {workspace.position + 1}</span>
        </span>
      </button>
      {workspace.collapsed ? <Badge variant="outline" className="hidden sm:inline-flex">Collapsed</Badge> : null}
      <IconButton label={workspace.collapsed ? `Expand ${workspace.name}` : `Collapse ${workspace.name}`} size="icon-xs" disabled={busy} onClick={onToggleCollapsed}>{workspace.collapsed ? <ChevronsDown /> : <ChevronsUp />}</IconButton>
      <IconButton label={`Edit ${workspace.name}`} size="icon-xs" disabled={busy} onClick={onEdit}><Pencil /></IconButton>
      <IconButton label={`Delete ${workspace.name}`} size="icon-xs" variant="ghost" className="text-destructive hover:bg-destructive/10 hover:text-destructive" disabled={busy} onClick={onDelete}><Trash2 /></IconButton>
    </div>
  )
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'The workspace order could not be saved.'
}

export function WorkspaceManagerDialog({ open, workspaces, activeWorkspaceId, onOpenChange, onWorkspaceChange, onCreate, onEdit, onDelete, onToggleCollapsed, onReorder }: WorkspaceManagerDialogProps) {
  const [ordered, setOrdered] = useState(workspaces)
  const [busyId, setBusyId] = useState('')
  const [reordering, setReordering] = useState(false)
  const [actionError, setActionError] = useState('')
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }), useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }))

  useEffect(() => setOrdered([...workspaces].sort((left, right) => left.position - right.position)), [workspaces])
  useEffect(() => { if (!open) setActionError('') }, [open])

  async function dragEnd(event: DragEndEvent) {
    const activeId = String(event.active.id)
    const overId = event.over ? String(event.over.id) : ''
    if (!overId || activeId === overId) return
    const oldIndex = ordered.findIndex((workspace) => workspace.id === activeId)
    const newIndex = ordered.findIndex((workspace) => workspace.id === overId)
    if (oldIndex < 0 || newIndex < 0) return
    const next = arrayMove(ordered, oldIndex, newIndex).map((workspace, position) => ({ ...workspace, position }))
    setOrdered(next)
    setActionError('')
    setReordering(true)
    try {
      await onReorder(next.map((workspace) => workspace.id))
    } catch (error) {
      setOrdered([...workspaces].sort((left, right) => left.position - right.position))
      setActionError(errorMessage(error))
    } finally {
      setReordering(false)
    }
  }

  async function toggleCollapsed(workspace: Workspace) {
    setBusyId(workspace.id)
    setActionError('')
    try {
      await onToggleCollapsed(workspace)
    } catch (error) {
      setActionError(errorMessage(error))
    } finally {
      setBusyId('')
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!reordering && !busyId) onOpenChange(next) }}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Manage workspaces</DialogTitle>
          <DialogDescription>Switch, organize, collapse, and customize database groups from one place.</DialogDescription>
        </DialogHeader>
        {ordered.length ? (
          <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={(event) => void dragEnd(event)}>
            <SortableContext items={ordered.map((workspace) => workspace.id)} strategy={verticalListSortingStrategy}>
              <div className="grid max-h-[min(52vh,440px)] gap-2 overflow-y-auto pr-1">
                {ordered.map((workspace) => <SortableWorkspaceRow key={workspace.id} workspace={workspace} active={workspace.id === activeWorkspaceId} busy={reordering || Boolean(busyId)} onSelect={() => onWorkspaceChange(workspace.id)} onEdit={() => onEdit(workspace)} onDelete={() => onDelete(workspace)} onToggleCollapsed={() => void toggleCollapsed(workspace)} />)}
              </div>
            </SortableContext>
          </DndContext>
        ) : <div className="rounded-xl border border-dashed border-border bg-muted/25 px-5 py-8 text-center"><p className="text-[length:var(--font-size-ui)] font-semibold text-foreground">No workspaces yet</p><p className="mt-1 text-[length:var(--font-size-meta)] text-muted-foreground">Create one to start organizing database profiles.</p></div>}
        {actionError ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-ui)] text-destructive">{actionError}</div> : null}
        <DialogFooter className="sm:justify-between">
          <Button variant="outline" disabled={reordering || Boolean(busyId)} onClick={onCreate}><Plus />New workspace</Button>
          <Button variant="secondary" disabled={reordering || Boolean(busyId)} onClick={() => onOpenChange(false)}>{reordering ? 'Saving order…' : 'Done'}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

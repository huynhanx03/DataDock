import { useMemo, useState, type CSSProperties } from 'react'
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { Column, Table as TableModel } from '@tanstack/react-table'
import { Eye, EyeOff, GripVertical, PanelLeft, PanelRight, RotateCcw, Search } from 'lucide-react'
import type { DataColumn, TableRow } from '@/entities/database-object'
import { cn } from '@/shared/lib/cn'
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  IconButton,
  Input,
} from '@/shared/ui'

type SortableColumnRowProps = {
  column: Column<TableRow, unknown>
  definition: DataColumn
}

function SortableColumnRow({ column, definition }: SortableColumnRowProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: column.id })
  const pinned = column.getIsPinned()
  const style: CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    zIndex: isDragging ? 2 : undefined,
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        'group flex min-h-[var(--toolbar-height)] items-center gap-2 border-b border-border px-3 transition-colors last:border-b-0 hover:bg-accent/35',
        isDragging && 'rounded-lg border border-primary/35 bg-popover shadow-overlay',
        !column.getIsVisible() && 'opacity-60',
      )}
    >
      <button
        type="button"
        className="grid size-7 shrink-0 touch-none place-items-center rounded-md text-muted-foreground outline-none hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
        aria-label={`Reorder ${definition.name}`}
        {...attributes}
        {...listeners}
      >
        <GripVertical className="size-4" />
      </button>
      <button
        type="button"
        role="checkbox"
        aria-checked={column.getIsVisible()}
        className="grid size-7 shrink-0 place-items-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
        onClick={() => column.toggleVisibility()}
      >
        {column.getIsVisible() ? <Eye className="size-4" /> : <EyeOff className="size-4" />}
        <span className="sr-only">{column.getIsVisible() ? 'Hide' : 'Show'} {definition.name}</span>
      </button>
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <span className="truncate text-[length:var(--font-size-data)] font-medium text-foreground">{definition.name}</span>
        <Badge variant="outline" className="shrink-0 rounded px-1.5 font-mono text-[length:var(--font-size-meta)] font-normal text-muted-foreground">{definition.type}</Badge>
      </div>
      <div className="flex shrink-0 items-center gap-0.5">
        <IconButton
          label={pinned === 'left' ? `Unpin ${definition.name} from left` : `Pin ${definition.name} left`}
          size="icon-xs"
          variant={pinned === 'left' ? 'subtle' : 'ghost'}
          onClick={() => column.pin(pinned === 'left' ? false : 'left')}
        >
          <PanelLeft />
        </IconButton>
        <IconButton
          label={pinned === 'right' ? `Unpin ${definition.name} from right` : `Pin ${definition.name} right`}
          size="icon-xs"
          variant={pinned === 'right' ? 'subtle' : 'ghost'}
          onClick={() => column.pin(pinned === 'right' ? false : 'right')}
        >
          <PanelRight />
        </IconButton>
      </div>
    </div>
  )
}

type ColumnManagerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  table: TableModel<TableRow>
  definitions: DataColumn[]
  columnOrder: string[]
  onColumnMove: (activeId: string, overId: string) => void
  onResetOrder: () => void
  onResetWidths: () => void
  onResetLayout: () => void
}

export function ColumnManager({
  open,
  onOpenChange,
  table,
  definitions,
  columnOrder,
  onColumnMove,
  onResetOrder,
  onResetWidths,
  onResetLayout,
}: ColumnManagerProps) {
  const [search, setSearch] = useState('')
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )
  const definitionById = useMemo(() => new Map(definitions.map((definition) => [definition.key ?? definition.name, definition])), [definitions])
  const orderedIds = useMemo(() => {
    const ids = definitions.map((definition) => definition.key ?? definition.name)
    return [...columnOrder.filter((id) => ids.includes(id)), ...ids.filter((id) => !columnOrder.includes(id))]
  }, [columnOrder, definitions])
  const normalizedSearch = search.trim().toLowerCase()
  const visibleIds = orderedIds.filter((id) => {
    const definition = definitionById.get(id)
    return definition && (!normalizedSearch || definition.name.toLowerCase().includes(normalizedSearch) || definition.type.toLowerCase().includes(normalizedSearch))
  })
  const visibleCount = definitions.filter((definition) => table.getColumn(definition.key ?? definition.name)?.getIsVisible()).length

  function handleDragEnd(event: DragEndEvent) {
    const activeId = String(event.active.id)
    const overId = event.over ? String(event.over.id) : undefined
    if (!overId || activeId === overId) return
    if (!orderedIds.includes(activeId) || !orderedIds.includes(overId)) return
    onColumnMove(activeId, overId)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl gap-0 overflow-hidden p-0 sm:p-0">
        <DialogHeader className="border-b border-border px-5 py-4">
          <DialogTitle>Manage columns</DialogTitle>
          <DialogDescription>Choose what is visible, drag to reorder, and pin important fields while you explore data.</DialogDescription>
        </DialogHeader>
        <div className="flex items-center gap-3 border-b border-border bg-surface/45 px-4 py-3">
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input value={search} onChange={(event) => setSearch(event.target.value)} className="pl-9" placeholder="Find a column or data type…" autoFocus />
          </div>
          <Badge variant="secondary" className="shrink-0 text-[length:var(--font-size-meta)]">{visibleCount} / {definitions.length} visible</Badge>
        </div>
        <div className="flex flex-wrap items-center gap-2 border-b border-border px-4 py-2.5">
          <Button size="xs" variant="outline" onClick={() => table.toggleAllColumnsVisible(true)}>Show all</Button>
          <Button size="xs" variant="ghost" onClick={onResetOrder}>Reset order</Button>
          <Button size="xs" variant="ghost" onClick={onResetWidths}>Reset widths</Button>
          <Button size="xs" variant="ghost" className="ml-auto" onClick={onResetLayout}><RotateCcw />Reset layout</Button>
        </div>
        <div className="max-h-[min(58vh,520px)] overflow-y-auto bg-background/35">
          {visibleIds.length ? (
            <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
              <SortableContext items={visibleIds} strategy={verticalListSortingStrategy}>
                {visibleIds.map((id) => {
                  const column = table.getColumn(id)
                  const definition = definitionById.get(id)
                  return column && definition ? <SortableColumnRow key={id} column={column} definition={definition} /> : null
                })}
              </SortableContext>
            </DndContext>
          ) : (
            <div className="grid min-h-40 place-items-center px-5 text-center text-[length:var(--font-size-ui)] text-muted-foreground">No columns match “{search}”.</div>
          )}
        </div>
        <div className="flex items-center justify-between gap-3 border-t border-border bg-surface/55 px-5 py-3">
          <p className="text-[length:var(--font-size-meta)] text-muted-foreground">Column layout is saved for this table.</p>
          <Button size="sm" onClick={() => onOpenChange(false)}>Done</Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

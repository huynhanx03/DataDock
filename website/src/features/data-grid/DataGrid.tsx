import { useEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
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
  arrayMove,
  horizontalListSortingStrategy,
  sortableKeyboardCoordinates,
  useSortable,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type Column,
  type ColumnDef,
  type ColumnOrderState,
  type ColumnPinningState,
  type ColumnSizingState,
  type Header,
  type Updater,
  type VisibilityState,
} from '@tanstack/react-table'
import { useVirtualizer } from '@tanstack/react-virtual'
import { ChevronDown, ChevronsUpDown, Columns3, Filter, GripVertical, RefreshCw, Search, SlidersHorizontal } from 'lucide-react'
import type { DataColumn, TableRow } from '@/entities/database-object'
import { APP_CONFIG } from '@/shared/config/constants'
import { cn } from '@/shared/lib/cn'
import { Badge, Button, IconButton, Input, Skeleton } from '@/shared/ui'
import { ColumnManager } from '@/features/data-grid/ColumnManager'

const ROW_NUMBER_COLUMN_ID = '__datadock_row_number__'

type GridPreferences = {
  columnOrder: ColumnOrderState
  columnVisibility: VisibilityState
  columnSizing: ColumnSizingState
  columnPinning: ColumnPinningState
}

type DataGridProps = {
  connectionId: string
  tableName: string
  columns: DataColumn[]
  rows: TableRow[]
  page: number
  pageSize: number
  search: string
  searchPlaceholder?: string
  sortBy?: string
  sortDirection: 'asc' | 'desc'
  loading: boolean
  fetching: boolean
  error?: Error | null
  onSearchChange: (value: string) => void
  onSort: (column: string) => void
  onRefresh: () => void
  onRetry: () => void
}

function resolveUpdater<T>(updater: Updater<T>, previous: T) {
  return typeof updater === 'function' ? (updater as (value: T) => T)(previous) : updater
}

function normalizeOrder(order: string[], columnIds: string[]) {
  return [ROW_NUMBER_COLUMN_ID, ...order.filter((id) => id !== ROW_NUMBER_COLUMN_ID && columnIds.includes(id)), ...columnIds.filter((id) => !order.includes(id))]
}

function normalizePinning(pinning: ColumnPinningState, columnIds: string[]) {
  const left = (pinning.left ?? []).filter((id) => id !== ROW_NUMBER_COLUMN_ID && columnIds.includes(id))
  const right = (pinning.right ?? []).filter((id) => columnIds.includes(id) && !left.includes(id))
  return { left: [ROW_NUMBER_COLUMN_ID, ...left], right }
}

function visualColumnOrder(order: string[], pinning: ColumnPinningState, columnIds: string[]) {
  const normalized = normalizeOrder(order, columnIds).filter((id) => id !== ROW_NUMBER_COLUMN_ID)
  const left = (pinning.left ?? []).filter((id) => id !== ROW_NUMBER_COLUMN_ID && normalized.includes(id))
  const right = (pinning.right ?? []).filter((id) => normalized.includes(id) && !left.includes(id))
  const center = normalized.filter((id) => !left.includes(id) && !right.includes(id))
  return [...left, ...center, ...right]
}

function createDefaultPreferences(columnIds: string[]): GridPreferences {
  return {
    columnOrder: [ROW_NUMBER_COLUMN_ID, ...columnIds],
    columnVisibility: {},
    columnSizing: {},
    columnPinning: { left: [ROW_NUMBER_COLUMN_ID], right: [] },
  }
}

function loadPreferences(storageKey: string, columnIds: string[]): GridPreferences {
  const defaults = createDefaultPreferences(columnIds)
  if (typeof window === 'undefined') return defaults
  try {
    const raw = window.localStorage.getItem(storageKey)
    if (!raw) return defaults
    const stored = JSON.parse(raw) as Partial<GridPreferences>
    const visibility: VisibilityState = {}
    const sizing: ColumnSizingState = {}
    for (const id of columnIds) {
      if (stored.columnVisibility?.[id] === false) visibility[id] = false
      const width = stored.columnSizing?.[id]
      if (typeof width === 'number' && Number.isFinite(width)) sizing[id] = Math.min(APP_CONFIG.table.maxColumnWidth, Math.max(APP_CONFIG.table.minColumnWidth, width))
    }
    return {
      columnOrder: normalizeOrder(stored.columnOrder ?? [], columnIds),
      columnVisibility: visibility,
      columnSizing: sizing,
      columnPinning: normalizePinning(stored.columnPinning ?? {}, columnIds),
    }
  } catch {
    return defaults
  }
}

function savePreferences(storageKey: string, preferences: GridPreferences) {
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(preferences))
  } catch {
    return
  }
}

function estimateColumnWidth(column: DataColumn) {
  const name = column.name.toLowerCase()
  const type = column.type.toLowerCase()
  if (name === 'email' || name.endsWith('_email')) return 260
  if (name.includes('name') || name.includes('title')) return 200
  if (type.includes('timestamp') || type.includes('datetime')) return 240
  if (type === 'date' || type.includes('time')) return 190
  if (type.includes('bool')) return 126
  if (type.includes('json')) return 320
  if (type.includes('text')) return 260
  if (type.includes('numeric') || type.includes('decimal') || type.includes('int')) return 144
  if (name === 'id' || name.endsWith('_id') || type.includes('uuid')) return 176
  return APP_CONFIG.table.defaultColumnWidth
}

function formatValue(value: unknown) {
  if (value === null || value === undefined) return <span className="font-mono italic text-data-null">NULL</span>
  if (typeof value === 'boolean') return <Badge variant={value ? 'success' : 'outline'} className="text-[length:var(--font-size-meta)]">{String(value)}</Badge>
  if (typeof value === 'number') return <span className="font-mono text-data-number">{value.toLocaleString()}</span>
  if (typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T/.test(value)) return <span className="font-mono text-data-date">{new Date(value).toLocaleString()}</span>
  return <span className="font-mono text-data-string">{String(value)}</span>
}

function pinnedStyle(column: Column<TableRow, unknown>, lastLeftId?: string, firstRightId?: string): CSSProperties {
  const pinned = column.getIsPinned()
  return {
    position: pinned ? 'sticky' : 'relative',
    left: pinned === 'left' ? column.getStart('left') : undefined,
    right: pinned === 'right' ? column.getAfter('right') : undefined,
    zIndex: pinned ? 2 : 1,
    boxShadow: column.id === lastLeftId ? '5px 0 9px -8px color-mix(in oklab, var(--foreground) 45%, transparent)' : column.id === firstRightId ? '-5px 0 9px -8px color-mix(in oklab, var(--foreground) 45%, transparent)' : undefined,
  }
}

type SortableHeaderCellProps = {
  header: Header<TableRow, unknown>
  definition: DataColumn
  sortBy?: string
  sortDirection: 'asc' | 'desc'
  lastLeftId?: string
  firstRightId?: string
  columnIndex: number
  onSort: (column: string) => void
  onAutoSize: (column: string) => void
  onResizeBy: (column: string, delta: number) => void
}

function SortableHeaderCell({ header, definition, sortBy, sortDirection, lastLeftId, firstRightId, columnIndex, onSort, onAutoSize, onResizeBy }: SortableHeaderCellProps) {
  const column = header.column
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: column.id })
  const activeSort = sortBy === column.id
  return (
    <div
      ref={setNodeRef}
      role="columnheader"
      aria-colindex={columnIndex}
      aria-sort={activeSort ? sortDirection === 'asc' ? 'ascending' : 'descending' : 'none'}
      style={{ ...pinnedStyle(column, lastLeftId, firstRightId), transform: CSS.Transform.toString(transform), transition, zIndex: isDragging ? 30 : column.getIsPinned() ? 22 : 11 }}
      className={cn('group/header flex min-w-0 items-center border-r border-grid-line bg-grid-header/95', isDragging && 'opacity-85 shadow-overlay')}
    >
      <button type="button" onClick={() => onSort(column.id)} className="flex h-full min-w-0 flex-1 items-center gap-2 px-3 text-left hover:bg-accent/70">
        <span className="truncate text-[length:var(--font-size-data)] font-semibold text-foreground">{definition.name}</span>
        <Badge variant="outline" className="shrink-0 rounded px-1.5 font-mono text-[length:var(--font-size-meta)] font-normal text-muted-foreground">{definition.type}</Badge>
        {activeSort ? <ChevronDown className={cn('ml-auto size-3.5 shrink-0 text-primary transition-transform', sortDirection === 'asc' && 'rotate-180')} /> : <ChevronsUpDown className="ml-auto size-3.5 shrink-0 text-muted-foreground/45 opacity-0 transition-opacity group-hover/header:opacity-100" />}
      </button>
      <button type="button" className="mr-1 grid size-6 shrink-0 touch-none place-items-center rounded text-muted-foreground/55 opacity-0 hover:bg-accent hover:text-foreground group-hover/header:opacity-100 focus-visible:opacity-100" aria-label={`Reorder ${definition.name}`} {...attributes} {...listeners}>
        <GripVertical className="size-3.5" />
      </button>
      {header.column.getCanResize() ? <div role="separator" aria-orientation="vertical" aria-label={`Resize ${definition.name}`} aria-valuemin={APP_CONFIG.table.minColumnWidth} aria-valuemax={APP_CONFIG.table.maxColumnWidth} aria-valuenow={column.getSize()} tabIndex={0} onMouseDown={header.getResizeHandler()} onTouchStart={header.getResizeHandler()} onDoubleClick={(event) => { event.stopPropagation(); onAutoSize(column.id) }} onKeyDown={(event) => { if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') { event.preventDefault(); onResizeBy(column.id, event.key === 'ArrowLeft' ? -APP_CONFIG.table.keyboardResizeStep : APP_CONFIG.table.keyboardResizeStep) } if (event.key === 'Enter') { event.preventDefault(); onAutoSize(column.id) } }} className={cn('absolute top-0 right-0 z-30 h-full w-1 translate-x-1/2 touch-none cursor-col-resize select-none hover:bg-primary focus-visible:bg-primary', column.getIsResizing() && 'bg-primary')} /> : null}
    </div>
  )
}

export function DataGrid({
  connectionId,
  tableName,
  columns,
  rows,
  page,
  pageSize,
  search,
  searchPlaceholder = 'Search table rows…',
  sortBy,
  sortDirection,
  loading,
  fetching,
  error,
  onSearchChange,
  onSort,
  onRefresh,
  onRetry,
}: DataGridProps) {
  const [columnManagerOpen, setColumnManagerOpen] = useState(false)
  const scrollRef = useRef<HTMLDivElement>(null)
  const columnIds = useMemo(() => columns.map((column) => column.key ?? column.name), [columns])
  const columnSignature = columnIds.join('\u0000')
  const storageKey = `${APP_CONFIG.table.columnPreferencesStoragePrefix}:${connectionId}:${tableName}`
  const [preferences, setPreferences] = useState<GridPreferences>(() => loadPreferences(storageKey, columnIds))
  const preferencesReady = useRef(columnIds.length > 0)
  const definitionById = useMemo(() => new Map(columns.map((column) => [column.key ?? column.name, column])), [columns])

  useEffect(() => {
    if (!columnIds.length) return
    if (!preferencesReady.current) {
      preferencesReady.current = true
      setPreferences(loadPreferences(storageKey, columnIds))
      return
    }
    setPreferences((current) => ({
      columnOrder: normalizeOrder(current.columnOrder, columnIds),
      columnVisibility: Object.fromEntries(Object.entries(current.columnVisibility).filter(([id]) => columnIds.includes(id))),
      columnSizing: Object.fromEntries(Object.entries(current.columnSizing).filter(([id]) => columnIds.includes(id))),
      columnPinning: normalizePinning(current.columnPinning, columnIds),
    }))
  }, [columnSignature, storageKey])

  useEffect(() => {
    if (!preferencesReady.current || !columnIds.length) return
    const timeout = window.setTimeout(() => savePreferences(storageKey, preferences), APP_CONFIG.table.preferencesDebounceMs)
    return () => window.clearTimeout(timeout)
  }, [columnSignature, preferences, storageKey])

  function updatePreference<K extends keyof GridPreferences>(key: K, updater: Updater<GridPreferences[K]>) {
    setPreferences((current) => ({ ...current, [key]: resolveUpdater(updater, current[key]) }))
  }

  const columnDefinitions = useMemo<ColumnDef<TableRow>[]>(() => [
    {
      id: ROW_NUMBER_COLUMN_ID,
      header: '#',
      cell: ({ row }) => (page - 1) * pageSize + row.index + 1,
      size: APP_CONFIG.table.rowNumberWidth,
      minSize: APP_CONFIG.table.rowNumberWidth,
      maxSize: APP_CONFIG.table.rowNumberWidth,
      enableHiding: false,
      enablePinning: false,
      enableResizing: false,
    },
    ...columns.map((column) => ({
      id: column.key ?? column.name,
      accessorFn: (row: TableRow) => row[column.key ?? column.name],
      header: column.name,
      cell: ({ getValue }: { getValue: () => unknown }) => formatValue(getValue()),
      size: estimateColumnWidth(column),
      minSize: APP_CONFIG.table.minColumnWidth,
      maxSize: APP_CONFIG.table.maxColumnWidth,
    })),
  ], [columns, page, pageSize])

  const table = useReactTable({
    data: rows,
    columns: columnDefinitions,
    state: preferences,
    onColumnOrderChange: (updater) => updatePreference('columnOrder', updater),
    onColumnVisibilityChange: (updater) => updatePreference('columnVisibility', updater),
    onColumnSizingChange: (updater) => updatePreference('columnSizing', updater),
    onColumnPinningChange: (updater) => updatePreference('columnPinning', updater),
    getCoreRowModel: getCoreRowModel(),
    getRowId: (row, index) => String(row.id ?? `${page}:${index}`),
    columnResizeMode: 'onChange',
    enableColumnResizing: true,
  })

  const rowModel = table.getRowModel().rows
  const virtualizer = useVirtualizer({
    count: rowModel.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => APP_CONFIG.ui.densities[APP_CONFIG.ui.defaultDensity].dataRowHeight,
    overscan: APP_CONFIG.table.rowOverscan,
  })
  const leftColumns = table.getLeftVisibleLeafColumns()
  const centerColumns = table.getCenterVisibleLeafColumns()
  const rightColumns = table.getRightVisibleLeafColumns()
  const visibleColumns = [...leftColumns, ...centerColumns, ...rightColumns]
  const lastLeftId = leftColumns.at(-1)?.id
  const firstRightId = rightColumns[0]?.id
  const gridTemplateColumns = `${[...leftColumns, ...centerColumns].map((column) => `${column.getSize()}px`).join(' ')} minmax(0, 1fr) ${rightColumns.map((column) => `${column.getSize()}px`).join(' ')}`
  const totalWidth = visibleColumns.reduce((total, column) => total + column.getSize(), 0)
  const visibleDataCount = visibleColumns.filter((column) => column.id !== ROW_NUMBER_COLUMN_ID).length
  const visibleColumnIndex = new Map(visibleColumns.map((column, index) => [column.id, index + 1]))
  const headers = new Map(table.getFlatHeaders().map((header) => [header.column.id, header]))
  const sortableHeaderIds = visibleColumns.filter((column) => column.id !== ROW_NUMBER_COLUMN_ID).map((column) => column.id)
  const displayedColumnOrder = visualColumnOrder(preferences.columnOrder, preferences.columnPinning, columnIds)
  const headerSensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  function setDataColumnOrder(order: string[]) {
    setPreferences((current) => {
      const left = new Set((current.columnPinning.left ?? []).filter((id) => id !== ROW_NUMBER_COLUMN_ID))
      const right = new Set(current.columnPinning.right ?? [])
      return {
        ...current,
        columnOrder: [ROW_NUMBER_COLUMN_ID, ...order],
        columnPinning: {
          left: [ROW_NUMBER_COLUMN_ID, ...order.filter((id) => left.has(id))],
          right: order.filter((id) => right.has(id)),
        },
      }
    })
  }

  function moveDataColumn(activeId: string, overId: string) {
    setPreferences((current) => {
      const orderedDataIds = visualColumnOrder(current.columnOrder, current.columnPinning, columnIds)
      const oldIndex = orderedDataIds.indexOf(activeId)
      const newIndex = orderedDataIds.indexOf(overId)
      if (oldIndex < 0 || newIndex < 0) return current
      const nextOrder = arrayMove(orderedDataIds, oldIndex, newIndex)
      const left = new Set((current.columnPinning.left ?? []).filter((id) => id !== ROW_NUMBER_COLUMN_ID))
      const right = new Set(current.columnPinning.right ?? [])
      const overPin = left.has(overId) ? 'left' : right.has(overId) ? 'right' : false
      left.delete(activeId)
      right.delete(activeId)
      if (overPin === 'left') left.add(activeId)
      if (overPin === 'right') right.add(activeId)
      return {
        ...current,
        columnOrder: [ROW_NUMBER_COLUMN_ID, ...nextOrder],
        columnPinning: {
          left: [ROW_NUMBER_COLUMN_ID, ...nextOrder.filter((id) => left.has(id))],
          right: nextOrder.filter((id) => right.has(id)),
        },
      }
    })
  }

  function handleHeaderDragEnd(event: DragEndEvent) {
    const activeId = String(event.active.id)
    const overId = event.over ? String(event.over.id) : undefined
    if (!overId || activeId === overId) return
    moveDataColumn(activeId, overId)
  }

  function autoSizeColumn(columnId: string) {
    const definition = definitionById.get(columnId)
    if (!definition) return
    const longestValue = rows.slice(0, APP_CONFIG.table.autoSizeSampleRows).reduce((longest, row) => Math.max(longest, String(row[columnId] ?? 'NULL').length), 0)
    const characterCount = Math.min(80, Math.max(definition.name.length, definition.type.length, longestValue))
    const width = Math.min(APP_CONFIG.table.maxColumnWidth, Math.max(APP_CONFIG.table.minColumnWidth, characterCount * 8 + 54))
    updatePreference('columnSizing', (current) => ({ ...current, [columnId]: width }))
  }

  function resizeColumnBy(columnId: string, delta: number) {
    const column = table.getColumn(columnId)
    if (!column) return
    const width = Math.min(APP_CONFIG.table.maxColumnWidth, Math.max(APP_CONFIG.table.minColumnWidth, column.getSize() + delta))
    updatePreference('columnSizing', (current) => ({ ...current, [columnId]: width }))
  }

  function resetOrder() {
    setDataColumnOrder(columnIds)
  }

  function resetWidths() {
    updatePreference('columnSizing', {})
  }

  function resetLayout() {
    setPreferences(createDefaultPreferences(columnIds))
  }

  function renderHeaderCell(column: Column<TableRow, unknown>) {
    const header = headers.get(column.id)
    if (!header) return null
    if (column.id === ROW_NUMBER_COLUMN_ID) {
      return <div key={column.id} role="columnheader" aria-colindex={visibleColumnIndex.get(column.id)} style={{ ...pinnedStyle(column, lastLeftId, firstRightId), zIndex: 22 }} className="flex items-center justify-end border-r border-grid-line bg-grid-header/95 px-3 font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{flexRender(header.column.columnDef.header, header.getContext())}</div>
    }
    const definition = definitionById.get(column.id)
    return definition ? <SortableHeaderCell key={column.id} header={header} definition={definition} sortBy={sortBy} sortDirection={sortDirection} lastLeftId={lastLeftId} firstRightId={firstRightId} columnIndex={visibleColumnIndex.get(column.id) ?? 1} onSort={onSort} onAutoSize={autoSizeColumn} onResizeBy={resizeColumnBy} /> : null
  }

  return (
    <div className="flex min-h-0 min-w-0 w-full flex-1 flex-col overflow-hidden">
      <div className="flex h-[var(--toolbar-height)] shrink-0 items-center gap-2 border-b border-border bg-surface/45 px-4 lg:px-6">
        <div className="relative w-full max-w-sm">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={search} onChange={(event) => onSearchChange(event.target.value)} className="pl-9" placeholder={searchPlaceholder} />
        </div>
        <Button variant="outline" size="sm"><Filter />Filter</Button>
        <Button variant="outline" size="sm"><SlidersHorizontal />Sort</Button>
        <Button variant="outline" size="sm" onClick={() => setColumnManagerOpen(true)}><Columns3 />Columns <span className="font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{visibleDataCount}/{columns.length}</span></Button>
        <IconButton label="Refresh rows" className="ml-auto" onClick={onRefresh}><RefreshCw className={fetching ? 'animate-spin' : ''} /></IconButton>
      </div>

      <div ref={scrollRef} className="min-h-0 min-w-0 w-full max-w-full flex-1 overflow-auto bg-background">
        {loading ? (
          <div className="space-y-px p-3">{Array.from({ length: 12 }, (_, index) => <Skeleton key={index} className="h-[var(--data-row-height)] w-full rounded-sm" />)}</div>
        ) : error ? (
          <div className="grid h-full place-items-center"><div className="text-center"><p className="font-medium text-foreground">Could not load table rows</p><p className="mt-1 text-[length:var(--font-size-ui)] text-muted-foreground">{error.message}</p><Button className="mt-4" variant="outline" onClick={onRetry}>Try again</Button></div></div>
        ) : (
          <div role="table" aria-label={`${tableName} data`} aria-rowcount={rowModel.length + 1} aria-colcount={visibleColumns.length} style={{ width: totalWidth, minWidth: '100%' }}>
            <DndContext sensors={headerSensors} collisionDetection={closestCenter} onDragEnd={handleHeaderDragEnd}>
              <SortableContext items={sortableHeaderIds} strategy={horizontalListSortingStrategy}>
                <div role="row" aria-rowindex={1} className="sticky top-0 z-10 grid h-[var(--data-row-height)] border-b border-border bg-grid-header/95 shadow-[0_1px_0_var(--border)] backdrop-blur" style={{ gridTemplateColumns }}>
                  {[...leftColumns, ...centerColumns].map(renderHeaderCell)}
                  <div role="presentation" className="bg-grid-header/95" />
                  {rightColumns.map(renderHeaderCell)}
                </div>
              </SortableContext>
            </DndContext>
            {rowModel.length ? (
              <div role="rowgroup" className="relative" style={{ height: virtualizer.getTotalSize() }}>
                {virtualizer.getVirtualItems().map((virtualRow) => {
                  const row = rowModel[virtualRow.index]
                  const cells = new Map(row.getVisibleCells().map((cell) => [cell.column.id, cell]))
                  const renderCell = (column: Column<TableRow, unknown>) => {
                    const cell = cells.get(column.id)
                    if (!cell) return null
                    const rowNumber = column.id === ROW_NUMBER_COLUMN_ID
                    return <div key={cell.id} role="cell" aria-colindex={visibleColumnIndex.get(column.id)} style={pinnedStyle(column, lastLeftId, firstRightId)} className={cn('flex min-w-0 items-center overflow-hidden border-r border-grid-line px-3 whitespace-nowrap', rowNumber ? 'justify-end bg-background font-mono text-[length:var(--font-size-meta)] text-muted-foreground group-hover/row:bg-accent/35' : column.getIsPinned() && 'bg-background group-hover/row:bg-accent/35')}><span className="truncate">{flexRender(cell.column.columnDef.cell, cell.getContext())}</span></div>
                  }
                  return <div key={row.id} ref={virtualizer.measureElement} data-index={virtualRow.index} role="row" aria-rowindex={virtualRow.index + 2} className="group/row absolute left-0 grid h-[var(--data-row-height)] min-w-full border-b border-grid-line text-[length:var(--font-size-data)] hover:bg-accent/35" style={{ width: totalWidth, transform: `translateY(${virtualRow.start}px)`, gridTemplateColumns }}>
                    {[...leftColumns, ...centerColumns].map(renderCell)}
                    <div role="presentation" />
                    {rightColumns.map(renderCell)}
                  </div>
                })}
              </div>
            ) : (
              <div className="grid min-h-56 place-items-center px-6 text-center"><div><p className="font-medium text-foreground">No rows found</p><p className="mt-1 text-[length:var(--font-size-ui)] text-muted-foreground">Try changing the search or filter conditions.</p></div></div>
            )}
          </div>
        )}
      </div>

      <ColumnManager open={columnManagerOpen} onOpenChange={setColumnManagerOpen} table={table} definitions={columns} columnOrder={displayedColumnOrder} onColumnMove={moveDataColumn} onResetOrder={resetOrder} onResetWidths={resetWidths} onResetLayout={resetLayout} />
    </div>
  )
}

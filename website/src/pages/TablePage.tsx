import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, Clipboard, CopyPlus, DatabaseZap, Download, MoreHorizontal, PencilLine, Plus, RefreshCw, Rows3, Table2, Trash2 } from 'lucide-react'
import type { Connection } from '@/entities/connection'
import type { TableRow } from '@/entities/database-object'
import { useDataDockGateway } from '@/app/providers'
import { DataGrid } from '@/features/data-grid/DataGrid'
import { BatchUpdateDialog } from '@/features/data-editor/BatchUpdateDialog'
import { StagedChangesPanel } from '@/features/data-editor/StagedChangesPanel'
import { dataRowIdentity, useStagedMutations } from '@/features/data-editor/useStagedMutations'
import { APP_CONFIG } from '@/shared/config/constants'
import {
  Badge,
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
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  IconButton,
  WorkspaceHeader,
  WorkspacePage,
  WorkspaceToolbar,
} from '@/shared/ui'

type TablePageProps = {
  connection: Connection
  table: string
  onOpenSchema?: () => void
  onDirtyChange?: (dirty: boolean) => void
  onApplyingChange?: (applying: boolean) => void
}

function csvValue(value: unknown) {
  const rendered = value === null || value === undefined ? '' : typeof value === 'object' ? JSON.stringify(value) : String(value)
  return `"${rendered.replaceAll('"', '""')}"`
}

export function TablePage({ connection, table, onOpenSchema, onDirtyChange, onApplyingChange }: TablePageProps) {
  const gateway = useDataDockGateway()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [querySearch, setQuerySearch] = useState('')
  const [page, setPage] = useState(1)
  const [sortBy, setSortBy] = useState<string>()
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('asc')
  const [editing, setEditing] = useState(false)
  const [batchUpdateOpen, setBatchUpdateOpen] = useState(false)
  const [discardEditOpen, setDiscardEditOpen] = useState(false)
  const [selectedRowIds, setSelectedRowIds] = useState<Set<string>>(new Set())
  const pageSize = APP_CONFIG.table.defaultPageSize

  useEffect(() => {
    const timeout = window.setTimeout(() => setQuerySearch(search), 220)
    return () => window.clearTimeout(timeout)
  }, [search])

  useEffect(() => {
    setSelectedRowIds(new Set())
    setEditing(false)
  }, [connection.id, table])

  useEffect(() => {
    setSelectedRowIds(new Set())
  }, [page, querySearch, sortBy, sortDirection])

  const result = useQuery({
    queryKey: ['table-rows', connection.id, table, page, querySearch, sortBy, sortDirection],
    queryFn: ({ signal }) => gateway.getTableRows(connection.id, table, { page, pageSize, search: querySearch, sortBy, sortDirection }, signal),
  })
  const schemaResult = useQuery({
    queryKey: ['table-schema', connection.id, table],
    queryFn: ({ signal }) => gateway.getTableSchema(connection.id, table, signal),
  })
  const tableName = table.split('.').at(-1) ?? table
  const schemaName = table.includes('.') ? table.split('.')[0] : connection.database
  const keyColumns = useMemo(() => {
    if (result.data?.primaryKeyColumns?.length) return result.data.primaryKeyColumns
    const primaryConstraint = schemaResult.data?.constraints.find((constraint) => constraint.type.toUpperCase().includes('PRIMARY'))
    return primaryConstraint?.columns.length ? primaryConstraint.columns : []
  }, [result.data?.primaryKeyColumns, schemaResult.data?.constraints])
  const staged = useStagedMutations({ columns: result.data?.columns ?? [], keyColumns })
  const rows = useMemo(() => staged.displayRows(result.data?.rows ?? []), [result.data?.rows, staged.displayRows])
  const selectedRows = useMemo(() => rows.filter((row, index) => selectedRowIds.has(dataRowIdentity(row, keyColumns, index))), [keyColumns, rows, selectedRowIds])
  const total = result.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const rangeStart = total ? (page - 1) * pageSize + 1 : 0
  const rangeEnd = Math.min(page * pageSize, total)
  const canEdit = !connection.readOnly && keyColumns.length > 0
  const canInsert = canEdit && Boolean(result.data?.columns.some((column) => !column.generated && !column.identity))

  useEffect(() => {
    onDirtyChange?.(staged.changes.length > 0)
  }, [onDirtyChange, staged.changes.length])

  useEffect(() => () => onDirtyChange?.(false), [onDirtyChange])

  const apply = useMutation({
    mutationFn: async () => {
      const mutationResult = await gateway.mutateTableRows(connection.id, table, staged.mutations)
      if (mutationResult.status === 'conflict') {
        const conflicts = mutationResult.conflicts ?? []
        const summary = conflicts.map((conflict) => `#${conflict.index + 1} ${conflict.kind}: ${conflict.reason.replaceAll('_', ' ')}`).join('; ')
        throw new Error(`The atomic batch was not applied because the source rows changed${summary ? ` (${summary})` : ''}. Refresh the table and review the staged values.`)
      }
      if (mutationResult.atomic === false || mutationResult.applied !== staged.mutations.length) throw new Error('The database did not confirm the complete atomic mutation batch.')
      return mutationResult
    },
    onSuccess: async () => {
      staged.discard()
      setSelectedRowIds(new Set())
      await queryClient.invalidateQueries({ queryKey: ['table-rows', connection.id, table] })
    },
  })

  useEffect(() => {
    onApplyingChange?.(apply.isPending)
  }, [apply.isPending, onApplyingChange])

  useEffect(() => () => onApplyingChange?.(false), [onApplyingChange])

  function toggleSort(column: string) {
    if (sortBy === column) setSortDirection((value) => value === 'asc' ? 'desc' : 'asc')
    else {
      setSortBy(column)
      setSortDirection('asc')
    }
    setPage(1)
  }

  function beginEditing() {
    if (!canEdit) return
    setEditing(true)
  }

  function addRow() {
    if (!canInsert) return
    setEditing(true)
    const rowId = staged.insertRow()
    setSelectedRowIds(new Set([rowId]))
  }

  function duplicateSelected() {
    if (!canInsert || !selectedRows.length) return
    setEditing(true)
    const ids = staged.duplicateRows(selectedRows)
    setSelectedRowIds(new Set(ids))
  }

  function deleteSelected() {
    if (!canEdit || !selectedRows.length) return
    staged.deleteRows(selectedRows)
    setSelectedRowIds(new Set())
  }

  async function copySelected() {
    if (!selectedRows.length) return
    const columns = result.data?.columns.map((column) => column.key ?? column.name) ?? []
    const text = [columns.join('\t'), ...selectedRows.map((row) => columns.map((column) => String(row[column] ?? '')).join('\t'))].join('\n')
    await navigator.clipboard.writeText(text)
  }

  function exportRows() {
    const columns = result.data?.columns.map((column) => column.key ?? column.name) ?? []
    const exportData = selectedRows.length ? selectedRows : rows
    const csv = [columns.map(csvValue).join(','), ...exportData.map((row) => columns.map((column) => csvValue(row[column])).join(','))].join('\n')
    const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }))
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `${tableName}.csv`
    anchor.click()
    URL.revokeObjectURL(url)
  }

  function stopEditing() {
    setEditing(false)
    setSelectedRowIds(new Set())
    staged.discard()
  }

  function requestStopEditing() {
    if (staged.changes.length) setDiscardEditOpen(true)
    else stopEditing()
  }

  return (
    <WorkspacePage>
      <WorkspaceHeader
        icon={<Table2 />}
        eyebrow="Data browser"
        title={tableName}
        description={<><span className="font-mono text-[length:var(--font-size-data)]">{schemaName}.{tableName}</span><span className="mx-2">·</span>{total.toLocaleString()} rows<span className="mx-2">·</span>{result.data?.durationMs ?? 0} ms</>}
        meta={<Badge variant={editing ? 'warning' : 'outline'}>{editing ? 'Editing' : 'Browse'}</Badge>}
        compact
        actions={<>
          <Button variant="outline" size="sm" onClick={exportRows}><Download />Export</Button>
          {onOpenSchema ? <Button variant="outline" size="sm" onClick={onOpenSchema}><Rows3 />Structure</Button> : null}
          <DropdownMenu>
            <DropdownMenuTrigger asChild><IconButton label="More table actions" variant="outline"><MoreHorizontal /></IconButton></DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => result.refetch()}><RefreshCw />Refresh rows</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => navigator.clipboard.writeText(`${schemaName}.${tableName}`)}><Clipboard />Copy qualified name</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={onOpenSchema} disabled={!onOpenSchema}><DatabaseZap />Open schema studio</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button size="sm" variant={editing ? 'subtle' : 'outline'} disabled={!canEdit} onClick={editing ? requestStopEditing : beginEditing}><PencilLine />{editing ? 'Exit editing' : 'Edit data'}</Button>
          <Button size="sm" disabled={!canInsert} onClick={addRow}><Plus />New row</Button>
        </>}
      />

      {!canEdit ? (
        <div className="flex min-h-10 shrink-0 items-center gap-2 border-b border-warning/30 bg-warning/55 px-5 text-[length:var(--font-size-ui)] text-warning-foreground">
          <AlertTriangle className="size-4 shrink-0" />
          {connection.readOnly ? 'This connection is read-only. Data editing actions are unavailable.' : 'A primary key is required for safe update and delete operations.'}
        </div>
      ) : null}

      {editing && selectedRowIds.size ? (
        <WorkspaceToolbar className="border-b-primary/20 bg-accent/45">
          <Badge variant="accent">{selectedRowIds.size} selected</Badge>
          <Button size="xs" variant="outline" onClick={() => setBatchUpdateOpen(true)}><PencilLine />Batch update</Button>
          <Button size="xs" variant="outline" disabled={!canInsert} onClick={duplicateSelected}><CopyPlus />Duplicate</Button>
          <Button size="xs" variant="outline" onClick={copySelected}><Clipboard />Copy rows</Button>
          <Button size="xs" variant="outline" className="text-destructive hover:text-destructive" onClick={deleteSelected}><Trash2 />Delete</Button>
          <Button size="xs" variant="ghost" className="ml-auto" onClick={() => setSelectedRowIds(new Set())}>Clear selection</Button>
        </WorkspaceToolbar>
      ) : null}

      <DataGrid
        key={`${connection.id}:${table}`}
        connectionId={connection.id}
        tableName={table}
        columns={result.data?.columns ?? []}
        rows={rows}
        page={page}
        pageSize={pageSize}
        search={search}
        sortBy={sortBy}
        sortDirection={sortDirection}
        loading={result.isLoading}
        fetching={result.isFetching}
        error={result.error}
        onSearchChange={(value) => { setSearch(value); setPage(1) }}
        onSort={toggleSort}
        onRefresh={() => result.refetch()}
        onRetry={() => result.refetch()}
        editing={editing}
        pendingCells={staged.pendingCells}
        insertedRowIds={staged.insertedRowIds}
        deletedRowIds={staged.deletedRowIds}
        validationErrors={staged.validationErrors}
        selectedRowIds={selectedRowIds}
        keyColumns={keyColumns}
        onSelectedRowIdsChange={setSelectedRowIds}
        onEditCell={staged.editCell}
      />

      {editing ? (
        <StagedChangesPanel
          table={`${schemaName}.${tableName}`}
          changes={staged.changes}
          summary={staged.summary}
          validationErrors={staged.validationErrors}
          canUndo={staged.canUndo}
          canRedo={staged.canRedo}
          applying={apply.isPending}
          applyError={apply.error}
          onUndo={staged.undo}
          onRedo={staged.redo}
          onDiscard={() => { staged.discard(); setSelectedRowIds(new Set()) }}
          onDiscardChange={staged.discardChange}
          onApply={() => apply.mutate()}
        />
      ) : null}

      <footer className="flex h-[var(--toolbar-height)] shrink-0 items-center border-t border-border bg-surface/70 px-4 text-[length:var(--font-size-meta)] text-muted-foreground lg:px-5">
        <span>{result.data ? `${rangeStart.toLocaleString()}–${rangeEnd.toLocaleString()} of ${total.toLocaleString()}` : 'Loading rows…'}</span>
        {staged.changes.length ? <span className="ml-3 text-primary">{staged.changes.length} staged</span> : null}
        <div className="ml-auto flex items-center gap-2"><Button size="xs" variant="outline" disabled={page <= 1} onClick={() => setPage((value) => value - 1)}>Previous</Button><span className="px-2 font-mono">{page} / {totalPages}</span><Button size="xs" variant="outline" disabled={page >= totalPages} onClick={() => setPage((value) => value + 1)}>Next</Button></div>
      </footer>

      <BatchUpdateDialog open={batchUpdateOpen} onOpenChange={setBatchUpdateOpen} columns={result.data?.columns ?? []} keyColumns={keyColumns} selectedCount={selectedRows.length} onApply={(column, value) => staged.updateRows(selectedRows, column, value)} />
      <Dialog open={discardEditOpen} onOpenChange={setDiscardEditOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader><DialogTitle>Discard staged changes?</DialogTitle><DialogDescription>{staged.changes.length} staged {staged.changes.length === 1 ? 'change has' : 'changes have'} not been applied to {schemaName}.{tableName}.</DialogDescription></DialogHeader>
          <DialogFooter><Button variant="outline" onClick={() => setDiscardEditOpen(false)}>Keep editing</Button><Button variant="destructive" onClick={() => { setDiscardEditOpen(false); stopEditing() }}><Trash2 />Discard and exit</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </WorkspacePage>
  )
}

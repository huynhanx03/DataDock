import { useCallback, useMemo, useState } from 'react'
import type { DataColumn, RowMutation, TableRow } from '@/entities/database-object'
import { editorValueKind, validateEditorValue } from '@/features/data-editor/value-coercion'

export const DRAFT_ROW_KEY = '__datadockDraftRowId'

export type StagedCellUpdate = {
  id: string
  kind: 'update'
  rowId: string
  column: string
  keys: TableRow
  expectedValues: TableRow
  original: unknown
  value: unknown
}

export type StagedInsert = {
  id: string
  kind: 'insert'
  rowId: string
  values: TableRow
  source: 'new' | 'duplicate'
}

export type StagedDelete = {
  id: string
  kind: 'delete'
  rowId: string
  original: TableRow
}

export type StagedChange = StagedCellUpdate | StagedInsert | StagedDelete

type Snapshot = StagedChange[]

type LedgerState = {
  changes: StagedChange[]
  undoStack: Snapshot[]
  redoStack: Snapshot[]
}

type StagedMutationOptions = {
  columns: DataColumn[]
  keyColumns?: string[]
}

function sameValue(left: unknown, right: unknown) {
  if (typeof left === 'object' || typeof right === 'object') return JSON.stringify(left) === JSON.stringify(right)
  return Object.is(left, right)
}

function copyChanges(changes: StagedChange[]): StagedChange[] {
  return changes.map((change) => change.kind === 'insert'
    ? { ...change, values: { ...change.values } }
    : change.kind === 'delete'
      ? { ...change, original: { ...change.original } }
      : { ...change, keys: { ...change.keys }, expectedValues: { ...change.expectedValues } })
}

function draftId() {
  const suffix = typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID().slice(0, 8)
    : Math.random().toString(36).slice(2, 10)
  return `draft_${suffix}`
}

let numericDraftSequence = 0

function draftKeyValue(column: DataColumn, draftRowId: string) {
  const kind = editorValueKind(column.type)
  if (kind === 'exact-integer' || kind === 'exact-decimal') {
    numericDraftSequence = (numericDraftSequence + 1) % 1000
    return `-${Date.now()}${String(numericDraftSequence).padStart(3, '0')}`
  }
  if (kind === 'integer' || kind === 'float') {
    numericDraftSequence = (numericDraftSequence + 1) % 1000
    return -(Date.now() * 1000 + numericDraftSequence)
  }
  if (column.type.toLowerCase().includes('uuid') && typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  return draftRowId
}

export function dataRowIdentity(row: TableRow, keyColumns: string[], index = 0) {
  if (row[DRAFT_ROW_KEY] !== undefined) return String(row[DRAFT_ROW_KEY])
  const keyValues = keyColumns.map((key) => row[key])
  if (keyValues.length && keyValues.every((value) => value !== undefined)) return `pk:${JSON.stringify(keyValues)}`
  if (row.id !== undefined) return `id:${String(row.id)}`
  return `row_${index}`
}

function defaultValue(column: DataColumn, draftRowId: string, keyColumns: string[]) {
  const type = column.type.toLowerCase()
  const kind = editorValueKind(column.type)
  const name = column.key ?? column.name
  if (keyColumns.includes(name)) return draftKeyValue(column, draftRowId)
  if (column.nullable) return null
  if (type.includes('bool')) return false
  if (kind === 'exact-integer' || kind === 'exact-decimal') return '0'
  if (kind === 'integer' || kind === 'float') return 0
  if (type.includes('json')) return {}
  if (type === 'date') return new Date().toISOString().slice(0, 10)
  if (type.includes('timestamp') || type.includes('datetime')) return new Date().toISOString()
  return ''
}

function stripInternalValues(values: TableRow) {
  return Object.fromEntries(Object.entries(values).filter(([key]) => key !== DRAFT_ROW_KEY))
}

function insertMutationValues(values: TableRow, columns: DataColumn[]) {
  const writable = new Set(columns.filter((column) => !column.generated && !column.identity).map((column) => column.key ?? column.name))
  return Object.fromEntries(Object.entries(stripInternalValues(values)).filter(([key]) => writable.has(key)))
}

export function useStagedMutations({ columns, keyColumns = ['id'] }: StagedMutationOptions) {
  const [ledger, setLedger] = useState<LedgerState>({ changes: [], undoStack: [], redoStack: [] })
  const changes = ledger.changes

  const commit = useCallback((recipe: (current: StagedChange[]) => StagedChange[]) => {
    setLedger((current) => {
      const next = recipe(copyChanges(current.changes))
      if (JSON.stringify(next) === JSON.stringify(current.changes)) return current
      return {
        changes: next,
        undoStack: [...current.undoStack.slice(-49), copyChanges(current.changes)],
        redoStack: [],
      }
    })
  }, [])

  const editCell = useCallback((row: TableRow, column: string, value: unknown) => {
    const rowId = dataRowIdentity(row, keyColumns)
    commit((current) => {
      const inserted = current.find((change): change is StagedInsert => change.kind === 'insert' && change.rowId === rowId)
      if (inserted) {
        inserted.values[column] = value
        return current
      }
      const withoutCell = current.filter((change) => !(change.kind === 'update' && change.rowId === rowId && change.column === column))
      const previous = current.find((change): change is StagedCellUpdate => change.kind === 'update' && change.rowId === rowId && change.column === column)
      const previousRowUpdate = current.find((change): change is StagedCellUpdate => change.kind === 'update' && change.rowId === rowId)
      const original = previous?.original ?? row[column]
      return sameValue(original, value)
        ? withoutCell
        : [...withoutCell, { id: `update:${rowId}:${column}`, kind: 'update', rowId, column, keys: Object.fromEntries(keyColumns.map((key) => [key, row[key]])), expectedValues: previous?.expectedValues ?? previousRowUpdate?.expectedValues ?? stripInternalValues(row), original, value }]
    })
  }, [commit, keyColumns])

  const updateRows = useCallback((rows: TableRow[], column: string, value: unknown) => {
    if (!rows.length) return
    commit((current) => {
      let next = current
      for (const row of rows) {
        const rowId = dataRowIdentity(row, keyColumns)
        const inserted = next.find((change): change is StagedInsert => change.kind === 'insert' && change.rowId === rowId)
        if (inserted) {
          inserted.values[column] = value
          continue
        }
        const existing = next.find((change): change is StagedCellUpdate => change.kind === 'update' && change.rowId === rowId && change.column === column)
        const existingRowUpdate = next.find((change): change is StagedCellUpdate => change.kind === 'update' && change.rowId === rowId)
        const original = existing?.original ?? row[column]
        next = next.filter((change) => !(change.kind === 'update' && change.rowId === rowId && change.column === column))
        if (!sameValue(original, value)) next.push({ id: `update:${rowId}:${column}`, kind: 'update', rowId, column, keys: Object.fromEntries(keyColumns.map((key) => [key, row[key]])), expectedValues: existing?.expectedValues ?? existingRowUpdate?.expectedValues ?? stripInternalValues(row), original, value })
      }
      return next
    })
  }, [commit, keyColumns])

  const insertRow = useCallback(() => {
    const rowId = draftId()
    const values = Object.fromEntries(columns.map((column) => [column.key ?? column.name, defaultValue(column, rowId, keyColumns)]))
    commit((current) => [{ id: `insert:${rowId}`, kind: 'insert', rowId, values, source: 'new' }, ...current])
    return rowId
  }, [columns, commit, keyColumns])

  const duplicateRows = useCallback((rows: TableRow[]) => {
    if (!rows.length) return []
    const columnByName = new Map(columns.map((column) => [column.key ?? column.name, column]))
    const inserts = rows.map((row) => {
      const rowId = draftId()
      const values = { ...row }
      delete values[DRAFT_ROW_KEY]
      for (const key of keyColumns) {
        const column = columnByName.get(key)
        values[key] = column ? draftKeyValue(column, rowId) : rowId
      }
      return { id: `insert:${rowId}`, kind: 'insert' as const, rowId, values, source: 'duplicate' as const }
    })
    commit((current) => [...inserts, ...current])
    return inserts.map((insert) => insert.rowId)
  }, [columns, commit, keyColumns])

  const deleteRows = useCallback((rows: TableRow[]) => {
    if (!rows.length) return
    commit((current) => {
      let next = current
      for (const row of rows) {
        const rowId = dataRowIdentity(row, keyColumns)
        const inserted = next.some((change) => change.kind === 'insert' && change.rowId === rowId)
        if (inserted) {
          next = next.filter((change) => change.rowId !== rowId)
          continue
        }
        const existingDelete = next.some((change) => change.kind === 'delete' && change.rowId === rowId)
        if (existingDelete) continue
        const existingUpdate = next.find((change): change is StagedCellUpdate => change.kind === 'update' && change.rowId === rowId)
        next = [
          ...next.filter((change) => !(change.kind === 'update' && change.rowId === rowId)),
          { id: `delete:${rowId}`, kind: 'delete', rowId, original: { ...(existingUpdate?.expectedValues ?? stripInternalValues(row)) } },
        ]
      }
      return next
    })
  }, [commit, keyColumns])

  const discardChange = useCallback((id: string) => {
    commit((current) => {
      const target = current.find((change) => change.id === id)
      return target?.kind === 'insert'
        ? current.filter((change) => change.rowId !== target.rowId)
        : current.filter((change) => change.id !== id)
    })
  }, [commit])

  const undo = useCallback(() => {
    setLedger((current) => {
      const previous = current.undoStack.at(-1)
      if (!previous) return current
      return {
        changes: copyChanges(previous),
        undoStack: current.undoStack.slice(0, -1),
        redoStack: [...current.redoStack.slice(-49), copyChanges(current.changes)],
      }
    })
  }, [])

  const redo = useCallback(() => {
    setLedger((current) => {
      const next = current.redoStack.at(-1)
      if (!next) return current
      return {
        changes: copyChanges(next),
        undoStack: [...current.undoStack.slice(-49), copyChanges(current.changes)],
        redoStack: current.redoStack.slice(0, -1),
      }
    })
  }, [])

  const discard = useCallback(() => {
    setLedger({ changes: [], undoStack: [], redoStack: [] })
  }, [])

  const pendingCells = useMemo(() => {
    const cells = new Map<string, unknown>()
    for (const change of changes) {
      if (change.kind === 'update') cells.set(`${change.rowId}:${change.column}`, change.value)
      if (change.kind === 'insert') for (const [column, value] of Object.entries(change.values)) cells.set(`${change.rowId}:${column}`, value)
    }
    return cells
  }, [changes])

  const insertedRowIds = useMemo(() => new Set(changes.filter((change) => change.kind === 'insert').map((change) => change.rowId)), [changes])
  const deletedRowIds = useMemo(() => new Set(changes.filter((change) => change.kind === 'delete').map((change) => change.rowId)), [changes])

  const validationErrors = useMemo(() => {
    const errors = new Map<string, string>()
    const columnByName = new Map(columns.map((column) => [column.key ?? column.name, column]))
    for (const change of changes) {
      if (change.kind === 'update') {
        const column = columnByName.get(change.column)
        const error = column ? validateEditorValue(column, change.value) : undefined
        if (error) errors.set(`${change.rowId}:${change.column}`, error)
      }
      if (change.kind === 'insert') {
        for (const column of columns) {
          if (column.generated || column.identity) continue
          const key = column.key ?? column.name
          const error = validateEditorValue(column, change.values[key])
          if (error) errors.set(`${change.rowId}:${key}`, error)
        }
      }
    }
    return errors
  }, [changes, columns])

  const displayRows = useCallback((rows: TableRow[]) => {
    const updates = new Map<string, TableRow>()
    const inserts: TableRow[] = []
    for (const change of changes) {
      if (change.kind === 'update') updates.set(change.rowId, { ...(updates.get(change.rowId) ?? {}), [change.column]: change.value })
      if (change.kind === 'insert') inserts.push({ ...change.values, [DRAFT_ROW_KEY]: change.rowId })
    }
    return [
      ...inserts,
      ...rows.map((row, index) => {
        const id = dataRowIdentity(row, keyColumns, index)
        return updates.has(id) ? { ...row, ...updates.get(id) } : row
      }),
    ]
  }, [changes, keyColumns])

  const mutations = useMemo<RowMutation[]>(() => {
    const groupedUpdates = new Map<string, TableRow>()
    const groupedExpectedValues = new Map<string, TableRow>()
    const result: RowMutation[] = []
    for (const change of changes) {
      if (change.kind === 'update' && !deletedRowIds.has(change.rowId)) {
        groupedUpdates.set(change.rowId, { ...(groupedUpdates.get(change.rowId) ?? {}), [change.column]: change.value })
        groupedExpectedValues.set(change.rowId, change.expectedValues)
      }
      if (change.kind === 'insert') result.push({ kind: 'insert', values: insertMutationValues(change.values, columns) })
      if (change.kind === 'delete') result.push({ kind: 'delete', keys: Object.fromEntries(keyColumns.map((key) => [key, change.original[key]])), expectedValues: stripInternalValues(change.original) })
    }
    for (const [rowId, values] of groupedUpdates) {
      const original = changes.find((change): change is StagedCellUpdate => change.kind === 'update' && change.rowId === rowId)
      if (!original) continue
      result.push({ kind: 'update', keys: original.keys, values, expectedValues: groupedExpectedValues.get(rowId) })
    }
    return result
  }, [changes, columns, deletedRowIds, keyColumns])

  const summary = useMemo(() => ({
    inserts: changes.filter((change) => change.kind === 'insert').length,
    updates: new Set(changes.filter((change) => change.kind === 'update').map((change) => change.rowId)).size,
    deletes: changes.filter((change) => change.kind === 'delete').length,
    cells: changes.filter((change) => change.kind === 'update').length,
  }), [changes])

  return {
    changes,
    pendingCells,
    insertedRowIds,
    deletedRowIds,
    validationErrors,
    mutations,
    summary,
    canUndo: ledger.undoStack.length > 0,
    canRedo: ledger.redoStack.length > 0,
    hasErrors: validationErrors.size > 0,
    editCell,
    updateRows,
    insertRow,
    duplicateRows,
    deleteRows,
    discardChange,
    displayRows,
    undo,
    redo,
    discard,
  }
}

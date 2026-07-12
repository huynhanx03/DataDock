import { useEffect, useMemo, useState } from 'react'
import { AlertCircle, ChevronDown, ChevronLeft, ChevronRight, ChevronsUpDown, ClipboardPlus, Copy, Database, LoaderCircle, Plus, Redo2, RefreshCw, Search, Table2, Trash2, Undo2, Upload } from 'lucide-react'
import { datadockApi, type QueryColumn, type RowMutation, type TableRowsResult, type TableSchema } from './api'

type Props = { connectionId?: string; table: string; schema?: string; pageSize?: number }
type Row = Record<string, unknown>

function displayValue(value: unknown) {
  if (value === null || value === undefined) return <span className="table-browser-null">NULL</span>
  if (typeof value === 'object') return JSON.stringify(value)
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  return String(value)
}

function normalizeRows(result: TableRowsResult) {
  const raw = result as TableRowsResult & { columns?: Array<QueryColumn | string>; rows?: Array<unknown[] | Record<string, unknown>>; total?: number; totalRows?: number; limit?: number }
  const columns = (raw.columns || []).map((column) => typeof column === 'string' ? { name: column } : column)
  const rows = (raw.rows || []).map((row) => Array.isArray(row) ? Object.fromEntries(columns.map((column, index) => [column.name, row[index]])) : row)
  const resolvedPageSize = raw.pageSize || raw.limit || 50
  return { columns, rows, page: raw.page || Math.floor((raw.offset || 0) / resolvedPageSize) + 1, pageSize: resolvedPageSize, total: raw.total ?? raw.totalRows ?? rows.length }
}

function cloneMutations(mutations: RowMutation[]) { return JSON.parse(JSON.stringify(mutations)) as RowMutation[] }
function keyFor(keys: Record<string, unknown>) { return Object.entries(keys).sort(([a], [b]) => a.localeCompare(b)).map(([key, value]) => `${key}:${JSON.stringify(value)}`).join('|') }
function inputValue(value: unknown) { return value === null || value === undefined ? '' : typeof value === 'object' ? JSON.stringify(value) : String(value) }
function parseValue(value: string, prior: unknown) { if (value === '' && prior === null) return null; if (typeof prior === 'number') { const number = Number(value); return Number.isFinite(number) ? number : value }; if (typeof prior === 'boolean') return value === 'true'; if (typeof prior === 'object' && value) { try { return JSON.parse(value) } catch { return value } }; return value }

export function TableBrowser({ connectionId, table, schema, pageSize = 50 }: Props) {
  const [result, setResult] = useState<TableRowsResult>()
  const [tableSchema, setTableSchema] = useState<TableSchema>()
  const [search, setSearch] = useState('')
  const [appliedSearch, setAppliedSearch] = useState('')
  const [page, setPage] = useState(1)
  const [sortBy, setSortBy] = useState('')
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('asc')
  const [reloadKey, setReloadKey] = useState(0)
  const [loading, setLoading] = useState(false)
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState('')
  const [mutations, setMutations] = useState<RowMutation[]>([])
  const [undo, setUndo] = useState<RowMutation[][]>([])
  const [redo, setRedo] = useState<RowMutation[][]>([])
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const reference = schema ? `${schema}.${table}` : table

  useEffect(() => { const timer = window.setTimeout(() => { setAppliedSearch(search); setPage(1) }, 280); return () => window.clearTimeout(timer) }, [search])
  useEffect(() => { setMutations([]); setUndo([]); setRedo([]); setSelected(new Set()); setTableSchema(undefined) }, [connectionId, reference])
  useEffect(() => {
    if (!connectionId || !table) { setResult(undefined); setError(''); return }
    let current = true; setLoading(true); setError('')
    void Promise.all([datadockApi.getTableRows(connectionId, reference, { page, pageSize, search: appliedSearch, sortBy, sortDirection }), datadockApi.getTableSchema(connectionId, reference)])
      .then(([data, definition]) => { if (current) { setResult(data); setTableSchema(definition) } })
      .catch((caught) => { if (current) setError(caught instanceof Error ? caught.message : 'Unable to load table data') })
      .finally(() => { if (current) setLoading(false) })
    return () => { current = false }
  }, [connectionId, reference, page, pageSize, appliedSearch, sortBy, sortDirection, reloadKey])

  const data = useMemo(() => result ? normalizeRows(result) : undefined, [result])
  const columns = data?.columns || []
  const primaryKeys = useMemo(() => tableSchema?.constraints.find((item) => item.type === 'PRIMARY KEY')?.columns || tableSchema?.indexes.find((item) => item.primary)?.definition.split(',').map((item) => item.trim()).filter(Boolean) || [], [tableSchema])
  const existingRows = useMemo(() => {
    const changed = new Map<string, RowMutation>(); const deleted = new Set<string>(); const added: Row[] = []
    mutations.forEach((mutation) => { if (mutation.kind === 'insert') added.push(mutation.values || {}); else if (mutation.kind === 'delete') deleted.add(keyFor(mutation.keys || {})); else changed.set(keyFor(mutation.keys || {}), mutation) })
    return [...(data?.rows || []).filter((row) => !deleted.has(keyFor(Object.fromEntries(primaryKeys.map((key) => [key, row[key]]))))).map((row) => ({ ...row, ...(changed.get(keyFor(Object.fromEntries(primaryKeys.map((key) => [key, row[key]]))))?.values || {}) })), ...added]
  }, [data, mutations, primaryKeys])
  const pageCount = Math.max(1, Math.ceil((data?.total || 0) / pageSize)); const start = data?.total ? (page - 1) * pageSize + 1 : 0; const end = data ? Math.min(page * pageSize, data.total) : 0
  const editable = primaryKeys.length > 0
  function record(next: RowMutation[]) { setUndo((items) => [...items.slice(-39), cloneMutations(mutations)]); setMutations(next); setRedo([]) }
  function rowKeys(row: Row) { return Object.fromEntries(primaryKeys.map((key) => [key, row[key]])) }
  function rowId(row: Row) { return row.__datadockNew as string || keyFor(rowKeys(row)) }
  function setCell(row: Row, column: string, raw: string) { const value = parseValue(raw, row[column]); const id = rowId(row); const isNew = Boolean(row.__datadockNew); const next = cloneMutations(mutations); const index = next.findIndex((mutation) => isNew ? mutation.kind === 'insert' && mutation.values?.__datadockNew === id : mutation.kind === 'update' && keyFor(mutation.keys || {}) === id); if (isNew) { if (index >= 0) next[index].values = { ...next[index].values, [column]: value }; record(next); return }; const keys = rowKeys(row); const original = data?.rows.find((item) => keyFor(rowKeys(item)) === id); const values = { ...(index >= 0 ? next[index].values : {}), [column]: value }; if (original && Object.entries(values).every(([name, changed]) => JSON.stringify(changed) === JSON.stringify(original[name]))) { if (index >= 0) next.splice(index, 1) } else if (index >= 0) next[index].values = values; else next.push({ kind: 'update', keys, values }); record(next) }
  function addRow(copy?: Row) { const values = Object.fromEntries(columns.map((column) => [column.name, copy ? copy[column.name] : null])); primaryKeys.forEach((key) => { if (copy) delete values[key] }); values.__datadockNew = crypto.randomUUID(); record([...mutations, { kind: 'insert', values }]) }
  function removeRows() { const chosen = existingRows.filter((row) => selected.has(rowId(row))); if (!chosen.length) return; const next = mutations.filter((mutation) => !(mutation.kind === 'insert' && selected.has(String(mutation.values?.__datadockNew)))); chosen.filter((row) => !row.__datadockNew).forEach((row) => { const keys = rowKeys(row); next.push({ kind: 'delete', keys }) }); record(next); setSelected(new Set()) }
  function batchUpdate() { const column = window.prompt('Column name to update'); if (!column || !columns.some((item) => item.name === column)) return; const value = window.prompt(`New value for ${column}`); if (value === null) return; let next = cloneMutations(mutations); existingRows.filter((row) => selected.has(rowId(row))).forEach((row) => { const parsed = parseValue(value, row[column]); const id = rowId(row); if (row.__datadockNew) { const index = next.findIndex((mutation) => mutation.kind === 'insert' && mutation.values?.__datadockNew === id); if (index >= 0) next[index].values = { ...next[index].values, [column]: parsed }; return }; const index = next.findIndex((mutation) => mutation.kind === 'update' && keyFor(mutation.keys || {}) === id); const original = data?.rows.find((item) => keyFor(rowKeys(item)) === id); const values = { ...(index >= 0 ? next[index].values : {}), [column]: parsed }; if (original && Object.entries(values).every(([name, changed]) => JSON.stringify(changed) === JSON.stringify(original[name]))) { if (index >= 0) next.splice(index, 1) } else if (index >= 0) next[index].values = values; else next.push({ kind: 'update', keys: rowKeys(row), values }) }); record(next) }
  async function applyChanges() { if (!connectionId || !mutations.length) return; const sanitized = mutations.map((mutation) => ({ ...mutation, values: mutation.values ? Object.fromEntries(Object.entries(mutation.values).filter(([key]) => key !== '__datadockNew')) : undefined })); setApplying(true); setError(''); try { await datadockApi.mutateTableRows(connectionId, reference, sanitized); setMutations([]); setUndo([]); setRedo([]); setSelected(new Set()); setReloadKey((value) => value + 1) } catch (caught) { setError(caught instanceof Error ? caught.message : 'Unable to apply changes') } finally { setApplying(false) } }
  function changeSort(column: string) { if (column === sortBy) setSortDirection((direction) => direction === 'asc' ? 'desc' : 'asc'); else { setSortBy(column); setSortDirection('asc') }; setPage(1) }
  if (!connectionId) return <section className="table-browser-state"><Database size={21} /><strong>Select a connection</strong><span>Choose a saved connection to browse table data.</span></section>
  return <section className="table-browser" aria-label={`Browse ${table}`}>
    <div className="table-browser-toolbar"><div className="table-browser-search"><Search size={15} /><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search all columns…" aria-label="Search table rows" />{search && <button onClick={() => setSearch('')} aria-label="Clear search">×</button>}</div><span className="table-browser-caption"><Table2 size={14} />{reference}</span><button className="tool-button" onClick={() => addRow()} disabled={!editable} title={editable ? 'Insert row' : 'A primary key is required to edit'}><Plus size={14} />Row</button><button className="tool-button" onClick={batchUpdate} disabled={!selected.size || !editable}><ClipboardPlus size={14} />Batch</button><button className="tool-button danger" onClick={removeRows} disabled={!selected.size || !editable}><Trash2 size={14} />Delete</button><button className="tool-button" onClick={() => { setRedo((items) => [...items, cloneMutations(mutations)]); setMutations(undo[undo.length - 1] || []); setUndo((items) => items.slice(0, -1)) }} disabled={!undo.length}><Undo2 size={14} /></button><button className="tool-button" onClick={() => { setUndo((items) => [...items, cloneMutations(mutations)]); setMutations(redo[redo.length - 1] || []); setRedo((items) => items.slice(0, -1)) }} disabled={!redo.length}><Redo2 size={14} /></button><button className="tool-button" onClick={() => { setMutations([]); setUndo([]); setRedo([]); setSelected(new Set()) }} disabled={!mutations.length}>Discard</button><button className="tool-button" onClick={() => setReloadKey((value) => value + 1)} disabled={loading} title="Refresh rows"><RefreshCw size={14} className={loading ? 'spin' : ''} /></button><button className="button primary" onClick={() => void applyChanges()} disabled={!mutations.length || applying || !editable}><Upload size={14} />{applying ? 'Applying…' : `Apply ${mutations.length || ''}`}</button></div>
    {!editable && tableSchema && <div className="table-browser-warning">This table has no primary key. Data editing is disabled to prevent unsafe changes.</div>}
    <div className="table-browser-card">{error && <div className="table-browser-error"><AlertCircle size={17} /><div><strong>Couldn’t complete data operation</strong><span>{error}</span></div><button className="button secondary" onClick={() => setReloadKey((value) => value + 1)}>Try again</button></div>}{!error && <div className="table-browser-grid-wrap"><table className="table-browser-grid"><thead><tr><th className="table-browser-row-number"><input type="checkbox" aria-label="Select all visible rows" checked={existingRows.length > 0 && selected.size === existingRows.length} onChange={(event) => setSelected(event.target.checked ? new Set(existingRows.map(rowId)) : new Set())} /></th><th className="table-browser-row-number">#</th>{columns.map((column) => <th key={column.name}><button onClick={() => changeSort(column.name)} title={`Sort by ${column.name}`}><span>{column.type && <small>{column.type}</small>}{column.name}{primaryKeys.includes(column.name) && <em>PK</em>}</span>{sortBy === column.name ? <ChevronDown size={14} className={sortDirection === 'asc' ? 'sort-ascending' : ''} /> : <ChevronsUpDown size={13} />}</button></th>)}<th /></tr></thead><tbody>{loading && !data ? <tr><td className="table-browser-loading" colSpan={columns.length + 3}><LoaderCircle size={18} className="spin" />Loading rows…</td></tr> : !existingRows.length ? <tr><td className="table-browser-empty" colSpan={columns.length + 3}>{appliedSearch ? 'No rows match your search.' : 'This table has no rows.'}</td></tr> : existingRows.map((row, rowIndex) => <tr key={rowId(row)} className={row.__datadockNew ? 'table-browser-new' : ''}><td className="table-browser-row-number"><input type="checkbox" checked={selected.has(rowId(row))} onChange={(event) => setSelected((items) => { const next = new Set(items); event.target.checked ? next.add(rowId(row)) : next.delete(rowId(row)); return next })} /></td><td className="table-browser-row-number">{row.__datadockNew ? '+' : (page - 1) * pageSize + rowIndex + 1}</td>{columns.map((column) => <td key={column.name} title={typeof row[column.name] === 'object' && row[column.name] !== null ? JSON.stringify(row[column.name]) : undefined}><input className="table-browser-cell" disabled={!editable} value={inputValue(row[column.name])} onChange={(event) => setCell(row, column.name, event.target.value)} /></td>)}<td><button className="mini-icon" onClick={() => addRow(row)} disabled={!editable} title="Duplicate row"><Copy size={13} /></button></td></tr>)}</tbody></table>{loading && data && <div className="table-browser-overlay"><LoaderCircle size={18} className="spin" /></div>}</div>}<footer className="table-browser-footer"><span>{mutations.length ? `${mutations.length} staged change${mutations.length === 1 ? '' : 's'} · ` : ''}{data ? `Showing ${start}–${end} of ${data.total.toLocaleString()} rows` : 'Rows will appear here'}</span><div><button onClick={() => setPage(1)} disabled={page <= 1 || loading} aria-label="First page"><ChevronLeft size={14} /><ChevronLeft size={14} /></button><button onClick={() => setPage((value) => value - 1)} disabled={page <= 1 || loading} aria-label="Previous page"><ChevronLeft size={15} /></button><span>Page {page} of {pageCount}</span><button onClick={() => setPage((value) => Math.min(pageCount, value + 1))} disabled={page >= pageCount || loading} aria-label="Next page"><ChevronRight size={15} /></button><button onClick={() => setPage(pageCount)} disabled={page >= pageCount || loading} aria-label="Last page"><ChevronRight size={14} /><ChevronRight size={14} /></button></div></footer></div>
  </section>
}

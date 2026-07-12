import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { AlertCircle, Check, ChevronDown, Code2, Columns3, FilePlus2, Fingerprint, KeyRound, LoaderCircle, Plus, RefreshCw, TableProperties, Trash2, X } from 'lucide-react'
import { datadockApi, type ColumnDefinition, type TableSchema } from './api'

type Props = { connectionId?: string; table: string; schema?: string; view: 'structure' | 'indexes' | 'relations'; onTableCreated?: (name: string) => void }
type Dialog = 'index' | 'column' | 'table' | 'ddl' | null

const emptyColumn = (): ColumnDefinition => ({ name: '', dataType: 'text', nullable: true, defaultValue: null, comment: '' })

function tableReference(table: string, schema?: string) { return schema && !table.includes('.') ? `${schema}.${table}` : table }

export function SchemaManager({ connectionId, table, schema, view, onTableCreated }: Props) {
  const [data, setData] = useState<TableSchema>()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [dialog, setDialog] = useState<Dialog>(null)
  const [saving, setSaving] = useState(false)
  const [notice, setNotice] = useState('')
  const [indexName, setIndexName] = useState('')
  const [indexColumns, setIndexColumns] = useState<string[]>([])
  const [unique, setUnique] = useState(false)
  const [column, setColumn] = useState<ColumnDefinition>(emptyColumn())
  const [newTable, setNewTable] = useState('')
  const [newColumns, setNewColumns] = useState<ColumnDefinition[]>([{ name: 'id', dataType: 'bigint', nullable: false, defaultValue: null, comment: '' }])
  const [ddl, setDdl] = useState('')
  const ref = tableReference(table, schema)

  async function refresh() {
    if (!connectionId) return
    setLoading(true); setError('')
    try { setData(await datadockApi.getTableSchema(connectionId, ref)) }
    catch (caught) { setError(caught instanceof Error ? caught.message : 'Unable to load structure') }
    finally { setLoading(false) }
  }
  useEffect(() => { void refresh() }, [connectionId, ref])

  function closeDialog() { if (!saving) setDialog(null) }
  async function createIndex(event: FormEvent) {
    event.preventDefault()
    if (!connectionId || !indexName || !indexColumns.length) return
    setSaving(true); setError('')
    try { await datadockApi.createIndex(connectionId, ref, { name: indexName, columns: indexColumns, unique }); setNotice(`Index ${indexName} created`); setDialog(null); setIndexName(''); setIndexColumns([]); await refresh() }
    catch (caught) { setError(caught instanceof Error ? caught.message : 'Unable to create index') }
    finally { setSaving(false) }
  }
  async function addColumn(event: FormEvent) {
    event.preventDefault()
    if (!connectionId || !column.name) return
    setSaving(true); setError('')
    try { await datadockApi.alterTable(connectionId, ref, [{ kind: 'add_column', definition: column }]); setNotice(`Column ${column.name} added`); setDialog(null); setColumn(emptyColumn()); await refresh() }
    catch (caught) { setError(caught instanceof Error ? caught.message : 'Unable to add column') }
    finally { setSaving(false) }
  }
  async function createTable(event: FormEvent) {
    event.preventDefault()
    if (!connectionId || !newTable || newColumns.some((item) => !item.name || !item.dataType)) return
    setSaving(true); setError('')
    try { await datadockApi.createTable(connectionId, { schema, name: newTable, columns: newColumns }); setNotice(`Table ${newTable} created`); onTableCreated?.(newTable); setDialog(null); setNewTable(''); setNewColumns([{ name: 'id', dataType: 'bigint', nullable: false, defaultValue: null, comment: '' }]) }
    catch (caught) { setError(caught instanceof Error ? caught.message : 'Unable to create table') }
    finally { setSaving(false) }
  }
  async function showDDL() {
    if (!connectionId) return
    setSaving(true); setError('')
    try { setDdl((await datadockApi.getTableDDL(connectionId, ref)).ddl); setDialog('ddl') }
    catch (caught) { setError(caught instanceof Error ? caught.message : 'Unable to load DDL') }
    finally { setSaving(false) }
  }
  async function dropIndex(name: string) {
    if (!connectionId || !window.confirm(`Drop index ${name}?`)) return
    setSaving(true); setError('')
    try { await datadockApi.dropIndex(connectionId, ref, name); setNotice(`Index ${name} dropped`); await refresh() }
    catch (caught) { setError(caught instanceof Error ? caught.message : 'Unable to drop index') }
    finally { setSaving(false) }
  }

  if (!connectionId) return <div className="schema-empty"><DatabaseState /><strong>Select a connection</strong><span>Choose a database connection to inspect its schema.</span></div>
  return <section className="schema-manager">
    <div className="schema-toolbar">
      <div><span className="eyebrow">DATABASE STRUCTURE</span><strong>{schema ? `${schema}.` : ''}{table}</strong></div>
      <div className="schema-toolbar-actions"><button className="tool-button" onClick={() => void showDDL()} disabled={saving}><Code2 size={14} />DDL</button><button className="tool-button" onClick={() => setDialog('table')}><FilePlus2 size={14} />New table</button><button className="tool-button" onClick={() => void refresh()} disabled={loading}><RefreshCw size={14} className={loading ? 'spin' : ''} />Refresh</button></div>
    </div>
    {notice && <div className="schema-notice"><Check size={14} />{notice}<button onClick={() => setNotice('')}><X size={13} /></button></div>}
    {error && <div className="schema-error"><AlertCircle size={16} /><span>{error}</span><button onClick={() => { setError(''); void refresh() }}>Retry</button></div>}
    {loading && !data ? <div className="schema-empty"><LoaderCircle className="spin" size={20} /><span>Inspecting table structure…</span></div> : view === 'structure' ? <Structure data={data} onAdd={() => setDialog('column')} /> : view === 'indexes' ? <Indexes data={data} onAdd={() => setDialog('index')} onDrop={(name) => void dropIndex(name)} /> : <Relations data={data} />}
    {dialog === 'index' && <Dialog title="Create index" onClose={closeDialog}><form onSubmit={(event) => void createIndex(event)} className="schema-form"><label>Index name<input autoFocus value={indexName} onChange={(event) => setIndexName(event.target.value)} placeholder={`idx_${table}_…`} required /></label><fieldset><legend>Columns</legend><div className="check-list">{data?.columns.map((item) => <label key={item.name} className="check-option"><input type="checkbox" checked={indexColumns.includes(item.name)} onChange={() => setIndexColumns((current) => current.includes(item.name) ? current.filter((name) => name !== item.name) : [...current, item.name])} />{item.name}<small>{item.dataType}</small></label>)}</div></fieldset><label className="check-option"><input type="checkbox" checked={unique} onChange={(event) => setUnique(event.target.checked)} />Unique index</label><DialogActions saving={saving} label="Create index" onClose={closeDialog} /></form></Dialog>}
    {dialog === 'column' && <Dialog title="Add column" onClose={closeDialog}><form onSubmit={(event) => void addColumn(event)} className="schema-form"><ColumnFields value={column} onChange={setColumn} /><DialogActions saving={saving} label="Add column" onClose={closeDialog} /></form></Dialog>}
    {dialog === 'table' && <Dialog title="Create table" onClose={closeDialog}><form onSubmit={(event) => void createTable(event)} className="schema-form"><label>Table name<input autoFocus value={newTable} onChange={(event) => setNewTable(event.target.value)} placeholder="events" required /></label><fieldset><legend>Columns</legend>{newColumns.map((item, index) => <div className="new-column" key={index}><input value={item.name} placeholder="name" onChange={(event) => setNewColumns((items) => items.map((value, position) => position === index ? { ...value, name: event.target.value } : value))} required /><input value={item.dataType} placeholder="type" onChange={(event) => setNewColumns((items) => items.map((value, position) => position === index ? { ...value, dataType: event.target.value } : value))} required /><label title="Nullable"><input type="checkbox" checked={item.nullable} onChange={(event) => setNewColumns((items) => items.map((value, position) => position === index ? { ...value, nullable: event.target.checked } : value))} />Null</label>{newColumns.length > 1 && <button type="button" className="mini-icon" onClick={() => setNewColumns((items) => items.filter((_, position) => position !== index))}><Trash2 size={14} /></button>}</div>)}<button type="button" className="add-inline" onClick={() => setNewColumns((items) => [...items, emptyColumn()])}><Plus size={13} />Add column</button></fieldset><DialogActions saving={saving} label="Create table" onClose={closeDialog} /></form></Dialog>}
    {dialog === 'ddl' && <Dialog title="Table DDL" onClose={closeDialog}><pre className="ddl-view">{ddl}</pre><div className="dialog-actions"><button type="button" className="button secondary" onClick={() => navigator.clipboard.writeText(ddl)}>Copy DDL</button><button type="button" className="button primary" onClick={closeDialog}>Done</button></div></Dialog>}
  </section>
}

function Structure({ data, onAdd }: { data?: TableSchema; onAdd: () => void }) { return <div className="schema-card"><header><span><Columns3 size={15} />Columns <small>{data?.columns.length || 0}</small></span><button className="tool-button" onClick={onAdd}><Plus size={14} />Add column</button></header><table className="schema-table"><thead><tr><th>Name</th><th>Type</th><th>Nullable</th><th>Default</th><th>Comment</th></tr></thead><tbody>{data?.columns.map((column) => <tr key={column.name}><td><strong>{column.name}</strong></td><td><code>{column.dataType}</code></td><td>{column.nullable ? <span className="schema-pill muted">NULL</span> : <span className="schema-pill accent">NOT NULL</span>}</td><td><code>{column.defaultValue || '—'}</code></td><td>{column.comment || '—'}</td></tr>)}</tbody></table>{!data?.columns.length && <Empty label="No columns found" />}</div> }
function Indexes({ data, onAdd, onDrop }: { data?: TableSchema; onAdd: () => void; onDrop: (name: string) => void }) { return <div className="schema-card"><header><span><Fingerprint size={15} />Indexes <small>{data?.indexes.length || 0}</small></span><button className="tool-button" onClick={onAdd}><Plus size={14} />Create index</button></header><table className="schema-table"><thead><tr><th>Name</th><th>Type</th><th>Definition</th><th /></tr></thead><tbody>{data?.indexes.map((index) => <tr key={index.name}><td><strong>{index.name}</strong>{index.primary && <span className="inline-badge">PRIMARY</span>}{index.unique && !index.primary && <span className="inline-badge">UNIQUE</span>}</td><td><code>{index.type}</code></td><td className="schema-definition"><code>{index.definition}</code></td><td>{!index.primary && <button className="mini-icon danger" title={`Drop ${index.name}`} onClick={() => onDrop(index.name)}><Trash2 size={14} /></button>}</td></tr>)}</tbody></table>{!data?.indexes.length && <Empty label="No indexes found" />}</div> }
function Relations({ data }: { data?: TableSchema }) { return <div className="schema-card"><header><span><KeyRound size={15} />Constraints & relations <small>{data?.constraints.length || 0}</small></span></header><table className="schema-table"><thead><tr><th>Name</th><th>Type</th><th>Columns</th><th>Definition</th></tr></thead><tbody>{data?.constraints.map((item) => <tr key={item.name}><td><strong>{item.name}</strong></td><td><span className="schema-pill muted">{item.type}</span></td><td>{item.columns.join(', ') || '—'}</td><td className="schema-definition"><code>{item.definition || '—'}</code></td></tr>)}</tbody></table>{!data?.constraints.length && <Empty label="No constraints found" />}</div> }
function ColumnFields({ value, onChange }: { value: ColumnDefinition; onChange: (value: ColumnDefinition) => void }) { return <><label>Column name<input autoFocus value={value.name} onChange={(event) => onChange({ ...value, name: event.target.value })} placeholder="created_at" required /></label><label>Data type<input value={value.dataType} onChange={(event) => onChange({ ...value, dataType: event.target.value })} placeholder="timestamp" required /></label><label>Default expression<input value={value.defaultValue || ''} onChange={(event) => onChange({ ...value, defaultValue: event.target.value || null })} placeholder="CURRENT_TIMESTAMP" /></label><label>Comment<input value={value.comment} onChange={(event) => onChange({ ...value, comment: event.target.value })} placeholder="Optional description" /></label><label className="check-option"><input type="checkbox" checked={value.nullable} onChange={(event) => onChange({ ...value, nullable: event.target.checked })} />Allow NULL values</label></> }
function Dialog({ title, children, onClose }: { title: string; children: ReactNode; onClose: () => void }) { return <div className="modal-backdrop" onMouseDown={onClose}><section className="schema-dialog" onMouseDown={(event) => event.stopPropagation()} role="dialog" aria-modal="true" aria-label={title}><header><div><span className="dialog-kicker">SCHEMA MANAGER</span><h2>{title}</h2></div><button type="button" className="icon-button" onClick={onClose}><X size={18} /></button></header>{children}</section></div> }
function DialogActions({ saving, label, onClose }: { saving: boolean; label: string; onClose: () => void }) { return <div className="dialog-actions"><button type="button" className="button secondary" onClick={onClose}>Cancel</button><button className="button primary" disabled={saving}>{saving && <LoaderCircle size={14} className="spin" />}{label}</button></div> }
function Empty({ label }: { label: string }) { return <div className="schema-empty small"><TableProperties size={19} /><span>{label}</span></div> }
function DatabaseState() { return <TableProperties size={21} /> }

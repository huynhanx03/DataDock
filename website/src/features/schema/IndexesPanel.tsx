import { useMemo, useState } from 'react'
import { Activity, BarChart3, Check, DatabaseZap, Pencil, Plus, RefreshCw, Search, ShieldCheck, Trash2, WandSparkles } from 'lucide-react'
import type { DatabaseEngine } from '@/entities/connection'
import type { TableIndex } from '@/entities/database-object'
import type { SchemaDraftChange } from '@/entities/schema'
import type { SchemaColumn } from '@/features/schema/ColumnsPanel'
import {
  Badge,
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  Input,
  PropertyField,
  Select,
  WorkspacePanel,
} from '@/shared/ui'

type IndexStats = {
  size: string
  scans: number
  tuplesRead: number
  hitRate: number
  lastUsed: string
}

type IndexDraft = {
  name: string
  method: string
  columns: string[]
  unique: boolean
  predicate: string
}

type IndexesPanelProps = {
  indexes: TableIndex[]
  columns: SchemaColumn[]
  engine: DatabaseEngine
  table: string
  source: 'mock' | 'api'
  readOnly?: boolean
  onIndexesChange: (indexes: TableIndex[]) => void
  onStage: (change: SchemaDraftChange) => void
}

function statsFor(index: TableIndex, position: number): IndexStats {
  if (index.primary) return { size: '18.4 MB', scans: 894221, tuplesRead: 2210842, hitRate: 99.8, lastUsed: '12 seconds ago' }
  if (index.unique) return { size: '12.8 MB', scans: 321884, tuplesRead: 382104, hitRate: 99.2, lastUsed: '42 seconds ago' }
  return { size: `${(4.6 + position * 2.7).toFixed(1)} MB`, scans: 18420 - position * 2180, tuplesRead: 68210 + position * 4211, hitRate: 96.4 - position * 1.2, lastUsed: `${2 + position * 3} minutes ago` }
}

function isMySql(engine: DatabaseEngine) {
  return engine === 'mysql' || engine === 'mariadb'
}

function emptyDraft(tableName: string): IndexDraft {
  return { name: `idx_${tableName}_`, method: 'btree', columns: [], unique: false, predicate: '' }
}

function definitionFor(draft: IndexDraft, engine: DatabaseEngine) {
  if (isMySql(engine)) {
    const special = draft.method === 'fulltext' ? 'FULLTEXT ' : draft.method === 'spatial' ? 'SPATIAL ' : draft.unique ? 'UNIQUE ' : ''
    const using = draft.method === 'btree' || draft.method === 'hash' ? ` USING ${draft.method.toUpperCase()}` : ''
    return `CREATE ${special}INDEX ${draft.name || 'index_name'} ON {{table}} (${draft.columns.join(', ') || 'column_name'})${using};`
  }
  return `CREATE ${draft.unique ? 'UNIQUE ' : ''}INDEX ${draft.name || 'index_name'} ON {{table}} USING ${draft.method} (${draft.columns.join(', ') || 'column_name'})${draft.predicate.trim() ? ` WHERE ${draft.predicate.trim()}` : ''};`
}

function dropIndexSql(index: TableIndex, engine: DatabaseEngine) {
  return isMySql(engine) ? `ALTER TABLE {{table}} DROP INDEX ${index.name};` : `DROP INDEX ${index.name};`
}

function AddIndexDialog({ open, tableName, columns, engine, initial, onOpenChange, onSubmit }: { open: boolean; tableName: string; columns: SchemaColumn[]; engine: DatabaseEngine; initial?: IndexDraft; onOpenChange: (open: boolean) => void; onSubmit: (draft: IndexDraft) => void }) {
  const [draft, setDraft] = useState(() => initial ?? emptyDraft(tableName))
  const specialMethod = draft.method === 'fulltext' || draft.method === 'spatial'
  const valid = draft.name.trim() && draft.columns.length > 0 && !(specialMethod && draft.unique)
  function toggleColumn(name: string) {
    setDraft((value) => ({ ...value, columns: value.columns.includes(name) ? value.columns.filter((column) => column !== name) : [...value.columns, name] }))
  }
  const methods = isMySql(engine) ? ['btree', 'hash', 'fulltext', 'spatial'] : ['btree', 'hash', 'gin', 'gist', 'brin']
  return <Dialog open={open} onOpenChange={onOpenChange}><DialogContent className="max-w-2xl"><DialogHeader><DialogTitle>{initial ? 'Edit index' : 'Create index'}</DialogTitle><DialogDescription>Select the index method and ordered key columns. DataDock will request a dialect-safe preview before applying it.</DialogDescription></DialogHeader><div className="grid gap-4 sm:grid-cols-2"><PropertyField label="Index name" required><Input autoFocus className="font-mono" value={draft.name} onChange={(event) => setDraft((value) => ({ ...value, name: event.target.value }))} /></PropertyField><PropertyField label="Access method"><Select value={draft.method} onChange={(event) => setDraft((value) => ({ ...value, method: event.target.value, unique: ['fulltext', 'spatial'].includes(event.target.value) ? false : value.unique }))}>{methods.map((method) => <option key={method}>{method}</option>)}</Select></PropertyField></div><PropertyField label="Key columns" required description="Click columns in the desired index order."><div className="flex flex-wrap gap-2 rounded-lg border border-border bg-background/60 p-2.5">{columns.map((column) => <button key={column.id} type="button" aria-pressed={draft.columns.includes(column.name)} onClick={() => toggleColumn(column.name)} className="inline-flex items-center gap-1.5 rounded-md border border-border bg-surface px-2.5 py-1.5 font-mono text-[length:var(--font-size-data)] text-muted-foreground transition-colors hover:bg-accent aria-pressed:border-primary aria-pressed:bg-accent aria-pressed:text-accent-foreground">{draft.columns.includes(column.name) ? <span className="grid size-4 place-items-center rounded bg-primary text-[10px] font-bold text-primary-foreground">{draft.columns.indexOf(column.name) + 1}</span> : null}{column.name}</button>)}</div></PropertyField><div className="grid gap-3 rounded-lg border border-border bg-muted/35 p-3 sm:grid-cols-2"><Checkbox checked={draft.unique} disabled={specialMethod} label="Unique index" onChange={(event) => setDraft((value) => ({ ...value, unique: event.target.checked }))} /><div className="flex items-center text-[length:var(--font-size-meta)] text-muted-foreground">{specialMethod ? `${draft.method} indexes cannot be unique on MySQL or MariaDB.` : 'The server preview selects supported build syntax.'}</div></div>{isMySql(engine) ? null : <PropertyField label="Partial index predicate" description="Optional SQL expression after WHERE."><Input className="font-mono" value={draft.predicate} placeholder="status = 'active'" onChange={(event) => setDraft((value) => ({ ...value, predicate: event.target.value }))} /></PropertyField>}<pre className="overflow-auto rounded-lg border border-border bg-background/70 p-3 whitespace-pre-wrap font-mono text-[length:var(--font-size-data)] leading-5 text-foreground">{definitionFor(draft, engine)}</pre><DialogFooter><Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button><Button disabled={!valid} onClick={() => onSubmit({ ...draft, name: draft.name.trim() })}>{initial ? <Pencil /> : <Plus />}{initial ? 'Stage update' : 'Stage index'}</Button></DialogFooter></DialogContent></Dialog>
}

export function IndexesPanel({ indexes, columns, engine, table, source, readOnly, onIndexesChange, onStage }: IndexesPanelProps) {
  const [search, setSearch] = useState('')
  const [selectedName, setSelectedName] = useState(indexes[0]?.name ?? '')
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<TableIndex>()
  const [action, setAction] = useState<{ kind: 'rebuild' | 'analyze' | 'drop'; index: TableIndex }>()
  const visible = useMemo(() => {
    const query = search.trim().toLowerCase()
    return indexes.filter((index) => !query || `${index.name} ${index.type} ${index.definition}`.toLowerCase().includes(query))
  }, [indexes, search])
  const selected = visible.find((index) => index.name === selectedName) ?? visible[0]
  const stats = selected && source === 'mock' ? statsFor(selected, indexes.indexOf(selected)) : undefined
  const tableName = table.split('.').at(-1) ?? table

  function submitIndex(draft: IndexDraft) {
    const definition = definitionFor(draft, engine)
    const next: TableIndex = { name: draft.name, unique: draft.unique, primary: false, type: draft.method, definition: definition.replace('{{table}}', table) }
    onIndexesChange(editing ? indexes.map((index) => index.name === editing.name ? next : index) : [...indexes, next])
    setSelectedName(next.name)
    onStage({
      label: editing ? `Alter ${editing.name}` : `Create ${next.name}`,
      description: `${next.type} · ${draft.columns.join(', ')}`,
      actions: [
        ...(editing ? [{ kind: 'drop_index' as const, name: editing.name, cascade: false }] : []),
        { kind: 'create_index', index: { name: draft.name, columns: [...draft.columns], unique: draft.unique, method: draft.method, predicate: draft.predicate.trim() } },
      ],
    })
    setAdding(false)
    setEditing(undefined)
  }

  function confirmAction() {
    if (!action) return
    if (action.kind === 'drop') {
      const next = indexes.filter((index) => index.name !== action.index.name)
      onIndexesChange(next)
      setSelectedName(next[0]?.name ?? '')
      onStage({ label: `Drop ${action.index.name}`, description: 'Destructive index change', actions: [{ kind: 'drop_index', name: action.index.name, cascade: false }] })
    } else if (action.kind === 'rebuild') {
      onStage({ label: `Rebuild ${action.index.name}`, description: 'Recreate index pages', actions: [{ kind: 'rebuild_index', name: action.index.name }] })
    } else {
      onStage({ label: `Analyze ${table}`, description: `Refresh statistics for ${action.index.name}`, actions: [{ kind: 'analyze_index', name: action.index.name }] })
    }
    setAction(undefined)
  }

  return <div className="grid min-h-0 flex-1 grid-cols-[minmax(0,1.58fr)_minmax(18rem,0.72fr)] gap-4 max-xl:grid-cols-1">
    <WorkspacePanel noPadding title="Indexes" description={`${indexes.length} access paths${source === 'mock' ? ' · sample usage statistics' : ' · catalog definitions'}`} actions={<Button size="xs" disabled={readOnly} onClick={() => setAdding(true)}><Plus />New index</Button>}>
      <div className="flex min-h-[var(--toolbar-height)] items-center gap-2 border-b border-border px-3 py-1.5"><div className="relative max-w-sm flex-1"><Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" /><Input aria-label="Find an index" className="h-[var(--control-height-sm)] pl-8" value={search} placeholder="Find an index" onChange={(event) => setSearch(event.target.value)} /></div><Badge variant={source === 'mock' ? 'accent' : 'outline'}>{source === 'mock' ? 'Sample statistics' : 'Live catalog'}</Badge></div>
      {visible.length ? <div className="overflow-auto"><table className="w-full min-w-[780px] border-collapse text-[length:var(--font-size-data)]"><thead><tr className="bg-grid-header text-left text-muted-foreground"><th className="border-b border-grid-line px-3 py-2 font-medium">Index</th><th className="border-b border-grid-line px-3 py-2 font-medium">Method</th><th className="border-b border-grid-line px-3 py-2 font-medium">Size</th><th className="border-b border-grid-line px-3 py-2 font-medium">Scans</th><th className="border-b border-grid-line px-3 py-2 font-medium">Hit rate</th><th className="border-b border-grid-line px-3 py-2 font-medium">Last used</th></tr></thead><tbody>{visible.map((index) => { const itemStats = source === 'mock' ? statsFor(index, indexes.indexOf(index)) : undefined; return <tr key={index.name} tabIndex={0} aria-selected={selected?.name === index.name} onClick={() => setSelectedName(index.name)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelectedName(index.name) } }} className="border-b border-grid-line outline-none hover:bg-accent/35 focus-visible:bg-accent/45 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary aria-selected:bg-accent/65"><td className="px-3 py-2.5"><div className="flex items-center gap-2"><DatabaseZap className="size-4 text-primary" /><span className="font-mono font-medium text-foreground">{index.name}</span>{index.primary ? <Badge variant="warning">Primary</Badge> : index.unique ? <Badge variant="accent">Unique</Badge> : null}</div></td><td className="px-3 py-2.5"><Badge variant="outline" className="rounded-md font-mono font-normal">{index.type}</Badge></td><td className="px-3 py-2.5 font-mono text-muted-foreground">{itemStats?.size ?? 'Not collected'}</td><td className="px-3 py-2.5 font-mono tabular-nums text-foreground">{itemStats ? itemStats.scans.toLocaleString() : '—'}</td><td className="px-3 py-2.5">{itemStats ? <span className="inline-flex items-center gap-1.5 text-success-foreground"><Check className="size-3.5" />{itemStats.hitRate.toFixed(1)}%</span> : <span className="text-muted-foreground">—</span>}</td><td className="px-3 py-2.5 text-muted-foreground">{itemStats?.lastUsed ?? 'Not collected'}</td></tr>})}</tbody></table></div> : <EmptyState compact icon={<DatabaseZap />} title="No indexes found" description="Create an access path or change the search term." />}
    </WorkspacePanel>
    <WorkspacePanel title={selected?.name ?? 'Index details'} description={selected ? `${selected.type} access method` : 'Select an index'}>
      {selected ? <div className="space-y-5">{stats ? <div className="grid grid-cols-2 gap-2"><div className="rounded-lg border border-border bg-background/60 p-3"><BarChart3 className="size-4 text-primary" /><p className="mt-2 font-mono text-lg font-semibold tabular-nums">{stats.scans.toLocaleString()}</p><p className="text-[length:var(--font-size-meta)] text-muted-foreground">Index scans</p></div><div className="rounded-lg border border-border bg-background/60 p-3"><Activity className="size-4 text-primary" /><p className="mt-2 font-mono text-lg font-semibold tabular-nums">{stats.hitRate.toFixed(1)}%</p><p className="text-[length:var(--font-size-meta)] text-muted-foreground">Buffer hit</p></div></div> : <div className="rounded-lg border border-border bg-background/60 p-3 text-[length:var(--font-size-meta)] leading-5 text-muted-foreground">Usage and size statistics are not part of the table schema response. Open Operations for live performance data.</div>}<div><p className="text-[length:var(--font-size-meta)] font-medium text-muted-foreground">Definition</p><pre className="mt-2 overflow-auto rounded-lg border border-border bg-background/70 p-3 whitespace-pre-wrap font-mono text-[length:var(--font-size-data)] leading-5 text-foreground">{selected.definition}</pre></div><Button className="w-full" size="xs" variant="outline" disabled={readOnly || selected.primary} onClick={() => setEditing(selected)}><Pencil />Edit definition</Button><div className="grid grid-cols-2 gap-2"><Button size="xs" variant="outline" disabled={readOnly} onClick={() => setAction({ kind: 'analyze', index: selected })}><WandSparkles />Analyze</Button><Button size="xs" variant="outline" disabled={readOnly || isMySql(engine)} title={isMySql(engine) ? 'Individual index rebuild is unavailable for MySQL and MariaDB' : undefined} onClick={() => setAction({ kind: 'rebuild', index: selected })}><RefreshCw />Rebuild</Button></div><Button className="w-full text-destructive hover:text-destructive" size="xs" variant="ghost" disabled={readOnly || selected.primary} onClick={() => setAction({ kind: 'drop', index: selected })}><Trash2 />Drop index</Button>{selected.primary ? <p className="text-center text-[length:var(--font-size-meta)] text-muted-foreground"><ShieldCheck className="mr-1 inline size-3.5" />Drop the primary constraint to remove this index.</p> : null}</div> : <EmptyState compact title="No index selected" />}
    </WorkspacePanel>
    {adding ? <AddIndexDialog open tableName={tableName} columns={columns} engine={engine} onOpenChange={setAdding} onSubmit={submitIndex} /> : null}
    {editing ? <AddIndexDialog key={editing.name} open tableName={tableName} columns={columns} engine={engine} initial={{ name: editing.name, method: editing.type, columns: editing.definition.match(/\(([^)]+)\)/)?.[1].split(',').map((column) => column.trim().split(/\s+/)[0]) ?? [], unique: editing.unique, predicate: isMySql(engine) ? '' : editing.definition.match(/\sWHERE\s(.+)$/i)?.[1] ?? '' }} onOpenChange={(open) => { if (!open) setEditing(undefined) }} onSubmit={submitIndex} /> : null}
    <Dialog open={Boolean(action)} onOpenChange={(open) => { if (!open) setAction(undefined) }}><DialogContent><DialogHeader><DialogTitle>{action?.kind === 'drop' ? 'Drop index?' : action?.kind === 'rebuild' ? 'Rebuild index?' : 'Analyze table?'}</DialogTitle><DialogDescription>{action?.kind === 'drop' ? `Queries may slow down after ${action.index.name} is removed.` : action?.kind === 'rebuild' ? `DataDock will request an engine-safe rebuild preview for ${action?.index.name}.` : 'Optimizer statistics will be refreshed after the generated statement is reviewed and applied.'}</DialogDescription></DialogHeader><div className="overflow-x-auto rounded-lg border border-border bg-background/70 px-3 py-2.5 font-mono text-[length:var(--font-size-data)]"><code className="whitespace-nowrap">{action?.kind === 'drop' ? dropIndexSql(action.index, engine) : action?.kind === 'rebuild' ? `REINDEX INDEX ${action?.index.name};` : 'ANALYZE {{table}};'}</code></div><DialogFooter><Button variant="outline" onClick={() => setAction(undefined)}>Cancel</Button><Button variant={action?.kind === 'drop' ? 'destructive' : 'default'} onClick={confirmAction}>{action?.kind === 'drop' ? <Trash2 /> : <RefreshCw />}{action?.kind === 'drop' ? 'Stage drop' : action?.kind === 'rebuild' ? 'Stage rebuild' : 'Stage analyze'}</Button></DialogFooter></DialogContent></Dialog>
  </div>
}

import { useMemo, useState } from 'react'
import { CheckCircle2, ChevronRight, KeyRound, Link2, Plus, Search, ShieldCheck, Trash2 } from 'lucide-react'
import type { DatabaseEngine } from '@/entities/connection'
import type { TableConstraint } from '@/entities/database-object'
import type { SchemaConstraintDefinition, SchemaDraftChange, SchemaTarget } from '@/entities/schema'
import type { SchemaColumn } from '@/features/schema/ColumnsPanel'
import {
  Badge,
  Button,
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
  Textarea,
  WorkspacePanel,
} from '@/shared/ui'

type ConstraintKind = 'PRIMARY KEY' | 'FOREIGN KEY' | 'UNIQUE' | 'CHECK'

type ConstraintDraft = {
  name: string
  type: ConstraintKind
  columns: string[]
  referencedTable: string
  referencedColumn: string
  expression: string
  onDelete: string
  onUpdate: string
}

type ConstraintsPanelProps = {
  constraints: TableConstraint[]
  columns: SchemaColumn[]
  engine: DatabaseEngine
  readOnly?: boolean
  onConstraintsChange: (constraints: TableConstraint[]) => void
  onStage: (change: SchemaDraftChange) => void
}

function dropConstraintSql(constraint: TableConstraint, engine: DatabaseEngine) {
  if (engine !== 'mysql' && engine !== 'mariadb') return `ALTER TABLE {{table}} DROP CONSTRAINT ${constraint.name};`
  const type = constraint.type.toUpperCase()
  if (type === 'PRIMARY KEY') return 'ALTER TABLE {{table}} DROP PRIMARY KEY;'
  if (type === 'FOREIGN KEY') return `ALTER TABLE {{table}} DROP FOREIGN KEY ${constraint.name};`
  if (type === 'UNIQUE') return `ALTER TABLE {{table}} DROP INDEX ${constraint.name};`
  return `ALTER TABLE {{table}} DROP CHECK ${constraint.name};`
}

const kindMeta: Record<ConstraintKind, { icon: typeof KeyRound; badge: 'warning' | 'info' | 'accent' | 'success'; label: string }> = {
  'PRIMARY KEY': { icon: KeyRound, badge: 'warning', label: 'Primary key' },
  'FOREIGN KEY': { icon: Link2, badge: 'info', label: 'Foreign key' },
  UNIQUE: { icon: ShieldCheck, badge: 'accent', label: 'Unique' },
  CHECK: { icon: CheckCircle2, badge: 'success', label: 'Check' },
}

function emptyDraft(type: ConstraintKind = 'FOREIGN KEY'): ConstraintDraft {
  return { name: '', type, columns: [], referencedTable: 'public.', referencedColumn: 'id', expression: '', onDelete: 'NO ACTION', onUpdate: 'NO ACTION' }
}

function constraintClause(draft: ConstraintDraft) {
  if (draft.type === 'PRIMARY KEY') return `PRIMARY KEY (${draft.columns.join(', ')})`
  if (draft.type === 'UNIQUE') return `UNIQUE (${draft.columns.join(', ')})`
  if (draft.type === 'CHECK') return `CHECK (${draft.expression.trim() || 'expression'})`
  return `FOREIGN KEY (${draft.columns.join(', ')}) REFERENCES ${draft.referencedTable || 'public.table'} (${draft.referencedColumn || 'id'}) ON DELETE ${draft.onDelete} ON UPDATE ${draft.onUpdate}`
}

function referencedColumns(draft: ConstraintDraft) {
  return draft.referencedColumn.split(',').map((column) => column.trim()).filter(Boolean)
}

function targetFromReference(reference: string): SchemaTarget {
  const parts = reference.split('.').filter(Boolean)
  return parts.length > 1 ? { schema: parts.slice(0, -1).join('.'), table: parts.at(-1) ?? reference } : { table: reference }
}

function schemaConstraint(draft: ConstraintDraft): SchemaConstraintDefinition {
  if (draft.type === 'PRIMARY KEY') return { name: draft.name, type: 'primary_key', columns: [...draft.columns] }
  if (draft.type === 'UNIQUE') return { name: draft.name, type: 'unique', columns: [...draft.columns] }
  if (draft.type === 'CHECK') return { name: draft.name, type: 'check', expression: draft.expression.trim() }
  return {
    name: draft.name,
    type: 'foreign_key',
    columns: [...draft.columns],
    referencedTarget: targetFromReference(draft.referencedTable.trim()),
    referencedColumns: referencedColumns(draft),
    onDelete: draft.onDelete.toLowerCase(),
    onUpdate: draft.onUpdate.toLowerCase(),
  }
}

function AddConstraintDialog({ open, columns, onOpenChange, onSubmit }: { open: boolean; columns: SchemaColumn[]; onOpenChange: (open: boolean) => void; onSubmit: (draft: ConstraintDraft) => void }) {
  const [draft, setDraft] = useState<ConstraintDraft>(emptyDraft())
  const valid = draft.name.trim() && (draft.type === 'CHECK' ? draft.expression.trim() : draft.columns.length > 0) && (draft.type !== 'FOREIGN KEY' || Boolean(draft.referencedTable.replace('public.', '').trim()) && referencedColumns(draft).length === draft.columns.length)

  function toggleColumn(name: string) {
    setDraft((value) => ({ ...value, columns: value.columns.includes(name) ? value.columns.filter((column) => column !== name) : [...value.columns, name] }))
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader><DialogTitle>Create constraint</DialogTitle><DialogDescription>Compose a structured constraint action. DataDock will request dialect-safe SQL from the backend before anything is applied.</DialogDescription></DialogHeader>
        <div className="grid gap-4 sm:grid-cols-2">
          <PropertyField label="Constraint name" required><Input autoFocus value={draft.name} placeholder="users_account_id_fkey" onChange={(event) => setDraft((value) => ({ ...value, name: event.target.value }))} /></PropertyField>
          <PropertyField label="Type" required><Select value={draft.type} onChange={(event) => setDraft({ ...emptyDraft(event.target.value as ConstraintKind), name: draft.name })}><option value="PRIMARY KEY">Primary key</option><option value="FOREIGN KEY">Foreign key</option><option value="UNIQUE">Unique</option><option value="CHECK">Check</option></Select></PropertyField>
        </div>
        {draft.type !== 'CHECK' ? <PropertyField label="Columns" required description="Composite constraints preserve the selected order."><div className="flex flex-wrap gap-2 rounded-lg border border-border bg-background/60 p-2.5">{columns.map((column) => <button key={column.id} type="button" aria-pressed={draft.columns.includes(column.name)} onClick={() => toggleColumn(column.name)} className="rounded-md border border-border bg-surface px-2.5 py-1.5 font-mono text-[length:var(--font-size-data)] text-muted-foreground transition-colors hover:bg-accent aria-pressed:border-primary aria-pressed:bg-accent aria-pressed:text-accent-foreground">{column.name}</button>)}</div></PropertyField> : null}
        {draft.type === 'FOREIGN KEY' ? <div className="grid gap-4 sm:grid-cols-2"><PropertyField label="Referenced table" required><Input className="font-mono" value={draft.referencedTable} onChange={(event) => setDraft((value) => ({ ...value, referencedTable: event.target.value }))} /></PropertyField><PropertyField label="Referenced columns" required description="Comma-separated in the same order as the selected columns."><Input className="font-mono" value={draft.referencedColumn} placeholder="id, tenant_id" onChange={(event) => setDraft((value) => ({ ...value, referencedColumn: event.target.value }))} /></PropertyField><PropertyField label="On delete"><Select value={draft.onDelete} onChange={(event) => setDraft((value) => ({ ...value, onDelete: event.target.value }))}><option>NO ACTION</option><option>CASCADE</option><option>SET NULL</option><option>RESTRICT</option></Select></PropertyField><PropertyField label="On update"><Select value={draft.onUpdate} onChange={(event) => setDraft((value) => ({ ...value, onUpdate: event.target.value }))}><option>NO ACTION</option><option>CASCADE</option><option>SET NULL</option><option>RESTRICT</option></Select></PropertyField></div> : null}
        {draft.type === 'CHECK' ? <PropertyField label="Check expression" required description="Write only the expression inside CHECK (...)."><Textarea className="font-mono" value={draft.expression} placeholder="mrr >= 0" onChange={(event) => setDraft((value) => ({ ...value, expression: event.target.value }))} /></PropertyField> : null}
        <div className="rounded-lg border border-border bg-background/70 p-3"><p className="text-[length:var(--font-size-meta)] font-medium text-muted-foreground">Generated clause</p><code className="mt-1.5 block whitespace-pre-wrap text-[length:var(--font-size-data)] leading-5 text-foreground">CONSTRAINT {draft.name || 'constraint_name'} {constraintClause(draft)}</code></div>
        <DialogFooter><Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button><Button disabled={!valid} onClick={() => onSubmit({ ...draft, name: draft.name.trim() })}><Plus />Stage constraint</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function ConstraintsPanel({ constraints, columns, engine, readOnly, onConstraintsChange, onStage }: ConstraintsPanelProps) {
  const [search, setSearch] = useState('')
  const [type, setType] = useState<'ALL' | ConstraintKind>('ALL')
  const [selectedName, setSelectedName] = useState(constraints[0]?.name ?? '')
  const [adding, setAdding] = useState(false)
  const [deleteName, setDeleteName] = useState<string>()
  const visible = useMemo(() => {
    const query = search.trim().toLowerCase()
    return constraints.filter((constraint) => (type === 'ALL' || constraint.type.toUpperCase() === type) && (!query || `${constraint.name} ${constraint.type} ${constraint.columns.join(' ')} ${constraint.definition}`.toLowerCase().includes(query)))
  }, [constraints, search, type])
  const selected = visible.find((constraint) => constraint.name === selectedName) ?? visible[0]
  const deleting = constraints.find((constraint) => constraint.name === deleteName)

  function addConstraint(draft: ConstraintDraft) {
    const definition = constraintClause(draft)
    const next: TableConstraint = { name: draft.name, type: draft.type, columns: draft.columns, definition }
    onConstraintsChange([...constraints, next])
    setSelectedName(next.name)
    onStage({ label: `Add ${draft.name}`, description: kindMeta[draft.type].label, actions: [{ kind: 'add_constraint', constraint: schemaConstraint(draft) }] })
    setAdding(false)
  }

  function confirmDelete() {
    if (!deleting) return
    const next = constraints.filter((constraint) => constraint.name !== deleting.name)
    onConstraintsChange(next)
    setSelectedName(next[0]?.name ?? '')
    onStage({ label: `Drop ${deleting.name}`, description: `${deleting.type} constraint`, actions: [{ kind: 'drop_constraint', name: deleting.name, cascade: false }] })
    setDeleteName(undefined)
  }

  return (
    <div className="grid min-h-0 flex-1 grid-cols-[minmax(0,1.55fr)_minmax(17rem,0.72fr)] gap-4 max-xl:grid-cols-1">
      <WorkspacePanel noPadding title="Constraints" description={`${constraints.length} safeguards across ${columns.length} columns`} actions={<Button size="xs" disabled={readOnly} onClick={() => setAdding(true)}><Plus />New constraint</Button>}>
        <div className="flex min-h-[var(--toolbar-height)] flex-wrap items-center gap-2 border-b border-border px-3 py-1.5">
          <div className="relative min-w-48 flex-1 max-w-sm"><Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" /><Input aria-label="Filter constraints" className="h-[var(--control-height-sm)] pl-8" placeholder="Filter constraints" value={search} onChange={(event) => setSearch(event.target.value)} /></div>
          <Select aria-label="Constraint type" className="h-[var(--control-height-sm)] w-36" value={type} onChange={(event) => setType(event.target.value as typeof type)}><option value="ALL">All types</option><option value="PRIMARY KEY">Primary key</option><option value="FOREIGN KEY">Foreign key</option><option value="UNIQUE">Unique</option><option value="CHECK">Check</option></Select>
        </div>
        {visible.length ? <div className="divide-y divide-border">{visible.map((constraint) => {
          const normalizedType = constraint.type.toUpperCase() as ConstraintKind
          const meta = kindMeta[normalizedType] ?? kindMeta.CHECK
          const Icon = meta.icon
          return <button key={constraint.name} type="button" aria-pressed={selected?.name === constraint.name} onClick={() => setSelectedName(constraint.name)} className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-accent/35 aria-pressed:bg-accent/65"><span className="grid size-9 shrink-0 place-items-center rounded-lg border border-border bg-background text-primary"><Icon className="size-4" /></span><span className="min-w-0 flex-1"><span className="flex flex-wrap items-center gap-2"><strong className="truncate font-mono text-[length:var(--font-size-data)] font-medium text-foreground">{constraint.name}</strong><Badge variant={meta.badge}>{meta.label}</Badge></span><span className="mt-1 block truncate font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{constraint.definition}</span></span><ChevronRight className="size-4 shrink-0 text-muted-foreground" /></button>
        })}</div> : <EmptyState compact icon={<ShieldCheck />} title="No constraints found" description="Adjust the filter or create a new database safeguard." />}
      </WorkspacePanel>

      <WorkspacePanel title={selected?.name ?? 'Constraint details'} description={selected ? selected.type : 'Select a constraint'} actions={selected ? <Button size="xs" variant="ghost" className="text-destructive hover:text-destructive" disabled={readOnly} onClick={() => setDeleteName(selected.name)}><Trash2 />Drop</Button> : null}>
        {selected ? <div className="space-y-5">
          <div><p className="text-[length:var(--font-size-meta)] font-medium text-muted-foreground">Columns</p><div className="mt-2 flex flex-wrap gap-1.5">{selected.columns.length ? selected.columns.map((column) => <Badge key={column} variant="outline" className="rounded-md font-mono font-normal">{column}</Badge>) : <span className="text-[length:var(--font-size-ui)] text-muted-foreground">Expression-based</span>}</div></div>
          <div><p className="text-[length:var(--font-size-meta)] font-medium text-muted-foreground">Definition</p><pre className="mt-2 overflow-auto rounded-lg border border-border bg-background/70 p-3 whitespace-pre-wrap font-mono text-[length:var(--font-size-data)] leading-5 text-foreground">{selected.definition}</pre></div>
          {selected.type.toUpperCase() === 'FOREIGN KEY' ? <div className="rounded-lg border border-info/30 bg-info/50 p-3"><div className="flex items-center gap-2 text-[length:var(--font-size-ui)] font-medium text-info-foreground"><Link2 className="size-4" />Relationship</div><p className="mt-1.5 text-[length:var(--font-size-meta)] leading-5 text-muted-foreground">Values must resolve to the referenced record. Deleting a parent follows the configured action.</p></div> : null}
        </div> : <EmptyState compact title="No constraint selected" />}
      </WorkspacePanel>

      {adding ? <AddConstraintDialog open columns={columns} onOpenChange={setAdding} onSubmit={addConstraint} /> : null}
      <Dialog open={Boolean(deleting)} onOpenChange={(open) => { if (!open) setDeleteName(undefined) }}><DialogContent><DialogHeader><DialogTitle>Drop {deleting?.name}?</DialogTitle><DialogDescription>The database will stop enforcing this {deleting?.type.toLowerCase()} after the destructive backend preview is confirmed and applied.</DialogDescription></DialogHeader><div className="overflow-x-auto rounded-lg border border-destructive/25 bg-destructive/10 px-3 py-2.5 font-mono text-[length:var(--font-size-data)] text-destructive"><code className="whitespace-nowrap">{deleting ? dropConstraintSql(deleting, engine) : ''}</code></div><DialogFooter><Button variant="outline" onClick={() => setDeleteName(undefined)}>Keep constraint</Button><Button variant="destructive" onClick={confirmDelete}><Trash2 />Stage drop</Button></DialogFooter></DialogContent></Dialog>
    </div>
  )
}

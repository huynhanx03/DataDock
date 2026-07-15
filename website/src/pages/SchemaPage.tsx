import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, ArrowRight, Braces, CheckCircle2, Columns3, Database, FileCode2, GitBranch, KeyRound, Layers3, Network, Pencil, RefreshCw, Save, TableProperties, Trash2, X } from 'lucide-react'
import type { Connection } from '@/entities/connection'
import type { TableColumn, TableConstraint, TableIndex } from '@/entities/database-object'
import type { SchemaAction, SchemaDraftChange, SchemaTarget } from '@/entities/schema'
import { useDataDockGateway } from '@/app/providers'
import { ColumnsPanel, type SchemaColumn } from '@/features/schema/ColumnsPanel'
import { ConstraintsPanel } from '@/features/schema/ConstraintsPanel'
import { DdlPanel } from '@/features/schema/DdlPanel'
import { IndexesPanel } from '@/features/schema/IndexesPanel'
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
  Skeleton,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  WorkspaceHeader,
  WorkspacePage,
  WorkspacePanel,
  WorkspaceToolbar,
} from '@/shared/ui'

type SchemaPageProps = {
  connection: Connection
  table: string
  onDirtyChange?: (dirty: boolean) => void
}

type SchemaChange = SchemaDraftChange & {
  id: string
  actions: SchemaAction[]
}

function schemaColumns(columns: TableColumn[]): SchemaColumn[] {
  return columns.map((column, index) => ({ ...column, id: `${column.name}-${index}`, identity: column.identity ?? /identity|nextval/i.test(column.defaultValue ?? ''), collation: 'database_default' }))
}

function schemaTarget(reference: string): SchemaTarget {
  const parts = reference.split('.').filter(Boolean)
  return parts.length > 1 ? { schema: parts.at(-2), table: parts.at(-1) ?? reference } : { table: reference }
}

function MetricCard({ icon, label, value, detail, source }: { icon: React.ReactNode; label: string; value: string | number; detail: string; source: 'mock' | 'api' }) {
  return <div className="rounded-xl border border-border bg-surface p-4 shadow-control"><div className="flex items-start justify-between gap-3"><span className="grid size-9 place-items-center rounded-lg bg-accent text-primary [&_svg]:size-4">{icon}</span><Badge variant="outline">{source === 'mock' ? 'Sample catalog' : 'Catalog'}</Badge></div><p className="mt-4 text-2xl font-semibold tracking-[-0.04em] tabular-nums">{value}</p><p className="mt-0.5 text-[length:var(--font-size-ui)] font-medium text-foreground">{label}</p><p className="mt-1 text-[length:var(--font-size-meta)] text-muted-foreground">{detail}</p></div>
}

function RelationsPanel({ table, constraints }: { table: string; constraints: TableConstraint[] }) {
  const foreignKeys = constraints.filter((constraint) => constraint.type.toUpperCase() === 'FOREIGN KEY')
  const incoming: Array<{ table: string; column: string; target: string; constraint: string }> = []
  return <div className="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(19rem,0.65fr)]">
    <WorkspacePanel title="Relationship map" description="Foreign-key direction from the selected table">
      <div className="flex min-h-72 items-center justify-center overflow-auto rounded-xl border border-border bg-[radial-gradient(circle_at_center,color-mix(in_oklab,var(--primary)_8%,transparent),transparent_60%)] p-8">
        <div className="flex min-w-max items-center gap-4">
          <div className="space-y-3">{incoming.length ? incoming.map((relation) => <div key={relation.constraint} className="w-52 rounded-xl border border-border bg-surface p-3 shadow-control"><div className="flex items-center gap-2"><TableProperties className="size-4 text-primary" /><span className="font-mono text-[length:var(--font-size-data)] font-medium">{relation.table}</span></div><p className="mt-2 font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{relation.column} → {relation.target}</p></div>) : <div className="w-52 rounded-xl border border-dashed border-border p-3 text-center text-[length:var(--font-size-meta)] text-muted-foreground">No incoming references</div>}</div>
          <div className="flex items-center gap-1 text-muted-foreground"><span className="h-px w-10 bg-border-strong" /><ArrowRight className="size-4" /></div>
          <div className="w-56 rounded-2xl border-2 border-primary/55 bg-accent p-4 shadow-panel"><div className="flex items-center gap-2 text-primary"><Database className="size-4" /><Badge variant="accent">Selected</Badge></div><p className="mt-3 truncate font-mono text-sm font-semibold text-foreground">{table}</p><p className="mt-1 text-[length:var(--font-size-meta)] text-muted-foreground">{constraints.length} constraints</p></div>
          <div className="flex items-center gap-1 text-muted-foreground"><span className="h-px w-10 bg-border-strong" /><ArrowRight className="size-4" /></div>
          <div className="space-y-3">{foreignKeys.length ? foreignKeys.map((constraint) => <div key={constraint.name} className="w-56 rounded-xl border border-border bg-surface p-3 shadow-control"><div className="flex items-center gap-2"><Network className="size-4 text-primary" /><span className="truncate font-mono text-[length:var(--font-size-data)] font-medium">{constraint.definition.match(/REFERENCES\s+([^\s(]+)/i)?.[1] ?? 'Referenced table'}</span></div><p className="mt-2 truncate font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{constraint.name}</p></div>) : <div className="w-56 rounded-xl border border-dashed border-border p-3 text-center text-[length:var(--font-size-meta)] text-muted-foreground">No outgoing references</div>}</div>
        </div>
      </div>
    </WorkspacePanel>
    <WorkspacePanel title="Relation summary" description={`${foreignKeys.length} outgoing relationships`}>
      <div className="space-y-4"><div className="rounded-lg border border-border bg-background/65 p-3"><p className="text-[length:var(--font-size-meta)] font-medium text-muted-foreground">Referenced by</p><p className="mt-1 text-xl font-semibold">Not collected</p><p className="text-[length:var(--font-size-meta)] text-muted-foreground">Incoming references require a catalog-wide relation query.</p></div><div className="rounded-lg border border-border bg-background/65 p-3"><p className="text-[length:var(--font-size-meta)] font-medium text-muted-foreground">References</p><p className="mt-1 text-xl font-semibold tabular-nums">{foreignKeys.length}</p><p className="text-[length:var(--font-size-meta)] text-muted-foreground">foreign keys reported for this table</p></div><div className="rounded-lg border border-info/25 bg-info/45 p-3 text-[length:var(--font-size-meta)] leading-5 text-muted-foreground"><GitBranch className="mr-1.5 inline size-3.5 text-info-foreground" />Only relationships returned by the current table schema are displayed.</div></div>
    </WorkspacePanel>
  </div>
}

export function SchemaPage({ connection, table, onDirtyChange }: SchemaPageProps) {
  const gateway = useDataDockGateway()
  const queryClient = useQueryClient()
  const schema = useQuery({ queryKey: ['table-schema', connection.id, table], queryFn: ({ signal }) => gateway.getTableSchema(connection.id, table, signal) })
  const ddl = useQuery({ queryKey: ['table-ddl', connection.id, table], queryFn: ({ signal }) => gateway.getTableDDL(connection.id, table, signal) })
  const [columns, setColumns] = useState<SchemaColumn[]>([])
  const [constraints, setConstraints] = useState<TableConstraint[]>([])
  const [indexes, setIndexes] = useState<TableIndex[]>([])
  const [changes, setChanges] = useState<SchemaChange[]>([])
  const [renameOpen, setRenameOpen] = useState(false)
  const [dropOpen, setDropOpen] = useState(false)
  const [refreshOpen, setRefreshOpen] = useState(false)
  const [reviewOpen, setReviewOpen] = useState(false)
  const [newName, setNewName] = useState(table.split('.').at(-1) ?? table)
  const [confirmDestructive, setConfirmDestructive] = useState(false)
  const [appliedNotice, setAppliedNotice] = useState<{ at: number; steps: number }>()
  const initialized = useRef('')
  const key = `${connection.id}:${table}`
  const tableName = table.split('.').at(-1) ?? table
  const schemaName = table.includes('.') ? table.split('.').at(-2) ?? connection.database : connection.database
  const target = useMemo(() => schemaTarget(table), [table])
  const actions = useMemo(() => changes.flatMap((change) => change.actions), [changes])
  const actionSignature = useMemo(() => JSON.stringify(actions), [actions])
  const preview = useQuery({
    queryKey: ['schema-preview', connection.id, table, actionSignature],
    queryFn: ({ signal }) => gateway.previewSchema(connection.id, actions, signal),
    enabled: actions.length > 0,
    retry: false,
    staleTime: Number.POSITIVE_INFINITY,
  })
  const apply = useMutation({
    mutationFn: async () => {
      if (!preview.data) throw new Error('A valid schema preview is required before applying changes.')
      return gateway.applySchema(connection.id, {
        actions: preview.data.actions,
        previewHash: preview.data.hash,
        confirmDestructive: preview.data.destructive && confirmDestructive,
      })
    },
    onSuccess: async (result) => {
      setAppliedNotice({ at: Date.now(), steps: result.appliedSteps })
      setReviewOpen(false)
      setConfirmDestructive(false)
      initialized.current = ''
      discardDraft()
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['table-schema', connection.id, table] }),
        queryClient.invalidateQueries({ queryKey: ['table-ddl', connection.id, table] }),
        queryClient.invalidateQueries({ queryKey: ['catalog', connection.id] }),
      ])
    },
  })

  useEffect(() => {
    initialized.current = ''
    setChanges([])
    setAppliedNotice(undefined)
    setConfirmDestructive(false)
    setNewName(tableName)
  }, [key, tableName])

  useEffect(() => {
    onDirtyChange?.(changes.length > 0)
  }, [changes.length, onDirtyChange])

  useEffect(() => () => onDirtyChange?.(false), [onDirtyChange])

  useEffect(() => {
    if (!schema.data || initialized.current === key) return
    hydrateDesigner(schema.data)
    initialized.current = key
  }, [key, schema.data, schema.dataUpdatedAt])

  const generatedSql = preview.data?.sql ?? ''

  function hydrateDesigner(data: NonNullable<typeof schema.data>) {
    setColumns(schemaColumns(data.columns))
    setConstraints(data.constraints)
    setIndexes(data.indexes)
  }

  function stage(change: SchemaDraftChange) {
    setChanges((current) => {
      const id = `${Date.now()}-${current.length}`
      return [...current, { ...change, id, actions: change.actions.map((action, index) => ({ ...action, id: `draft-${id}-${index}`, target })) }]
    })
    setAppliedNotice(undefined)
  }

  function discardDraft() {
    if (schema.data) {
      hydrateDesigner(schema.data)
    }
    setChanges([])
    setNewName(tableName)
  }

  async function refreshSchema() {
    initialized.current = ''
    setChanges([])
    setAppliedNotice(undefined)
    setNewName(tableName)
    const [schemaResult] = await Promise.all([schema.refetch(), ddl.refetch()])
    if (schemaResult.data) {
      hydrateDesigner(schemaResult.data)
      initialized.current = key
    }
  }

  function requestRefresh() {
    if (changes.length) setRefreshOpen(true)
    else refreshSchema()
  }

  function stageRename() {
    const next = newName.trim()
    if (!next || next === tableName) return
    stage({ label: `Rename ${tableName}`, description: `${tableName} → ${next}`, actions: [{ kind: 'rename_table', newName: next }] })
    setRenameOpen(false)
  }

  function stageDrop() {
    stage({ label: `Drop ${tableName}`, description: 'Destructive table change', actions: [{ kind: 'drop_table', cascade: false }] })
    setDropOpen(false)
  }

  if (schema.isError) return <WorkspacePage><EmptyState icon={<AlertTriangle />} title="Schema could not be loaded" description={schema.error.message} actions={<Button variant="outline" onClick={() => schema.refetch()}><RefreshCw />Retry</Button>} /></WorkspacePage>

  return <WorkspacePage>
    <WorkspaceHeader icon={<Layers3 />} eyebrow="Schema studio" title={tableName} description={<><span className="font-mono text-[length:var(--font-size-data)]">{schemaName}.{tableName}</span><span className="mx-2">·</span>{connection.name}</>} meta={<div className="flex items-center gap-2"><Badge variant={gateway.source === 'mock' ? 'accent' : 'outline'}>{gateway.source === 'mock' ? 'Sample catalog' : 'Live catalog'}</Badge><Badge variant={connection.readOnly ? 'warning' : 'outline'}>{connection.readOnly ? 'Read only' : 'Preview mode'}</Badge></div>} actions={<><Button size="sm" variant="outline" disabled={connection.readOnly} onClick={() => { setNewName(tableName); setRenameOpen(true) }}><Pencil />Rename</Button><Button size="sm" variant="ghost" className="text-destructive hover:text-destructive" disabled={connection.readOnly} onClick={() => setDropOpen(true)}><Trash2 />Drop table</Button></>} />
    {appliedNotice ? <div aria-live="polite" className="flex shrink-0 items-center gap-2 border-b border-success/30 bg-success/45 px-5 py-2 text-[length:var(--font-size-ui)] text-success-foreground"><CheckCircle2 className="size-4" /><span>{appliedNotice.steps} schema {appliedNotice.steps === 1 ? 'step was' : 'steps were'} applied at {new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(appliedNotice.at)}.</span><Button className="ml-auto" size="icon-xs" variant="ghost" aria-label="Dismiss apply notice" onClick={() => setAppliedNotice(undefined)}><X /></Button></div> : null}
    <Tabs defaultValue="overview" className="min-h-0 flex-1 gap-0 overflow-hidden">
      <WorkspaceToolbar className="overflow-x-auto"><TabsList className="h-auto min-w-max border-0 bg-transparent p-0"><TabsTrigger value="overview"><TableProperties />Overview</TabsTrigger><TabsTrigger value="columns"><Columns3 />Columns <Badge variant="outline" className="ml-1 px-1.5">{columns.length}</Badge></TabsTrigger><TabsTrigger value="constraints"><KeyRound />Constraints <Badge variant="outline" className="ml-1 px-1.5">{constraints.length}</Badge></TabsTrigger><TabsTrigger value="indexes"><Database />Indexes <Badge variant="outline" className="ml-1 px-1.5">{indexes.length}</Badge></TabsTrigger><TabsTrigger value="relations"><Network />Relations</TabsTrigger><TabsTrigger value="ddl"><FileCode2 />DDL</TabsTrigger></TabsList><div className="ml-auto flex items-center gap-2"><span className="text-[length:var(--font-size-meta)] text-muted-foreground">{schema.data ? 'Catalog loaded' : 'Loading catalog…'}</span><Button size="icon-xs" variant="ghost" aria-label="Refresh schema catalog" disabled={schema.isFetching || ddl.isFetching} onClick={requestRefresh}><RefreshCw className={schema.isFetching || ddl.isFetching ? 'animate-spin' : ''} /></Button></div></WorkspaceToolbar>
      <TabsContent value="overview" className="min-h-0 overflow-auto p-4 lg:p-5">
        {schema.isLoading ? <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">{Array.from({ length: 4 }, (_, index) => <Skeleton key={index} className="h-40" />)}</div> : <div className="space-y-4"><div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4"><MetricCard icon={<Columns3 />} label="Columns" value={columns.length} detail={`${columns.filter((column) => column.nullable).length} nullable`} source={gateway.source} /><MetricCard icon={<KeyRound />} label="Constraints" value={constraints.length} detail={`${constraints.filter((item) => item.type.toUpperCase().includes('KEY')).length} key constraints`} source={gateway.source} /><MetricCard icon={<Database />} label="Indexes" value={indexes.length} detail="Definitions returned by the catalog" source={gateway.source} /><MetricCard icon={<FileCode2 />} label="Documented columns" value={`${columns.filter((column) => column.comment).length}/${columns.length}`} detail="Column comments in the catalog" source={gateway.source} /></div><div className="grid gap-4 xl:grid-cols-[minmax(0,1.4fr)_minmax(18rem,0.6fr)]"><WorkspacePanel title="Table profile" description="Properties returned by the live catalog"><dl className="grid gap-x-8 gap-y-4 sm:grid-cols-2 text-[length:var(--font-size-ui)]"><div><dt className="text-muted-foreground">Qualified name</dt><dd className="mt-1 font-mono text-foreground">{table}</dd></div><div><dt className="text-muted-foreground">Engine</dt><dd className="mt-1 text-foreground">{connection.engine}</dd></div><div><dt className="text-muted-foreground">Database</dt><dd className="mt-1 font-mono text-foreground">{connection.database}</dd></div><div><dt className="text-muted-foreground">Access</dt><dd className="mt-1 text-foreground">{connection.readOnly ? 'Read only' : 'Read and write'}</dd></div></dl></WorkspacePanel><WorkspacePanel title="Schema readiness" description={gateway.source === 'mock' ? 'Sample catalog analysis' : 'Live catalog analysis'}><div className="space-y-3"><div className="flex items-center justify-between rounded-lg border border-border bg-background/60 p-3"><span className="text-[length:var(--font-size-ui)]">Primary key</span><Badge variant={constraints.some((item) => item.type.toUpperCase() === 'PRIMARY KEY') ? 'success' : 'warning'}>{constraints.some((item) => item.type.toUpperCase() === 'PRIMARY KEY') ? 'Configured' : 'Missing'}</Badge></div><div className="flex items-center justify-between rounded-lg border border-border bg-background/60 p-3"><span className="text-[length:var(--font-size-ui)]">Documentation</span><Badge variant="outline">{columns.filter((column) => column.comment).length}/{columns.length} columns</Badge></div><div className="flex items-center justify-between rounded-lg border border-border bg-background/60 p-3"><span className="text-[length:var(--font-size-ui)]">Writable columns</span><Badge variant="outline">{columns.filter((column) => !column.generated).length}/{columns.length}</Badge></div></div></WorkspacePanel></div></div>}
      </TabsContent>
      <TabsContent value="columns" className="min-h-0 overflow-auto p-4 lg:p-5"><ColumnsPanel columns={columns} engine={connection.engine} readOnly={connection.readOnly} onColumnsChange={setColumns} onStage={stage} /></TabsContent>
      <TabsContent value="constraints" className="min-h-0 overflow-auto p-4 lg:p-5"><ConstraintsPanel constraints={constraints} columns={columns} engine={connection.engine} readOnly={connection.readOnly} onConstraintsChange={setConstraints} onStage={stage} /></TabsContent>
      <TabsContent value="indexes" className="min-h-0 overflow-auto p-4 lg:p-5"><IndexesPanel indexes={indexes} columns={columns} engine={connection.engine} table={table} source={gateway.source} readOnly={connection.readOnly} onIndexesChange={setIndexes} onStage={stage} /></TabsContent>
      <TabsContent value="relations" className="min-h-0 overflow-auto p-4 lg:p-5"><RelationsPanel table={table} constraints={constraints} /></TabsContent>
      <TabsContent value="ddl" className="min-h-0 overflow-auto p-4 lg:p-5"><DdlPanel table={table} ddl={ddl.data ?? ''} generatedSql={generatedSql} engine={connection.engine} source={gateway.source} loading={ddl.isLoading || preview.isFetching} /></TabsContent>
    </Tabs>
    {changes.length ? <div className="flex shrink-0 flex-wrap items-center gap-3 border-t border-primary/30 bg-accent/70 px-4 py-2.5 lg:px-5"><span className="grid size-8 place-items-center rounded-lg bg-primary text-primary-foreground"><Braces className="size-4" /></span><div><p className="text-[length:var(--font-size-ui)] font-semibold">{changes.length} staged schema {changes.length === 1 ? 'change' : 'changes'} · {actions.length} actions</p><p className="text-[length:var(--font-size-meta)] text-muted-foreground">{preview.isFetching ? 'Generating a server preview…' : preview.isError ? `Preview validation failed: ${preview.error instanceof Error ? preview.error.message : 'Unknown error'}` : preview.data ? `Preview ready · ${preview.data.steps.length} SQL steps · nothing applied yet.` : 'Waiting for preview.'}</p></div><div className="ml-auto flex items-center gap-2"><Button size="xs" variant="outline" onClick={discardDraft}><X />Discard</Button><Button size="xs" disabled={!preview.data || preview.isFetching} onClick={() => { apply.reset(); setConfirmDestructive(false); setReviewOpen(true) }}><Save />Review & apply</Button></div></div> : null}

    <Dialog open={renameOpen} onOpenChange={setRenameOpen}><DialogContent><DialogHeader><DialogTitle>Rename table</DialogTitle><DialogDescription>The rename will be staged as a structured action and validated by the database adapter before it can be applied.</DialogDescription></DialogHeader><PropertyField label="New table name" required><Input autoFocus value={newName} onChange={(event) => setNewName(event.target.value)} /></PropertyField><DialogFooter><Button variant="outline" onClick={() => setRenameOpen(false)}>Cancel</Button><Button disabled={!newName.trim() || newName.trim() === tableName} onClick={stageRename}>Stage rename</Button></DialogFooter></DialogContent></Dialog>
    <Dialog open={dropOpen} onOpenChange={setDropOpen}><DialogContent><DialogHeader><DialogTitle>Drop {table}?</DialogTitle><DialogDescription>This stages a destructive SQL preview that would remove the table, its data, indexes, constraints, and dependent objects if executed later.</DialogDescription></DialogHeader><div className="rounded-lg border border-destructive/25 bg-destructive/10 p-3"><p className="flex items-center gap-2 font-medium text-destructive"><AlertTriangle className="size-4" />The generated statement is destructive</p><code className="mt-2 block text-[length:var(--font-size-data)] text-destructive">DROP TABLE {table};</code></div><DialogFooter><Button variant="outline" onClick={() => setDropOpen(false)}>Keep table</Button><Button variant="destructive" onClick={stageDrop}><Trash2 />Stage drop</Button></DialogFooter></DialogContent></Dialog>
    <Dialog open={refreshOpen} onOpenChange={setRefreshOpen}><DialogContent><DialogHeader><DialogTitle>Refresh and discard schema draft?</DialogTitle><DialogDescription>The latest catalog metadata will replace the local designer state. Your {changes.length} staged {changes.length === 1 ? 'change' : 'changes'} cannot be restored.</DialogDescription></DialogHeader><DialogFooter><Button variant="outline" onClick={() => setRefreshOpen(false)}>Keep draft</Button><Button variant="destructive" onClick={() => { setRefreshOpen(false); refreshSchema() }}><RefreshCw />Discard and refresh</Button></DialogFooter></DialogContent></Dialog>
    <Dialog open={reviewOpen} onOpenChange={(open) => { if (!apply.isPending) setReviewOpen(open) }}><DialogContent className="max-w-3xl"><DialogHeader><DialogTitle>Review schema SQL</DialogTitle><DialogDescription>{preview.data ? `${preview.data.steps.length} SQL steps were generated by the ${preview.data.engine} adapter. Applying them changes ${connection.name}.` : 'The adapter preview is unavailable.'}</DialogDescription></DialogHeader>{preview.data ? <><div className="flex flex-wrap items-center gap-2"><Badge variant="outline">{preview.data.steps.length} steps</Badge><Badge variant={preview.data.destructive ? 'destructive' : 'success'}>{preview.data.destructive ? 'Destructive' : 'Non-destructive'}</Badge><code className="ml-auto max-w-full truncate font-mono text-[length:var(--font-size-meta)] text-muted-foreground" title={preview.data.hash}>{preview.data.hash}</code></div><div className="max-h-[50vh] overflow-auto rounded-lg border border-border bg-background/75"><pre className="m-0 min-w-max p-4 font-mono text-[length:var(--font-size-data)] leading-6 text-foreground">{preview.data.sql}</pre></div>{preview.data.destructive ? <div className="rounded-lg border border-destructive/30 bg-destructive/10 p-3"><Checkbox checked={confirmDestructive} label="I understand that these destructive steps can remove schema objects or data" onChange={(event) => setConfirmDestructive(event.target.checked)} /></div> : <div className="flex items-center gap-2 rounded-lg border border-success/30 bg-success/40 p-3 text-[length:var(--font-size-meta)] text-success-foreground"><CheckCircle2 className="size-4 shrink-0" />The adapter did not classify this preview as destructive.</div>}</> : <EmptyState compact icon={<AlertTriangle />} title="Schema preview unavailable" description={preview.error instanceof Error ? preview.error.message : 'Generate a new preview before applying changes.'} />} {apply.error ? <p role="alert" className="rounded-lg border border-destructive/25 bg-destructive/10 p-3 text-[length:var(--font-size-ui)] text-destructive">{apply.error.message}</p> : null}<DialogFooter><Button variant="outline" disabled={apply.isPending} onClick={() => setReviewOpen(false)}>Back to draft</Button><Button disabled={apply.isPending || !preview.data || (preview.data.destructive && !confirmDestructive)} onClick={() => apply.mutate()}>{apply.isPending ? <RefreshCw className="animate-spin" /> : <Save />}{apply.isPending ? 'Applying schema…' : gateway.source === 'mock' ? 'Apply to sample adapter' : `Apply ${preview.data?.steps.length ?? 0} steps`}</Button></DialogFooter></DialogContent></Dialog>
  </WorkspacePage>
}

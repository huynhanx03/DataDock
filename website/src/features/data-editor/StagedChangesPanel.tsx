import { useMemo, useState } from 'react'
import { AlertTriangle, ChevronDown, ChevronUp, CopyPlus, FilePlus2, PencilLine, Redo2, Trash2, Undo2, X } from 'lucide-react'
import type { StagedChange } from '@/features/data-editor/useStagedMutations'
import { Badge, Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, IconButton } from '@/shared/ui'

type ChangeSummary = {
  inserts: number
  updates: number
  deletes: number
  cells: number
}

type StagedChangesPanelProps = {
  table: string
  changes: StagedChange[]
  summary: ChangeSummary
  validationErrors: Map<string, string>
  canUndo: boolean
  canRedo: boolean
  applying: boolean
  applyError?: Error | null
  onUndo: () => void
  onRedo: () => void
  onDiscard: () => void
  onDiscardChange: (id: string) => void
  onApply: () => void
}

function valueLabel(value: unknown) {
  if (value === null || value === undefined) return 'NULL'
  const rendered = typeof value === 'object' ? JSON.stringify(value) : String(value)
  return rendered.length > 46 ? `${rendered.slice(0, 43)}…` : rendered
}

function changeTitle(change: StagedChange) {
  if (change.kind === 'insert') return change.source === 'duplicate' ? `Duplicate row ${change.rowId}` : `Insert row ${change.rowId}`
  if (change.kind === 'delete') return `Delete row ${change.rowId}`
  return `${change.rowId}.${change.column}`
}

function changeDescription(change: StagedChange) {
  if (change.kind === 'insert') return `${Object.keys(change.values).length} values staged`
  if (change.kind === 'delete') return 'Row will be removed when changes are applied'
  return `${valueLabel(change.original)} → ${valueLabel(change.value)}`
}

function ChangeIcon({ change }: { change: StagedChange }) {
  if (change.kind === 'delete') return <Trash2 className="size-3.5 text-destructive" />
  if (change.kind === 'insert' && change.source === 'duplicate') return <CopyPlus className="size-3.5 text-primary" />
  if (change.kind === 'insert') return <FilePlus2 className="size-3.5 text-success-foreground" />
  return <PencilLine className="size-3.5 text-warning-foreground" />
}

export function StagedChangesPanel({
  table,
  changes,
  summary,
  validationErrors,
  canUndo,
  canRedo,
  applying,
  applyError,
  onUndo,
  onRedo,
  onDiscard,
  onDiscardChange,
  onApply,
}: StagedChangesPanelProps) {
  const [expanded, setExpanded] = useState(false)
  const [previewOpen, setPreviewOpen] = useState(false)
  const groupedErrors = useMemo(() => Array.from(new Set(validationErrors.values())), [validationErrors])

  return (
    <>
      <div className="shrink-0 border-t border-primary/25 bg-surface shadow-[0_-10px_26px_-24px_var(--foreground)]">
        <div className="flex min-h-[var(--toolbar-height)] flex-wrap items-center gap-2 px-4 py-2 lg:px-5">
          <button type="button" className="flex min-w-0 items-center gap-2 rounded-md text-left focus-visible:ring-2 focus-visible:ring-ring" onClick={() => setExpanded((value) => !value)} aria-expanded={expanded}>
            <span className="grid size-7 shrink-0 place-items-center rounded-md bg-accent text-primary"><PencilLine className="size-3.5" /></span>
            <span className="font-medium text-foreground">Staged changes</span>
            <Badge variant={validationErrors.size ? 'destructive' : changes.length ? 'secondary' : 'outline'}>{changes.length}</Badge>
            {expanded ? <ChevronDown className="size-3.5 text-muted-foreground" /> : <ChevronUp className="size-3.5 text-muted-foreground" />}
          </button>
          {changes.length ? (
            <div className="hidden items-center gap-1.5 text-[length:var(--font-size-meta)] text-muted-foreground sm:flex">
              <span>{summary.inserts} insert</span><span>·</span><span>{summary.updates} update</span><span>·</span><span>{summary.deletes} delete</span>
            </div>
          ) : <span className="text-[length:var(--font-size-meta)] text-muted-foreground">Double-click a cell or add a row to begin.</span>}
          <div className="ml-auto flex items-center gap-1.5">
            <IconButton label="Undo staged change" size="icon-xs" variant="outline" disabled={!canUndo} onClick={onUndo}><Undo2 /></IconButton>
            <IconButton label="Redo staged change" size="icon-xs" variant="outline" disabled={!canRedo} onClick={onRedo}><Redo2 /></IconButton>
            <Button size="xs" variant="ghost" disabled={!changes.length} onClick={onDiscard}><X />Discard all</Button>
            <Button size="xs" disabled={!changes.length || applying} onClick={() => setPreviewOpen(true)}>
              {applying ? 'Applying…' : `Review & apply ${changes.length}`}
            </Button>
          </div>
          {applyError ? <p role="alert" className="basis-full text-[length:var(--font-size-meta)] text-destructive">{applyError.message}</p> : null}
        </div>

        {expanded ? (
          <div className="grid max-h-52 grid-cols-[minmax(0,1fr)_minmax(13rem,0.34fr)] border-t border-border max-md:grid-cols-1">
            <div className="overflow-y-auto border-r border-border max-md:border-r-0 max-md:border-b">
              {changes.length ? changes.map((change) => {
                const changeErrors = change.kind === 'insert'
                  ? Array.from(validationErrors.entries()).filter(([key]) => key.startsWith(`${change.rowId}:`)).map(([, error]) => error)
                  : change.kind === 'update'
                    ? [validationErrors.get(`${change.rowId}:${change.column}`)].filter(Boolean) as string[]
                    : []
                return (
                  <div key={change.id} className="flex min-h-11 items-center gap-2 border-b border-border px-4 py-2 last:border-b-0 hover:bg-accent/25">
                    <ChangeIcon change={change} />
                    <div className="min-w-0 flex-1">
                      <p className="truncate font-mono text-[length:var(--font-size-data)] text-foreground">{changeTitle(change)}</p>
                      <p className="truncate text-[length:var(--font-size-meta)] text-muted-foreground">{changeErrors[0] ?? changeDescription(change)}</p>
                    </div>
                    {changeErrors.length ? <AlertTriangle className="size-3.5 shrink-0 text-destructive" /> : null}
                    <IconButton label={`Discard ${changeTitle(change)}`} size="icon-xs" onClick={() => onDiscardChange(change.id)}><X /></IconButton>
                  </div>
                )
              }) : <div className="grid min-h-24 place-items-center text-[length:var(--font-size-ui)] text-muted-foreground">No changes staged for {table}.</div>}
            </div>
            <div className="overflow-y-auto bg-background/40 p-3">
              <p className="text-[length:var(--font-size-meta)] font-semibold tracking-[0.08em] text-muted-foreground uppercase">Validation</p>
              {groupedErrors.length ? (
                <div className="mt-2 space-y-2">{groupedErrors.map((error) => <div key={error} className="flex gap-2 rounded-md border border-destructive/25 bg-destructive/8 p-2 text-[length:var(--font-size-meta)] text-destructive"><AlertTriangle className="mt-0.5 size-3.5 shrink-0" />{error}</div>)}</div>
              ) : <p className="mt-2 text-[length:var(--font-size-meta)] leading-5 text-muted-foreground">All staged values are valid and ready to apply.</p>}
            </div>
          </div>
        ) : null}
      </div>

      <Dialog open={previewOpen} onOpenChange={setPreviewOpen}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>Apply changes to {table}?</DialogTitle>
            <DialogDescription>Review the mutation batch before it is sent. Failed operations stay staged so you can correct and retry them.</DialogDescription>
          </DialogHeader>
          <div className="grid grid-cols-3 gap-2">
            <div className="rounded-lg border border-border bg-surface p-3"><FilePlus2 className="size-4 text-success-foreground" /><p className="mt-2 text-xl font-semibold text-foreground">{summary.inserts}</p><p className="text-[length:var(--font-size-meta)] text-muted-foreground">Rows inserted</p></div>
            <div className="rounded-lg border border-border bg-surface p-3"><PencilLine className="size-4 text-warning-foreground" /><p className="mt-2 text-xl font-semibold text-foreground">{summary.updates}</p><p className="text-[length:var(--font-size-meta)] text-muted-foreground">Rows updated</p></div>
            <div className="rounded-lg border border-border bg-surface p-3"><Trash2 className="size-4 text-destructive" /><p className="mt-2 text-xl font-semibold text-foreground">{summary.deletes}</p><p className="text-[length:var(--font-size-meta)] text-muted-foreground">Rows deleted</p></div>
          </div>
          {groupedErrors.length ? <div role="alert" className="flex gap-2 rounded-lg border border-destructive/30 bg-destructive/8 p-3 text-[length:var(--font-size-ui)] text-destructive"><AlertTriangle className="size-4 shrink-0" /><span>Resolve {validationErrors.size} invalid {validationErrors.size === 1 ? 'value' : 'values'} before applying this batch.</span></div> : null}
          <DialogFooter>
            <Button variant="outline" onClick={() => setPreviewOpen(false)}>Keep editing</Button>
            <Button disabled={Boolean(groupedErrors.length) || applying} onClick={() => { onApply(); if (!groupedErrors.length) setPreviewOpen(false) }}>{applying ? 'Applying…' : `Apply ${changes.length} changes`}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

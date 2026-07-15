import { useState } from 'react'
import {
  AlertCircle,
  Braces,
  Copy,
  Database,
  FileCode2,
  Gauge,
  LockKeyhole,
  Network,
  Rows3,
  ShieldCheck,
  Table2,
  X,
} from 'lucide-react'
import { writeClipboardText } from '@/shared/lib/clipboard'
import { cn } from '@/shared/lib/cn'
import {
  Badge,
  Button,
  IconButton,
  Separator,
} from '@/shared/ui'
import type { InspectorContext } from '@/widgets/app-shell/shell-types'

type InspectorPaneProps = {
  context: InspectorContext
  onClose?: () => void
  onOpenData?: () => void
  onNewQuery: () => void
  onOpenSchema?: () => void
  onGenerateSelect?: () => void
}

export function InspectorPane({ context, onClose, onOpenData, onNewQuery, onOpenSchema, onGenerateSelect }: InspectorPaneProps) {
  const object = context.object
  const title = object?.name || context.connectionName
  const subtitle = object ? `${object.kind} in ${object.schema || object.database || context.connectionName}` : context.engine
  const [copyError, setCopyError] = useState('')
  const hasQuickActions = Boolean(object && (onOpenSchema || onGenerateSelect))

  async function copyIdentifier() {
    try {
      await writeClipboardText(object?.qualifiedName || context.connectionName)
      setCopyError('')
    } catch (error) {
      setCopyError(error instanceof Error ? error.message : 'The identifier could not be copied.')
    }
  }

  return (
    <aside className="flex h-full min-h-0 flex-col bg-surface/90" aria-label="Context inspector">
      <header className="flex h-[var(--toolbar-height)] shrink-0 items-center border-b border-border px-4">
        <span className="text-[length:var(--font-size-ui)] font-semibold text-foreground">Inspector</span>
        <Badge variant={context.source === 'mock' ? 'accent' : 'success'} className="ml-2 text-[length:var(--font-size-meta)]">{context.source === 'mock' ? 'Demo' : 'Live'}</Badge>
        <div className="ml-auto flex items-center gap-0.5">
          <IconButton label={`Copy ${object ? 'qualified name' : 'connection name'}`} size="icon-xs" onClick={() => void copyIdentifier()}><Copy /></IconButton>
          {onClose ? (
            <IconButton label="Close inspector" size="icon-xs" onClick={onClose}>
              <X />
            </IconButton>
          ) : null}
        </div>
      </header>
      {copyError ? <div role="alert" className="flex shrink-0 items-center gap-2 border-b border-destructive/30 bg-destructive/10 px-4 py-2 text-[length:var(--font-size-meta)] text-destructive"><AlertCircle className="size-3.5 shrink-0" /><span className="min-w-0 flex-1">{copyError}</span><Button size="xs" variant="ghost" onClick={() => setCopyError('')}>Dismiss</Button></div> : null}

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-5">
        <div className="flex items-start gap-3">
          <div className="grid size-11 shrink-0 place-items-center rounded-xl border border-primary/20 bg-accent text-accent-foreground shadow-[0_9px_24px_color-mix(in_oklab,var(--primary)_12%,transparent)]">
            {object?.kind === 'table' ? <Table2 className="size-5" /> : <Database className="size-5" />}
          </div>
          <div className="min-w-0 pt-0.5">
            <h2 className="truncate text-base font-semibold tracking-[-0.015em] text-foreground">{title}</h2>
            <p className="mt-0.5 truncate text-[length:var(--font-size-meta)] text-muted-foreground">{subtitle}</p>
          </div>
        </div>

        <div className={cn('mt-5 grid gap-2', onOpenData ? 'grid-cols-2' : 'grid-cols-1')}>
          {onOpenData ? <Button size="sm" onClick={onOpenData}>
            <Table2 />
            Open data
          </Button> : null}
          <Button size="sm" variant="outline" onClick={onNewQuery}>
            <Braces />
            New query
          </Button>
        </div>

        <Separator className="my-5" />

        <section>
          <h3 className="text-[length:var(--font-size-meta)] font-semibold tracking-[0.08em] text-muted-foreground uppercase">Connection</h3>
          <dl className="mt-3 space-y-3 text-[length:var(--font-size-ui)]">
            <div className="flex items-center justify-between gap-4">
              <dt className="flex items-center gap-2 text-muted-foreground"><Network className="size-3.5" />Engine</dt>
              <dd className="font-medium text-foreground capitalize">{context.engine}</dd>
            </div>
            <div className="flex items-center justify-between gap-4">
              <dt className="flex items-center gap-2 text-muted-foreground"><Gauge className="size-3.5" />Status</dt>
              <dd className="flex items-center gap-1.5 font-medium text-foreground capitalize">
                <span className={cn('size-1.5 rounded-full bg-muted-foreground', context.status === 'connected' && 'bg-emerald-400', context.status === 'error' && 'bg-red-400')} />
                {context.status}
              </dd>
            </div>
            <div className="flex items-center justify-between gap-4">
              <dt className="flex items-center gap-2 text-muted-foreground"><ShieldCheck className="size-3.5" />Access</dt>
              <dd className="font-medium text-foreground">{context.readOnly ? 'Read only' : 'Read & write'}</dd>
            </div>
            <div className="flex items-center justify-between gap-4">
              <dt className="flex items-center gap-2 text-muted-foreground"><LockKeyhole className="size-3.5" />Transport</dt>
              <dd className="max-w-[150px] truncate font-medium text-foreground" title={context.transport}>{context.transport}</dd>
            </div>
          </dl>
        </section>

        {object ? (
          <>
            <Separator className="my-5" />
            <section>
              <h3 className="text-[length:var(--font-size-meta)] font-semibold tracking-[0.08em] text-muted-foreground uppercase">Object details</h3>
              <dl className="mt-3 space-y-3 text-[length:var(--font-size-ui)]">
                <div>
                  <dt className="text-[length:var(--font-size-meta)] text-muted-foreground">Qualified name</dt>
                  <dd className="mt-1 truncate rounded-md border border-border bg-background/60 px-2.5 py-2 font-mono text-[length:var(--font-size-data)] text-foreground">{object.qualifiedName}</dd>
                </div>
                <div className="flex items-center justify-between gap-4">
                  <dt className="text-muted-foreground">Object type</dt>
                  <dd><Badge variant="secondary" className="text-[length:var(--font-size-meta)] capitalize">{object.kind}</Badge></dd>
                </div>
                {object.count !== undefined ? (
                  <div className="flex items-center justify-between gap-4">
                    <dt className="text-muted-foreground">Objects</dt>
                    <dd className="font-mono text-foreground">{object.count}</dd>
                  </div>
                ) : null}
              </dl>
            </section>
          </>
        ) : null}

        {hasQuickActions ? <><Separator className="my-5" /><section className="rounded-xl border border-border bg-background/45 p-3.5"><div className="flex items-center gap-2 text-[length:var(--font-size-ui)] font-semibold text-foreground"><FileCode2 className="size-4 text-primary" />Object actions</div><div className="mt-3 space-y-1">{object && onOpenSchema ? <button type="button" onClick={onOpenSchema} className="flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-[length:var(--font-size-ui)] text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"><Rows3 className="size-3.5" />Open structure and DDL</button> : null}{object && onGenerateSelect ? <button type="button" onClick={onGenerateSelect} className="flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-[length:var(--font-size-ui)] text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"><FileCode2 className="size-3.5" />Generate SELECT statement</button> : null}</div></section></> : null}
      </div>
    </aside>
  )
}

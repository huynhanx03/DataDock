import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertCircle, Check, CheckCircle2, Clock3, Copy, Eraser, FileClock, Play, Search, Trash2 } from 'lucide-react'
import type { Connection } from '@/entities/connection'
import type { QueryHistoryItem, QueryStatus } from '@/entities/query'
import { useDataDockGateway } from '@/app/providers'
import { writeClipboardText } from '@/shared/lib/clipboard'
import { Badge, Button, Checkbox, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, EmptyState, Input, MasterDetailLayout, PropertyField, PropertyGrid, Select, WorkspaceHeader, WorkspacePage, WorkspaceToolbar } from '@/shared/ui'

type StatusFilter = 'all' | QueryStatus
type TimeFilter = 'all' | '24h' | '7d' | '30d'

function elapsedAt(value: string) {
  const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1_000))
  if (seconds < 60) return 'just now'
  if (seconds < 3_600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86_400) return `${Math.floor(seconds / 3_600)}h ago`
  if (seconds < 604_800) return `${Math.floor(seconds / 86_400)}d ago`
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}

function statementTitle(item: QueryHistoryItem) {
  return item.sqlText.replace(/\s+/g, ' ').trim()
}

function statusVariant(status: QueryStatus) {
  if (status === 'success') return 'success' as const
  if (status === 'running') return 'info' as const
  if (status === 'cancelled') return 'warning' as const
  return 'destructive' as const
}

export function QueryHistoryPage({ connection, onOpenQuery }: { connection?: Connection; onOpenQuery: (sql: string, connectionId?: string) => void }) {
  const gateway = useDataDockGateway()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState<StatusFilter>('all')
  const [connectionFilter, setConnectionFilter] = useState('all')
  const [timeFilter, setTimeFilter] = useState<TimeFilter>('7d')
  const [selectedId, setSelectedId] = useState('')
  const [checkedIds, setCheckedIds] = useState<Set<string>>(new Set())
  const [confirmMode, setConfirmMode] = useState<'selected' | 'all' | null>(null)
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState('')
  const history = useQuery({ queryKey: ['query-history', 'all'], queryFn: ({ signal }) => gateway.listQueryHistory(undefined, signal) })
  const connections = useQuery({ queryKey: ['connections'], queryFn: ({ signal }) => gateway.listConnections(undefined, signal) })
  const connectionNames = useMemo(() => new Map((connections.data ?? []).map((item) => [item.id, item.name])), [connections.data])
  const clearAll = useMutation({
    mutationFn: (connectionId?: string) => gateway.clearQueryHistory(connectionId),
    onSuccess: (_, connectionId) => {
      queryClient.setQueryData<QueryHistoryItem[]>(['query-history', 'all'], (current) => connectionId ? current?.filter((item) => item.connectionId !== connectionId) : [])
      setConfirmMode(null)
      setCheckedIds(new Set())
      setSelectedId('')
      queryClient.invalidateQueries({ queryKey: ['query-history'] })
    },
  })
  const deleteSelected = useMutation({
    mutationFn: async (ids: string[]) => {
      for (let offset = 0; offset < ids.length; offset += 100) await gateway.deleteQueryHistory(ids.slice(offset, offset + 100))
    },
    onSuccess: (_, ids) => {
      const deleted = new Set(ids)
      queryClient.setQueryData<QueryHistoryItem[]>(['query-history', 'all'], (current) => current?.filter((item) => !deleted.has(item.id)))
      setCheckedIds(new Set())
      setSelectedId('')
      setConfirmMode(null)
      queryClient.invalidateQueries({ queryKey: ['query-history'] })
    },
  })
  const records = useMemo(() => (history.data ?? []).filter((item) => {
    const needle = search.trim().toLowerCase()
    const matchesSearch = !needle || [statementTitle(item), item.error, connectionNames.get(item.connectionId)].join(' ').toLowerCase().includes(needle)
    const matchesStatus = status === 'all' || item.status === status
    const matchesConnection = connectionFilter === 'all' || item.connectionId === connectionFilter
    const hours = timeFilter === '24h' ? 24 : timeFilter === '7d' ? 168 : timeFilter === '30d' ? 720 : 0
    const matchesTime = !hours || new Date(item.executedAt).getTime() >= Date.now() - hours * 3_600_000
    return matchesSearch && matchesStatus && matchesConnection && matchesTime
  }), [connectionFilter, connectionNames, history.data, search, status, timeFilter])
  const selected = records.find((item) => item.id === selectedId) ?? records[0]

  useEffect(() => {
    if (selected && selected.id !== selectedId) setSelectedId(selected.id)
    if (!selected && selectedId) setSelectedId('')
  }, [selected, selectedId])

  function toggleChecked(id: string) {
    setCheckedIds((current) => {
      const next = new Set(current)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function confirmClear() {
    if (confirmMode === 'selected') {
      if (checkedIds.size) deleteSelected.mutate([...checkedIds])
      return
    }
    clearAll.mutate(connectionFilter === 'all' ? undefined : connectionFilter)
  }

  async function copySql(value: string) {
    try {
      await writeClipboardText(value)
      setCopyError('')
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1400)
    } catch (error) {
      setCopyError(error instanceof Error ? error.message : 'SQL could not be copied.')
    }
  }

  return <WorkspacePage>
    <WorkspaceHeader compact icon={<Clock3 />} eyebrow="Query activity" title="Query history" description="Persisted executions, timing and database outcomes" meta={<Badge variant={gateway.source === 'mock' ? 'accent' : 'success'}>{gateway.source === 'mock' ? 'Demo data' : 'Live data'}</Badge>} actions={<><Button size="sm" variant="outline" disabled={!checkedIds.size || deleteSelected.isPending} onClick={() => setConfirmMode('selected')}><Trash2 />Clear selected {checkedIds.size ? `(${checkedIds.size})` : ''}</Button><Button size="sm" variant="ghost" disabled={!history.data?.length || clearAll.isPending} onClick={() => setConfirmMode('all')}><Eraser />Clear history</Button></>} />
    <WorkspaceToolbar>
      <div className="relative min-w-56 flex-1 max-w-lg"><Search className="pointer-events-none absolute top-1/2 left-3 size-3.5 -translate-y-1/2 text-muted-foreground" /><Input value={search} onChange={(event) => setSearch(event.target.value)} className="h-[var(--control-height-sm)] pl-8" placeholder="Search SQL or errors…" /></div>
      <Select aria-label="Connection filter" value={connectionFilter} onChange={(event) => setConnectionFilter(event.target.value)} className="h-[var(--control-height-sm)] w-44"><option value="all">All connections</option>{(connections.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</Select>
      <Select aria-label="Status filter" value={status} onChange={(event) => setStatus(event.target.value as StatusFilter)} className="h-[var(--control-height-sm)] w-32"><option value="all">All statuses</option><option value="success">Successful</option><option value="error">Failed</option><option value="timeout">Timed out</option><option value="cancelled">Cancelled</option></Select>
      <Select aria-label="Time filter" value={timeFilter} onChange={(event) => setTimeFilter(event.target.value as TimeFilter)} className="h-[var(--control-height-sm)] w-32"><option value="24h">Last 24 hours</option><option value="7d">Last 7 days</option><option value="30d">Last 30 days</option><option value="all">All time</option></Select>
      <Badge variant="outline" className="ml-auto">{records.length} statements</Badge>
    </WorkspaceToolbar>
    {connections.isError ? <div role="alert" className="flex shrink-0 items-center gap-2 border-b border-warning/30 bg-warning/10 px-4 py-2 text-[length:var(--font-size-ui)] text-amber-300"><AlertCircle className="size-3.5 shrink-0" /><span className="min-w-0 flex-1">Connection names are temporarily unavailable. History records are still shown with their connection IDs.</span><Button size="xs" variant="outline" onClick={() => connections.refetch()}>Retry</Button></div> : null}
    {history.isError && history.data ? <div role="alert" className="flex shrink-0 items-center gap-2 border-b border-warning/30 bg-warning/10 px-4 py-2 text-[length:var(--font-size-ui)] text-amber-300"><AlertCircle className="size-3.5 shrink-0" /><span className="min-w-0 flex-1">The latest history refresh failed. Previously loaded records remain visible.</span><Button size="xs" variant="outline" onClick={() => history.refetch()}>Retry</Button></div> : null}
    <MasterDetailLayout className="grid-cols-[minmax(22rem,28rem)_minmax(0,1fr)] max-lg:grid-cols-[minmax(19rem,23rem)_minmax(0,1fr)]" master={<div className="flex h-full min-h-0 flex-col">
      <div className="flex h-9 shrink-0 items-center gap-2 border-b border-border px-3 text-[length:var(--font-size-meta)] text-muted-foreground"><Checkbox aria-label="Select all visible history records" checked={Boolean(records.length) && records.every((item) => checkedIds.has(item.id))} onChange={() => setCheckedIds(records.every((item) => checkedIds.has(item.id)) ? new Set() : new Set(records.map((item) => item.id)))} /><span>{checkedIds.size ? `${checkedIds.size} selected` : 'Select statements to clear'}</span></div>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {history.isLoading ? <EmptyState compact icon={<FileClock />} title="Loading history" description={gateway.source === 'mock' ? 'Reading demo executions…' : 'Reading persisted query executions…'} /> : null}
        {history.isError && !history.data ? <EmptyState compact icon={<AlertCircle />} title="History unavailable" description={history.error.message} actions={<Button size="sm" variant="outline" onClick={() => history.refetch()}>Retry</Button>} /> : null}
        {!history.isLoading && !history.isError && !records.length ? <EmptyState compact icon={<Clock3 />} title="No matching statements" description="Change the filters or run a query in SQL Studio." /> : null}
        <div className="divide-y divide-grid-line">{records.map((item) => <div key={item.id} className={`group flex cursor-pointer items-start gap-2.5 px-3 py-3 outline-none transition-colors hover:bg-accent/40 focus-within:bg-accent/40 ${selected?.id === item.id ? 'bg-accent/70' : ''}`} onClick={() => setSelectedId(item.id)}>
          <Checkbox aria-label={`Select ${statementTitle(item)}`} checked={checkedIds.has(item.id)} onClick={(event) => event.stopPropagation()} onChange={() => toggleChecked(item.id)} className="mt-0.5" />
          <button type="button" className="min-w-0 flex-1 text-left outline-none" onClick={() => setSelectedId(item.id)}><div className="flex items-center gap-2"><span className={`size-1.5 shrink-0 rounded-full ${item.status === 'success' ? 'bg-emerald-400' : item.status === 'cancelled' ? 'bg-amber-400' : 'bg-destructive'}`} /><p className="truncate font-mono text-[length:var(--font-size-data)] text-foreground">{statementTitle(item)}</p></div><div className="mt-1.5 flex items-center gap-1.5 text-[length:var(--font-size-meta)] text-muted-foreground"><span className="truncate">{connectionNames.get(item.connectionId) ?? item.connectionId}</span><span>·</span><span>{elapsedAt(item.executedAt)}</span><span>·</span><span className="font-mono">{item.durationMs} ms</span></div></button>
          <Badge variant={statusVariant(item.status)} className="h-5 px-1.5">{item.status}</Badge>
        </div>)}</div>
      </div>
    </div>} detail={<div className="h-full min-h-0 overflow-y-auto p-4 lg:p-5">{selected ? <div className="mx-auto flex min-h-full max-w-5xl flex-col">
      <div className="flex flex-wrap items-start gap-3"><div className={`grid size-9 place-items-center rounded-lg ${selected.status === 'success' ? 'bg-success/15 text-emerald-400' : selected.status === 'cancelled' ? 'bg-warning/15 text-amber-400' : 'bg-destructive/10 text-destructive'}`}>{selected.status === 'success' ? <CheckCircle2 className="size-4" /> : <AlertCircle className="size-4" />}</div><div className="min-w-0"><div className="flex flex-wrap items-center gap-2"><h2 className="text-base font-semibold">Execution details</h2><Badge variant={statusVariant(selected.status)}>{selected.status}</Badge></div><p className="mt-1 text-[length:var(--font-size-meta)] text-muted-foreground">{new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(selected.executedAt))}</p></div><div className="ml-auto flex gap-2"><Button size="sm" variant="outline" onClick={() => copySql(selected.sqlText)}>{copied ? <Check /> : <Copy />}{copied ? 'Copied' : 'Copy SQL'}</Button><Button size="sm" onClick={() => onOpenQuery(selected.sqlText, selected.connectionId)}><Play />Open in SQL</Button></div></div>
      <PropertyGrid className="mt-5"><PropertyField label="Connection"><div className="rounded-md border border-border bg-surface px-3 py-2 text-[length:var(--font-size-ui)]">{connectionNames.get(selected.connectionId) ?? selected.connectionId}{selected.connectionId === connection?.id ? <Badge variant="accent" className="ml-2">Current</Badge> : null}</div></PropertyField><PropertyField label="Duration"><div className="rounded-md border border-border bg-surface px-3 py-2 font-mono text-[length:var(--font-size-ui)]">{selected.durationMs} ms</div></PropertyField><PropertyField label="Rows returned"><div className="rounded-md border border-border bg-surface px-3 py-2 font-mono text-[length:var(--font-size-ui)]">{selected.rowCount.toLocaleString()}</div></PropertyField></PropertyGrid>
      {copyError ? <div role="alert" className="mt-4 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-[length:var(--font-size-ui)] text-destructive">{copyError}</div> : null}
      {selected.error ? <div role="alert" className="mt-4 rounded-lg border border-destructive/30 bg-destructive/10 p-3"><p className="text-[length:var(--font-size-meta)] font-semibold tracking-wide text-destructive uppercase">Database error</p><p className="mt-1 font-mono text-[length:var(--font-size-data)] text-destructive">{selected.error}</p></div> : null}
      <div className="mt-4 flex min-h-[15rem] flex-1 flex-col overflow-hidden rounded-xl border border-border bg-[#070a11]"><div className="flex h-9 shrink-0 items-center gap-2 border-b border-border px-3 text-[length:var(--font-size-meta)] text-muted-foreground"><FileClock className="size-3.5 text-primary" /><span>Statement preview</span><span className="ml-auto font-mono">{selected.sqlText.split('\n').length} lines</span></div><pre className="min-h-0 flex-1 overflow-auto p-4 font-mono text-[length:var(--font-size-data)] leading-6 text-foreground"><code>{selected.sqlText}</code></pre></div>
    </div> : <EmptyState icon={<Clock3 />} title="Select a statement" description="Review SQL, duration and result details without leaving history." />}</div>} />
    <Dialog open={confirmMode !== null} onOpenChange={(open) => { if (!open) { setConfirmMode(null); clearAll.reset(); deleteSelected.reset() } }}><DialogContent><DialogHeader><DialogTitle>{confirmMode === 'selected' ? `Clear ${checkedIds.size} selected statements?` : `Clear ${connectionFilter === 'all' ? 'all query history' : `history for ${connectionNames.get(connectionFilter) ?? connectionFilter}`}?`}</DialogTitle><DialogDescription>This removes the selected records from DataDock metadata. Database logs and server audit records are not affected.</DialogDescription></DialogHeader>{(confirmMode === 'selected' ? deleteSelected.error : clearAll.error) ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-ui)] text-destructive">{(confirmMode === 'selected' ? deleteSelected.error : clearAll.error)?.message}</div> : null}<DialogFooter><Button variant="outline" onClick={() => { setConfirmMode(null); clearAll.reset(); deleteSelected.reset() }}>Cancel</Button><Button variant="destructive" disabled={clearAll.isPending || deleteSelected.isPending} onClick={confirmClear}><Trash2 />{clearAll.isPending || deleteSelected.isPending ? 'Clearing…' : 'Clear history'}</Button></DialogFooter></DialogContent></Dialog>
  </WorkspacePage>
}

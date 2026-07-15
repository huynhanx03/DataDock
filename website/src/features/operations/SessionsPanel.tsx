import { useMemo, useState } from 'react'
import { Activity, CircleStop, Clock3, Database, Monitor, Power, Search, ServerCog, UserRound } from 'lucide-react'
import type { DatabaseSession, DatabaseSessions } from '@/entities/database-object'
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
  Select,
  Skeleton,
  WorkspacePanel,
} from '@/shared/ui'

type SessionsPanelProps = {
  sessions?: DatabaseSessions
  loading?: boolean
  source: 'mock' | 'api'
  onCancel: (sessionId: string) => Promise<void>
  onTerminate: (sessionId: string) => Promise<void>
}

function stateVariant(state: string): 'success' | 'warning' | 'outline' | 'destructive' {
  const normalized = state.toLowerCase()
  if (normalized === 'active' || normalized === 'query') return 'success'
  if (normalized.includes('transaction')) return 'warning'
  if (normalized === 'cancelled') return 'destructive'
  return 'outline'
}

function duration(value = 0) {
  if (value < 1000) return `${value} ms`
  if (value < 60_000) return `${(value / 1000).toFixed(1)} s`
  return `${Math.floor(value / 60_000)}m ${Math.floor(value % 60_000 / 1000)}s`
}

export function SessionsPanel({ sessions, loading, source, onCancel, onTerminate }: SessionsPanelProps) {
  const [search, setSearch] = useState('')
  const [state, setState] = useState('all')
  const [selectedId, setSelectedId] = useState('')
  const [action, setAction] = useState<{ kind: 'cancel' | 'terminate'; session: DatabaseSession }>()
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const visible = useMemo(() => {
    const query = search.trim().toLowerCase()
    return (sessions?.items ?? []).filter((session) => (state === 'all' || session.state.toLowerCase() === state) && (!query || `${session.id} ${session.user} ${session.database} ${session.client} ${session.query}`.toLowerCase().includes(query)))
  }, [search, sessions?.items, state])
  const states = useMemo(() => Array.from(new Set((sessions?.items ?? []).map((session) => session.state.toLowerCase()))), [sessions?.items])
  const selected = visible.find((session) => session.id === selectedId) ?? visible[0]

  async function confirmAction() {
    if (!action) return
    setPending(true)
    setError('')
    try {
      if (action.kind === 'cancel') await onCancel(action.session.id)
      else await onTerminate(action.session.id)
      setAction(undefined)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'The session action failed.')
    } finally {
      setPending(false)
    }
  }

  if (loading) return <div className="grid gap-4 xl:grid-cols-[minmax(0,1.5fr)_minmax(18rem,0.5fr)]"><Skeleton className="h-[34rem]" /><Skeleton className="h-[34rem]" /></div>
  if (!sessions?.available) return <EmptyState icon={<ServerCog />} title="Session inspection unavailable" description={sessions?.message ?? 'The current user cannot inspect database sessions.'} />

  return <div className="grid min-h-0 grid-cols-[minmax(0,1.52fr)_minmax(18rem,0.48fr)] gap-4 max-xl:grid-cols-1">
    <WorkspacePanel noPadding title="Database sessions" description={`${sessions.items.length} sessions · ${sessions.items.filter((session) => ['active', 'query'].includes(session.state.toLowerCase())).length} running`} actions={<Badge variant={source === 'mock' ? 'accent' : 'success'}>{source === 'mock' ? 'Sample sessions' : 'Live adapter'}</Badge>}>
      <div className="flex min-h-[var(--toolbar-height)] flex-wrap items-center gap-2 border-b border-border px-3 py-1.5"><div className="relative min-w-52 flex-1 max-w-sm"><Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" /><Input aria-label="Search database sessions" className="h-[var(--control-height-sm)] pl-8" value={search} placeholder="PID, user, client, or SQL" onChange={(event) => setSearch(event.target.value)} /></div><Select aria-label="Session state" className="h-[var(--control-height-sm)] w-44" value={state} onChange={(event) => setState(event.target.value)}><option value="all">All states</option>{states.map((item) => <option key={item} value={item}>{item}</option>)}</Select><Badge variant="outline">{visible.length} shown</Badge></div>
      {visible.length ? <div className="overflow-auto"><table className="w-full min-w-[880px] border-collapse text-[length:var(--font-size-data)]"><thead><tr className="bg-grid-header text-left text-muted-foreground"><th className="border-b border-grid-line px-3 py-2 font-medium">PID</th><th className="border-b border-grid-line px-3 py-2 font-medium">User</th><th className="border-b border-grid-line px-3 py-2 font-medium">State</th><th className="border-b border-grid-line px-3 py-2 font-medium">Duration</th><th className="border-b border-grid-line px-3 py-2 font-medium">Wait event</th><th className="border-b border-grid-line px-3 py-2 font-medium">Query</th></tr></thead><tbody>{visible.map((session) => <tr key={session.id} tabIndex={0} aria-selected={selected?.id === session.id} onClick={() => setSelectedId(session.id)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelectedId(session.id) } }} className="border-b border-grid-line outline-none hover:bg-accent/35 focus-visible:bg-accent/45 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary aria-selected:bg-accent/65"><td className="px-3 py-2.5 font-mono font-medium text-foreground">{session.id}</td><td className="px-3 py-2.5"><div><span className="text-foreground">{session.user}</span><span className="mt-0.5 block font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{session.database}</span></div></td><td className="px-3 py-2.5"><Badge variant={stateVariant(session.state)}>{session.state}</Badge></td><td className="px-3 py-2.5 font-mono tabular-nums text-muted-foreground">{duration(session.durationMs)}</td><td className="max-w-44 truncate px-3 py-2.5 font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{session.waitEvent || '—'}</td><td className="max-w-md truncate px-3 py-2.5 font-mono text-muted-foreground">{session.query || '—'}</td></tr>)}</tbody></table></div> : <EmptyState compact icon={<Search />} title="No sessions match" description="Change the state filter or search phrase." />}
    </WorkspacePanel>
    <WorkspacePanel title={selected ? `Session ${selected.id}` : 'Session details'} description={selected ? selected.user : 'Select a session'}>
      {selected ? <div className="space-y-4"><div className="flex items-center justify-between gap-2"><Badge variant={stateVariant(selected.state)}><Activity />{selected.state}</Badge><span className="font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{duration(selected.durationMs)}</span></div><dl className="grid gap-3 text-[length:var(--font-size-ui)]"><div className="flex items-start gap-3"><UserRound className="mt-0.5 size-4 text-muted-foreground" /><div><dt className="text-muted-foreground">Role</dt><dd className="font-mono text-foreground">{selected.user}</dd></div></div><div className="flex items-start gap-3"><Database className="mt-0.5 size-4 text-muted-foreground" /><div><dt className="text-muted-foreground">Database</dt><dd className="font-mono text-foreground">{selected.database}</dd></div></div><div className="flex items-start gap-3"><Monitor className="mt-0.5 size-4 text-muted-foreground" /><div><dt className="text-muted-foreground">Client</dt><dd className="font-mono text-foreground">{selected.client || 'Unknown'}</dd></div></div><div className="flex items-start gap-3"><Clock3 className="mt-0.5 size-4 text-muted-foreground" /><div><dt className="text-muted-foreground">Started</dt><dd className="text-foreground">{selected.startedAt ? new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(selected.startedAt)) : 'Not available'}</dd></div></div></dl><div><p className="text-[length:var(--font-size-meta)] font-medium text-muted-foreground">Current statement</p><pre className="mt-2 max-h-44 overflow-auto rounded-lg border border-border bg-background/70 p-3 whitespace-pre-wrap font-mono text-[length:var(--font-size-data)] leading-5 text-foreground">{selected.query || 'No active statement'}</pre></div><div className="grid gap-2"><Button size="sm" variant="outline" disabled={!selected.query || !['active', 'query'].includes(selected.state.toLowerCase())} onClick={() => setAction({ kind: 'cancel', session: selected })}><CircleStop />Cancel query</Button><Button size="sm" variant="ghost" className="text-destructive hover:text-destructive" onClick={() => setAction({ kind: 'terminate', session: selected })}><Power />Terminate session</Button></div></div> : <EmptyState compact title="No session selected" />}
    </WorkspacePanel>
    <Dialog open={Boolean(action)} onOpenChange={(open) => { if (!open && !pending) { setAction(undefined); setError('') } }}><DialogContent><DialogHeader><DialogTitle>{action?.kind === 'terminate' ? 'Terminate database session?' : 'Cancel running query?'}</DialogTitle><DialogDescription>{source === 'mock' ? `This simulates ${action?.kind === 'terminate' ? 'terminating the session' : 'cancelling the statement'} in the local sample data only.` : action?.kind === 'terminate' ? `Session ${action.session.id} will be disconnected and its open transaction will roll back.` : `The current statement on session ${action?.session.id} will be cancelled. The session remains connected.`}</DialogDescription></DialogHeader>{action?.session.query ? <pre className="max-h-32 overflow-auto rounded-lg border border-border bg-background/70 p-3 whitespace-pre-wrap font-mono text-[length:var(--font-size-data)] leading-5 text-foreground">{action.session.query}</pre> : null}{error ? <p role="alert" className="rounded-lg border border-destructive/25 bg-destructive/10 p-3 text-[length:var(--font-size-ui)] text-destructive">{error}</p> : null}<DialogFooter><Button variant="outline" disabled={pending} onClick={() => setAction(undefined)}>Go back</Button><Button variant={action?.kind === 'terminate' ? 'destructive' : 'default'} disabled={pending} onClick={confirmAction}>{action?.kind === 'terminate' ? <Power /> : <CircleStop />}{pending ? 'Working…' : source === 'mock' ? action?.kind === 'terminate' ? 'Simulate terminate' : 'Simulate cancel' : action?.kind === 'terminate' ? 'Terminate session' : 'Cancel query'}</Button></DialogFooter></DialogContent></Dialog>
  </div>
}

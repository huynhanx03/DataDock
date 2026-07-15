import { useMemo, useState } from 'react'
import { AlertCircle, CircleDot, Database, Gauge, LockKeyhole, Search, Star } from 'lucide-react'
import type { Connection, ConnectionStatus, DatabaseEngine } from '@/entities/connection'
import { Badge, IconButton, Input, Select } from '@/shared/ui'

type ConnectionListProps = {
  connections: Connection[]
  selectedId?: string
  disabled?: boolean
  onSelect: (connection: Connection) => void
  onFavorite: (connection: Connection) => void
}

const engineLabels: Partial<Record<DatabaseEngine, string>> = {
  postgresql: 'PostgreSQL',
  mysql: 'MySQL',
  mariadb: 'MariaDB',
}

const engineMarks: Partial<Record<DatabaseEngine, string>> = {
  postgresql: 'PG',
  mysql: 'MY',
  mariadb: 'MA',
}

const statusLabels: Record<ConnectionStatus, string> = {
  connected: 'Connected',
  connecting: 'Connecting',
  disconnected: 'Offline',
  error: 'Error',
}

function StatusDot({ status }: { status: ConnectionStatus }) {
  if (status === 'error') return <AlertCircle className="size-3.5 text-destructive" />
  return <span className={`size-2 rounded-full ${status === 'connected' ? 'bg-success-foreground' : status === 'connecting' ? 'animate-pulse bg-warning-foreground' : 'bg-muted-foreground/55'}`} />
}

function ConnectionRow({ connection, selected, disabled, onSelect, onFavorite }: { connection: Connection; selected: boolean; disabled?: boolean; onSelect: () => void; onFavorite: () => void }) {
  return (
    <div className={`group flex items-stretch rounded-lg border transition-[background-color,border-color,box-shadow] ${disabled ? 'opacity-65' : ''} ${selected ? 'border-primary/50 bg-accent/70 shadow-control' : 'border-transparent hover:border-border hover:bg-accent/30'}`}>
      <button type="button" aria-pressed={selected} disabled={disabled} onClick={onSelect} className="flex min-w-0 flex-1 items-start gap-3 rounded-l-lg px-2.5 py-2.5 text-left outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/30 disabled:cursor-wait">
        <span className={`mt-0.5 grid size-9 shrink-0 place-items-center rounded-lg border font-mono text-[11px] font-bold ${selected ? 'border-primary/30 bg-primary text-primary-foreground' : 'border-border bg-muted text-muted-foreground'}`}>{engineMarks[connection.engine] ?? 'DB'}</span>
        <span className="min-w-0 flex-1">
          <span className="flex items-center gap-2"><span className="min-w-0 flex-1 truncate text-[length:var(--font-size-ui)] font-semibold text-foreground">{connection.name}</span><span className="flex shrink-0 items-center gap-1 text-[length:var(--font-size-meta)] font-medium text-muted-foreground"><StatusDot status={connection.status} />{statusLabels[connection.status]}</span></span>
          <span className="mt-0.5 block truncate font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{connection.host}:{connection.port}/{connection.database}</span>
          <span className="mt-2 flex min-w-0 items-center gap-1.5">
            <Badge variant="outline" className="max-w-24 truncate">{engineLabels[connection.engine] ?? connection.engine}</Badge>
            {connection.latencyMs !== undefined ? <Badge variant={connection.status === 'error' ? 'warning' : 'outline'}><Gauge />{connection.latencyMs} ms</Badge> : null}
            {connection.readOnly ? <Badge variant="info"><LockKeyhole />Read only</Badge> : null}
          </span>
        </span>
      </button>
      <div className="flex shrink-0 items-start pr-1.5 pt-1.5">
        <IconButton label={connection.favorite ? `Remove ${connection.name} from favorites` : `Add ${connection.name} to favorites`} size="icon-xs" variant="ghost" disabled={disabled} onClick={onFavorite} className={connection.favorite ? 'text-warning-foreground opacity-100' : 'opacity-0 group-hover:opacity-100 group-focus-within:opacity-100'}><Star className={connection.favorite ? 'fill-current' : ''} /></IconButton>
      </div>
    </div>
  )
}

function ConnectionGroup({ title, count, connections, selectedId, disabled, onSelect, onFavorite }: { title: string; count: number; connections: Connection[]; selectedId?: string; disabled?: boolean; onSelect: (connection: Connection) => void; onFavorite: (connection: Connection) => void }) {
  if (!connections.length) return null
  return (
    <section aria-label={title} className="grid gap-1.5">
      <div className="flex items-center justify-between px-1.5"><h3 className="text-[length:var(--font-size-meta)] font-semibold tracking-[0.08em] text-muted-foreground uppercase">{title}</h3><span className="font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{count}</span></div>
      <div className="grid gap-1">{connections.map((connection) => <ConnectionRow key={connection.id} connection={connection} selected={selectedId === connection.id} disabled={disabled} onSelect={() => onSelect(connection)} onFavorite={() => onFavorite(connection)} />)}</div>
    </section>
  )
}

export function ConnectionList({ connections, selectedId, disabled, onSelect, onFavorite }: ConnectionListProps) {
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState<'all' | ConnectionStatus>('all')
  const [engine, setEngine] = useState<'all' | DatabaseEngine>('all')
  const [readonlyOnly, setReadonlyOnly] = useState(false)
  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase()
    return connections.filter((connection) => {
      const matchesSearch = !needle || [connection.name, connection.host, connection.database, connection.username, connection.engine].join(' ').toLowerCase().includes(needle)
      return matchesSearch && (status === 'all' || connection.status === status) && (engine === 'all' || connection.engine === engine) && (!readonlyOnly || connection.readOnly)
    })
  }, [connections, engine, readonlyOnly, search, status])
  const favorites = filtered.filter((connection) => connection.favorite)
  const regular = filtered.filter((connection) => !connection.favorite)

  return (
    <div className="flex h-full min-h-0 flex-col bg-surface/35">
      <div className="border-b border-border p-3">
        <div className="relative"><Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" /><Input aria-label="Search connections" value={search} onChange={(event) => setSearch(event.target.value)} className="pl-9" placeholder="Search connections…" /></div>
        <div className="mt-2 grid grid-cols-2 gap-2">
          <label className="sr-only" htmlFor="connection-status-filter">Filter by status</label><Select id="connection-status-filter" value={status} onChange={(event) => setStatus(event.target.value as 'all' | ConnectionStatus)} className="h-[var(--control-height-sm)] text-[length:var(--font-size-meta)]"><option value="all">All statuses</option><option value="connected">Connected</option><option value="disconnected">Offline</option><option value="connecting">Connecting</option><option value="error">Error</option></Select>
          <label className="sr-only" htmlFor="connection-engine-filter">Filter by engine</label><Select id="connection-engine-filter" value={engine} onChange={(event) => setEngine(event.target.value as 'all' | DatabaseEngine)} className="h-[var(--control-height-sm)] text-[length:var(--font-size-meta)]"><option value="all">All engines</option><option value="postgresql">PostgreSQL</option><option value="mysql">MySQL</option><option value="mariadb">MariaDB</option></Select>
        </div>
        <div className="mt-2 flex items-center justify-between gap-2 px-0.5 text-[length:var(--font-size-meta)] text-muted-foreground"><button type="button" aria-pressed={readonlyOnly} onClick={() => setReadonlyOnly((value) => !value)} className={`flex h-6 items-center gap-1 rounded-md border px-1.5 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/25 ${readonlyOnly ? 'border-info/40 bg-info text-info-foreground' : 'border-border bg-surface hover:bg-accent/45'}`}><LockKeyhole className="size-3" />Readonly only</button><span className="ml-auto">{filtered.length} of {connections.length}</span><span className="flex items-center gap-1"><CircleDot className="size-3" />{connections.filter((connection) => connection.status === 'connected').length} active</span></div>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-2.5">
        {filtered.length ? <div className="grid gap-4"><ConnectionGroup title="Favorites" count={favorites.length} connections={favorites} selectedId={selectedId} disabled={disabled} onSelect={onSelect} onFavorite={onFavorite} /><ConnectionGroup title="Connections" count={regular.length} connections={regular} selectedId={selectedId} disabled={disabled} onSelect={onSelect} onFavorite={onFavorite} /></div> : <div className="grid min-h-48 place-items-center px-5 text-center"><div><span className="mx-auto grid size-10 place-items-center rounded-xl border border-border bg-surface text-muted-foreground"><Database className="size-4" /></span><p className="mt-3 text-[length:var(--font-size-ui)] font-semibold">{connections.length ? 'No matching connections' : 'No connection profiles yet'}</p><p className="mt-1 text-[length:var(--font-size-meta)] leading-5 text-muted-foreground">{connections.length ? 'Change the search or filters to see more profiles.' : 'Use New connection to add the first profile in this workspace.'}</p></div></div>}
      </div>
    </div>
  )
}

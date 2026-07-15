import { useMemo, useState } from 'react'
import { AlertCircle, ArrowDown, ArrowUp, BarChart3, Braces, Check, CircleGauge, Clipboard, Clock3, Rows3, Search, TimerReset, Zap } from 'lucide-react'
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { DatabaseMetric, DatabasePerformance, SlowQuery } from '@/entities/database-object'
import { writeClipboardText } from '@/shared/lib/clipboard'
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

type SortKey = 'totalMs' | 'meanMs' | 'calls' | 'rows'

type SlowQueriesPanelProps = {
  performance?: DatabasePerformance
  loading?: boolean
  source: 'mock' | 'api'
  onOpenQuery?: (sql: string) => void
}

function shortSql(sql: string, max = 94) {
  const compact = sql.replace(/\s+/g, ' ').trim()
  return compact.length > max ? `${compact.slice(0, max)}…` : compact
}

function queryId(query: SlowQuery) {
  return query.fingerprint?.trim() || `${query.calls}:${query.totalMs}:${query.meanMs}:${query.rows}:${query.query}`
}

function ExplainDialog({ query, source, open, onOpenChange, onOpenQuery }: { query?: SlowQuery; source: 'mock' | 'api'; open: boolean; onOpenChange: (open: boolean) => void; onOpenQuery?: (sql: string) => void }) {
  if (!query) return null
  const explainSql = `EXPLAIN (FORMAT JSON)\n${query.query.trim().replace(/;$/, '')};`
  return <Dialog open={open} onOpenChange={onOpenChange}><DialogContent className="max-w-2xl"><DialogHeader><DialogTitle>Prepare a real execution plan</DialogTitle><DialogDescription>{source === 'mock' ? 'This is a sample statement fingerprint. No database execution plan exists for the demo feed.' : 'Statement statistics contain aggregate timings, not plan nodes. Open a non-executing EXPLAIN in SQL Studio to request the real plan from the database.'}</DialogDescription></DialogHeader><pre className="max-h-40 overflow-auto rounded-lg border border-border bg-background/70 p-3 whitespace-pre-wrap font-mono text-[length:var(--font-size-data)] leading-5 text-foreground">{query.query}</pre><div className="grid gap-2 sm:grid-cols-4"><div className="rounded-lg border border-border bg-background/60 p-3"><p className="text-[length:var(--font-size-meta)] text-muted-foreground">Calls</p><p className="mt-1 font-mono font-semibold tabular-nums">{query.calls.toLocaleString()}</p></div><div className="rounded-lg border border-border bg-background/60 p-3"><p className="text-[length:var(--font-size-meta)] text-muted-foreground">Total time</p><p className="mt-1 font-mono font-semibold tabular-nums">{query.totalMs.toFixed(2)} ms</p></div><div className="rounded-lg border border-border bg-background/60 p-3"><p className="text-[length:var(--font-size-meta)] text-muted-foreground">Mean time</p><p className="mt-1 font-mono font-semibold tabular-nums">{query.meanMs.toFixed(2)} ms</p></div><div className="rounded-lg border border-border bg-background/60 p-3"><p className="text-[length:var(--font-size-meta)] text-muted-foreground">Rows</p><p className="mt-1 font-mono font-semibold tabular-nums">{query.rows.toLocaleString()}</p></div></div><p className="rounded-lg border border-info/25 bg-info/45 p-3 text-[length:var(--font-size-ui)] leading-5 text-info-foreground">EXPLAIN does not execute the statement. Replace normalized placeholders such as $1 or ? with representative values before running it.</p><DialogFooter><Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>{onOpenQuery ? <Button onClick={() => { onOpenQuery(explainSql); onOpenChange(false) }}><Braces />Open EXPLAIN in SQL Studio</Button> : null}</DialogFooter></DialogContent></Dialog>
}

export function SlowQueriesPanel({ performance, loading, source, onOpenQuery }: SlowQueriesPanelProps) {
  const [search, setSearch] = useState('')
  const [sort, setSort] = useState<SortKey>('totalMs')
  const [direction, setDirection] = useState<'asc' | 'desc'>('desc')
  const [selectedId, setSelectedId] = useState('')
  const [explain, setExplain] = useState<SlowQuery>()
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState('')
  const visible = useMemo(() => {
    const query = search.trim().toLowerCase()
    return [...(performance?.slowQueries ?? [])].filter((item) => !query || item.query.toLowerCase().includes(query)).sort((left, right) => (left[sort] - right[sort]) * (direction === 'asc' ? 1 : -1))
  }, [direction, performance?.slowQueries, search, sort])
  const selected = visible.find((query) => queryId(query) === selectedId) ?? visible[0]

  async function copyQuery(query: string) {
    try {
      await writeClipboardText(query)
      setCopyError('')
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch (error) {
      setCopied(false)
      setCopyError(error instanceof Error ? error.message : 'The SQL statement could not be copied.')
    }
  }

  if (loading) return <div className="grid gap-4 xl:grid-cols-[minmax(0,1.52fr)_minmax(18rem,0.48fr)]"><Skeleton className="h-[34rem]" /><Skeleton className="h-[34rem]" /></div>
  if (!performance?.available) return <EmptyState icon={<TimerReset />} title="Statement statistics unavailable" description={performance?.message ?? 'Enable the database statement statistics extension to inspect slow queries.'} />

  return <div className="grid gap-4 xl:grid-cols-[minmax(0,1.52fr)_minmax(18rem,0.48fr)]">
    <WorkspacePanel noPadding title="Slow query fingerprints" description={`${performance.slowQueries.length} statements ordered by ${sort}`} actions={<Badge variant={source === 'mock' ? 'accent' : 'success'}>{source === 'mock' ? 'Sample statistics' : 'Live adapter'}</Badge>}><div className="flex min-h-[var(--toolbar-height)] flex-wrap items-center gap-2 border-b border-border px-3 py-1.5"><div className="relative min-w-52 max-w-md flex-1"><Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" /><Input aria-label="Search slow query SQL" className="h-[var(--control-height-sm)] pl-8" value={search} placeholder="Search statement SQL" onChange={(event) => setSearch(event.target.value)} /></div><Select aria-label="Slow query sort" className="h-[var(--control-height-sm)] w-40" value={sort} onChange={(event) => setSort(event.target.value as SortKey)}><option value="totalMs">Total time</option><option value="meanMs">Mean time</option><option value="calls">Calls</option><option value="rows">Rows</option></Select><Button size="icon-xs" variant="outline" aria-label={`Sort ${direction === 'desc' ? 'ascending' : 'descending'}`} onClick={() => setDirection((value) => value === 'desc' ? 'asc' : 'desc')}>{direction === 'desc' ? <ArrowDown /> : <ArrowUp />}</Button></div>{visible.length ? <div className="overflow-auto"><table className="w-full min-w-[980px] border-collapse text-[length:var(--font-size-data)]"><thead><tr className="bg-grid-header text-left text-muted-foreground"><th className="border-b border-grid-line px-3 py-2 font-medium">Statement</th><th className="border-b border-grid-line px-3 py-2 text-right font-medium">Calls</th><th className="border-b border-grid-line px-3 py-2 text-right font-medium">Total</th><th className="border-b border-grid-line px-3 py-2 text-right font-medium">Mean</th><th className="border-b border-grid-line px-3 py-2 text-right font-medium">Rows</th><th className="border-b border-grid-line px-3 py-2"><span className="sr-only">Actions</span></th></tr></thead><tbody>{visible.map((query) => <tr key={queryId(query)} tabIndex={0} aria-selected={selected && queryId(selected) === queryId(query)} onClick={() => setSelectedId(queryId(query))} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelectedId(queryId(query)) } }} className="border-b border-grid-line outline-none hover:bg-accent/35 focus-visible:bg-accent/45 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary aria-selected:bg-accent/65"><td className="max-w-xl px-3 py-2.5"><span className="block truncate font-mono text-foreground">{shortSql(query.query, 112)}</span></td><td className="px-3 py-2.5 text-right font-mono tabular-nums text-muted-foreground">{query.calls.toLocaleString()}</td><td className="px-3 py-2.5 text-right font-mono tabular-nums text-foreground">{(query.totalMs / 1000).toFixed(2)} s</td><td className="px-3 py-2.5 text-right font-mono tabular-nums text-warning-foreground">{query.meanMs.toFixed(2)} ms</td><td className="px-3 py-2.5 text-right font-mono tabular-nums text-muted-foreground">{query.rows.toLocaleString()}</td><td className="px-3 py-2 text-right"><Button size="xs" variant="outline" onClick={(event) => { event.stopPropagation(); setExplain(query) }}><Braces />Prepare EXPLAIN</Button></td></tr>)}</tbody></table></div> : <EmptyState compact icon={<Search />} title={performance.slowQueries.length ? 'No statements match' : 'No statement statistics collected'} description={performance.slowQueries.length ? 'Change the search phrase to see query fingerprints.' : 'The adapter is available but returned no statement fingerprints for this collection.'} />}</WorkspacePanel>
    <WorkspacePanel title="Statement details" description={selected ? `${selected.calls.toLocaleString()} executions` : 'Select a statement'}>{selected ? <div className="space-y-4"><div className="grid grid-cols-2 gap-2"><div className="rounded-lg border border-border bg-background/60 p-3"><Clock3 className="size-4 text-primary" /><p className="mt-2 font-mono text-lg font-semibold tabular-nums">{selected.meanMs.toFixed(2)} ms</p><p className="text-[length:var(--font-size-meta)] text-muted-foreground">Mean time</p></div><div className="rounded-lg border border-border bg-background/60 p-3"><Rows3 className="size-4 text-primary" /><p className="mt-2 font-mono text-lg font-semibold tabular-nums">{selected.rows.toLocaleString()}</p><p className="text-[length:var(--font-size-meta)] text-muted-foreground">Rows returned</p></div></div><div><p className="text-[length:var(--font-size-meta)] font-medium text-muted-foreground">Normalized statement</p><pre className="mt-2 max-h-64 overflow-auto rounded-lg border border-border bg-background/70 p-3 whitespace-pre-wrap font-mono text-[length:var(--font-size-data)] leading-5 text-foreground">{selected.query}</pre></div><div className={`grid gap-2 ${onOpenQuery ? 'grid-cols-3' : 'grid-cols-2'}`}><Button size="xs" variant="outline" onClick={() => copyQuery(selected.query)}>{copied ? <Check /> : <Clipboard />}{copied ? 'Copied' : 'Copy SQL'}</Button><Button size="xs" onClick={() => setExplain(selected)}><Braces />Prepare</Button>{onOpenQuery ? <Button size="xs" variant="outline" onClick={() => onOpenQuery(selected.query)}><Braces />Open</Button> : null}</div>{copyError ? <div role="alert" className="flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-meta)] text-destructive"><AlertCircle className="size-3.5 shrink-0" /><span className="min-w-0 flex-1">{copyError}</span><Button size="xs" variant="ghost" onClick={() => setCopyError('')}>Dismiss</Button></div> : null}</div> : <EmptyState compact title="No query selected" />}</WorkspacePanel>
    <ExplainDialog query={explain} source={source} open={Boolean(explain)} onOpenChange={(open) => { if (!open) setExplain(undefined) }} onOpenQuery={onOpenQuery} />
  </div>
}

function formattedPerformanceMetric(metric: DatabaseMetric) {
  if (typeof metric.value !== 'number') return `${metric.value}${metric.unit ? ` ${metric.unit}` : ''}`
  if (metric.unit === 'bytes') {
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    const index = metric.value > 0 ? Math.min(Math.floor(Math.log(metric.value) / Math.log(1024)), units.length - 1) : 0
    return `${(metric.value / 1024 ** index).toFixed(index ? 1 : 0)} ${units[index]}`
  }
  const value = metric.value.toLocaleString(undefined, { maximumFractionDigits: 2 })
  return `${value}${metric.unit === 'percent' ? '%' : metric.unit ? ` ${metric.unit}` : ''}`
}

export function PerformancePanel({ performance, loading, source }: SlowQueriesPanelProps) {
  if (loading) return <div className="grid gap-3 md:grid-cols-3">{Array.from({ length: 3 }, (_, index) => <Skeleton key={index} className="h-40" />)}</div>
  if (!performance?.available) return <EmptyState icon={<CircleGauge />} title="Performance data unavailable" description={performance?.message ?? 'Statement performance statistics are not enabled.'} />
  const metrics = performance.metrics ?? []
  const totalTime = performance.slowQueries.reduce((sum, query) => sum + query.totalMs, 0)
  const totalCalls = performance.slowQueries.reduce((sum, query) => sum + query.calls, 0)
  const totalRows = performance.slowQueries.reduce((sum, query) => sum + query.rows, 0)
  const chart = performance.slowQueries.map((query, index) => ({ name: `Q${index + 1}`, total: Number((query.totalMs / 1000).toFixed(2)), mean: Number(query.meanMs.toFixed(2)), query: shortSql(query.query, 64) }))
  const maxMean = Math.max(...chart.map((query) => query.mean), 1)

  return <div className="space-y-4">
    {metrics.length ? <WorkspacePanel title="Database counters" description={performance.collectedAt ? `Collected ${new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(performance.collectedAt))}` : 'Current adapter snapshot'} actions={<Badge variant={source === 'mock' ? 'accent' : 'success'}>{source === 'mock' ? 'Sample metrics' : 'Live snapshot'}</Badge>}><div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">{metrics.slice(0, 4).map((metric) => <div key={metric.key} className="rounded-lg border border-border bg-background/60 p-3"><p className="text-[length:var(--font-size-meta)] text-muted-foreground">{metric.label}</p><p className="mt-1 font-mono text-lg font-semibold tabular-nums text-foreground">{formattedPerformanceMetric(metric)}</p></div>)}</div></WorkspacePanel> : null}
    {performance.slowQueries.length ? <><div className="grid gap-3 md:grid-cols-3"><div className="rounded-xl border border-border bg-surface p-4"><span className="grid size-9 place-items-center rounded-lg bg-accent text-primary"><TimerReset className="size-4" /></span><p className="mt-4 text-2xl font-semibold tracking-[-0.04em] tabular-nums">{(totalTime / 1000).toFixed(1)} s</p><p className="text-[length:var(--font-size-ui)] font-medium">Cumulative time</p><p className="mt-1 text-[length:var(--font-size-meta)] text-muted-foreground">Across returned statement fingerprints</p></div><div className="rounded-xl border border-border bg-surface p-4"><span className="grid size-9 place-items-center rounded-lg bg-info text-info-foreground"><Zap className="size-4" /></span><p className="mt-4 text-2xl font-semibold tracking-[-0.04em] tabular-nums">{totalCalls.toLocaleString()}</p><p className="text-[length:var(--font-size-ui)] font-medium">Executions</p><p className="mt-1 text-[length:var(--font-size-meta)] text-muted-foreground">{source === 'mock' ? 'Total sample calls' : 'Total tracked calls'}</p></div><div className="rounded-xl border border-border bg-surface p-4"><span className="grid size-9 place-items-center rounded-lg bg-success text-success-foreground"><Rows3 className="size-4" /></span><p className="mt-4 text-2xl font-semibold tracking-[-0.04em] tabular-nums">{totalRows.toLocaleString()}</p><p className="text-[length:var(--font-size-ui)] font-medium">Rows processed</p><p className="mt-1 text-[length:var(--font-size-meta)] text-muted-foreground">Returned or affected</p></div></div><div className="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(20rem,0.65fr)]"><WorkspacePanel title="Time contribution by statement" description="Cumulative execution time in seconds" actions={<Badge variant={source === 'mock' ? 'accent' : 'success'}>{source === 'mock' ? 'Sample statistics' : 'Live adapter'}</Badge>}><div className="h-72" role="img" aria-label="Total execution time per query"><ResponsiveContainer width="100%" height="100%"><BarChart data={chart} margin={{ top: 12, right: 10, left: -8, bottom: 4 }}><CartesianGrid stroke="var(--grid-line)" strokeDasharray="3 4" vertical={false} /><XAxis dataKey="name" tick={{ fill: 'var(--muted-foreground)', fontSize: 11 }} tickLine={false} axisLine={false} /><YAxis tick={{ fill: 'var(--muted-foreground)', fontSize: 11 }} tickLine={false} axisLine={false} /><Tooltip contentStyle={{ background: 'var(--popover)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12 }} formatter={(value) => [`${value} s`, 'Total time']} labelFormatter={(label) => `${label} · ${chart.find((item) => item.name === label)?.query}`} /><Bar dataKey="total" fill="var(--primary)" radius={[5, 5, 0, 0]} isAnimationActive={false} /></BarChart></ResponsiveContainer></div></WorkspacePanel><WorkspacePanel title="Mean latency ranking" description="Average duration per execution"><div className="space-y-3">{[...chart].sort((left, right) => right.mean - left.mean).map((item) => <div key={item.name}><div className="flex items-center justify-between gap-2 text-[length:var(--font-size-meta)]"><span className="truncate font-mono text-foreground">{item.name} · {item.query}</span><span className="shrink-0 font-mono tabular-nums text-warning-foreground">{item.mean} ms</span></div><div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-warning-foreground" style={{ width: `${Math.max(5, item.mean / maxMean * 100)}%` }} /></div></div>)}</div></WorkspacePanel></div></> : <WorkspacePanel><EmptyState compact icon={<BarChart3 />} title="No statement performance collected" description="The performance adapter is available but returned no statement fingerprints for this connection." /></WorkspacePanel>}
  </div>
}

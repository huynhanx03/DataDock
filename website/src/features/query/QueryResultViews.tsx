import { lazy, Suspense, useMemo, useState } from 'react'
import { AlertCircle, BarChart3, Check, CheckCircle2, ChevronRight, CodeXml, Copy, Download, FileJson2, GitBranch, RefreshCw, Table2 } from 'lucide-react'
import type { DataColumn } from '@/entities/database-object'
import type { QueryPlanNode, QueryResult, QueryResultMode } from '@/entities/query'
import { QueryResultGrid } from '@/features/query/QueryResultGrid'
import { writeClipboardText } from '@/shared/lib/clipboard'
import { Badge, Button, EmptyState, IconButton, Tabs, TabsList, TabsTrigger, Tooltip, TooltipContent, TooltipTrigger } from '@/shared/ui'

const LazyChart = lazy(async () => {
  const chart = await import('recharts')
  return {
    default: ({ result }: { result: QueryResult }) => {
      const numeric = result.columns.find((column) => isNumericColumn(column, result))
      const category = result.columns.find((column) => column !== numeric)
      if (!numeric) return <EmptyState compact icon={<BarChart3 />} title="No numeric series" description="Chart view needs at least one numeric result column." />
      const numericKey = numeric.key ?? numeric.name
      const categoryKey = category?.key ?? category?.name ?? numericKey
      const rows = result.rows.slice(0, 16).map((row, index) => ({ ...row, [numericKey]: chartNumber(row[numericKey]), __label: String(row[categoryKey] ?? `Row ${index + 1}`) }))
      return <div className="flex h-full min-h-[17rem] flex-col p-4">
        <div className="mb-3 flex items-center justify-between"><div><p className="text-[length:var(--font-size-ui)] font-semibold">{numeric.name} by {category?.name ?? 'row'}</p><p className="text-[length:var(--font-size-meta)] text-muted-foreground">First {rows.length} rows · hover bars for exact values</p></div><Badge variant="outline">Bar chart</Badge></div>
        <div className="min-h-0 flex-1" role="img" aria-label={`Bar chart of ${numeric.name} by ${category?.name ?? 'row'}`}>
          <chart.ResponsiveContainer width="100%" height="100%">
            <chart.BarChart data={rows} margin={{ top: 8, right: 12, bottom: 10, left: 4 }}>
              <chart.CartesianGrid stroke="var(--border)" strokeDasharray="3 4" vertical={false} />
              <chart.XAxis dataKey="__label" stroke="var(--muted-foreground)" tick={{ fontSize: 10 }} tickLine={false} axisLine={false} minTickGap={18} />
              <chart.YAxis stroke="var(--muted-foreground)" tick={{ fontSize: 10 }} tickLine={false} axisLine={false} width={45} />
              <chart.Tooltip cursor={{ fill: 'color-mix(in oklab, var(--accent) 40%, transparent)' }} contentStyle={{ background: 'var(--popover)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 11 }} />
              <chart.Bar dataKey={numericKey} fill="var(--primary)" radius={[4, 4, 0, 0]} maxBarSize={42} isAnimationActive={false} />
            </chart.BarChart>
          </chart.ResponsiveContainer>
        </div>
      </div>
    },
  }
})

const NUMERIC_LOGICAL_TYPES = new Set<DataColumn['logicalType']>(['integer', 'bigint', 'decimal', 'float'])

function chartNumber(value: unknown) {
  if (typeof value === 'number') return Number.isFinite(value) ? value : 0
  if (typeof value !== 'string' || !value.trim()) return 0
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function isNumericColumn(column: DataColumn, result: QueryResult) {
  const key = column.key ?? column.name
  if (result.rows.some((row) => typeof row[key] === 'number' && Number.isFinite(row[key]))) return true
  if (!NUMERIC_LOGICAL_TYPES.has(column.logicalType)) return false
  return result.rows.some((row) => {
    const value = row[key]
    return typeof value === 'string' && value.trim() !== '' && Number.isFinite(Number(value))
  })
}

type QueryResultViewsProps = {
  connectionId: string
  result: QueryResult
  mode: QueryResultMode
  refreshing: boolean
  canRefresh?: boolean
  onModeChange: (mode: QueryResultMode) => void
  onRefresh: () => void
}

function jsonText(result: QueryResult) {
  return JSON.stringify(result.rows, null, 2)
}

function csvText(result: QueryResult) {
  const keys = result.columns.map((column) => column.key ?? column.name)
  const cell = (value: unknown) => {
    const raw = value === null || value === undefined ? '' : typeof value === 'object' ? JSON.stringify(value) : String(value)
    return `"${raw.replaceAll('"', '""')}"`
  }
  return [keys.map(cell).join(','), ...result.rows.map((row) => keys.map((key) => cell(row[key])).join(','))].join('\n')
}

function download(name: string, value: string, type: string) {
  const url = URL.createObjectURL(new Blob([value], { type }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = name
  anchor.click()
  URL.revokeObjectURL(url)
}

function JsonView({ result }: { result: QueryResult }) {
  return <pre className="h-full overflow-auto bg-[#070a11] p-4 font-mono text-[length:var(--font-size-data)] leading-5 text-foreground"><code>{jsonText(result)}</code></pre>
}

function TreeView({ result }: { result: QueryResult }) {
  return <div className="h-full overflow-auto p-3"><div className="space-y-1.5">{result.rows.map((row, index) => <details key={index} className="group rounded-lg border border-border bg-surface/55 open:bg-surface"><summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-2 text-[length:var(--font-size-ui)] font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring/30"><ChevronRight className="size-3.5 text-muted-foreground transition-transform group-open:rotate-90" /><span className="font-mono">row[{index}]</span><span className="ml-auto text-[length:var(--font-size-meta)] font-normal text-muted-foreground">{Object.keys(row).length} fields</span></summary><div className="border-t border-border px-3 py-2">{Object.entries(row).map(([key, value]) => <div key={key} className="grid grid-cols-[minmax(8rem,0.35fr)_minmax(0,1fr)] gap-3 border-b border-grid-line py-1.5 last:border-0"><span className="truncate font-mono text-[length:var(--font-size-meta)] text-primary">{key}</span><span className="min-w-0 break-all font-mono text-[length:var(--font-size-data)] text-foreground">{value === null ? <span className="text-muted-foreground italic">NULL</span> : typeof value === 'object' ? JSON.stringify(value) : String(value)}</span></div>)}</div></details>)}</div></div>
}

function PlanNodeView({ node, depth = 0 }: { node: QueryPlanNode; depth?: number }) {
  return <div className="relative" style={{ paddingLeft: `${depth * 18}px` }}><div className="flex min-w-0 items-start gap-3 rounded-lg border border-border bg-surface px-3 py-2.5"><div className="grid size-6 shrink-0 place-items-center rounded-md bg-accent font-mono text-[10px] font-semibold text-primary">{depth + 1}</div><div className="min-w-0 flex-1"><div className="flex min-w-0 flex-wrap items-center gap-2"><p className="truncate font-mono text-[length:var(--font-size-data)] font-semibold text-foreground">{node.operation}</p>{node.relation ? <Badge variant="outline">{node.relation}</Badge> : null}</div><div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{node.cost !== undefined ? <span>cost {node.cost}</span> : null}{node.actualTimeMs !== undefined ? <span>{node.actualTimeMs} ms</span> : null}{node.rows !== undefined ? <span>{node.rows.toLocaleString()} rows</span> : null}{node.loops !== undefined ? <span>{node.loops.toLocaleString()} loops</span> : null}</div></div></div>{node.children?.length ? <div className="mt-2 grid gap-2 border-l border-border pl-2">{node.children.map((child) => <PlanNodeView key={child.id} node={child} depth={depth + 1} />)}</div> : null}</div>
}

function PlanView({ result }: { result: QueryResult }) {
  if (result.plan?.tree.length) return <div className="h-full overflow-auto p-4"><div className="mx-auto grid max-w-5xl gap-2">{result.plan.tree.map((node) => <PlanNodeView key={node.id} node={node} />)}</div></div>
  const key = result.columns[0]?.key ?? result.columns[0]?.name ?? 'QUERY PLAN'
  const stages = result.rows.map((row) => String(row[key] ?? '')).filter(Boolean)
  return <div className="h-full overflow-auto p-4"><div className="mx-auto max-w-4xl space-y-2">{stages.map((stage, index) => <div key={`${stage}-${index}`} className="relative flex gap-3 rounded-lg border border-border bg-surface px-3 py-2.5 before:absolute before:top-full before:left-[1.15rem] before:h-2 before:w-px before:bg-border last:before:hidden"><div className="grid size-6 shrink-0 place-items-center rounded-md bg-accent font-mono text-[10px] font-semibold text-primary">{index + 1}</div><div className="min-w-0"><p className="break-words font-mono text-[length:var(--font-size-data)] leading-5 text-foreground">{stage}</p></div></div>)}</div></div>
}

export function QueryResultViews({ connectionId, result, mode, refreshing, canRefresh = true, onModeChange, onRefresh }: QueryResultViewsProps) {
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState('')
  const hasNumeric = useMemo(() => result.columns.some((column) => isNumericColumn(column, result)), [result])
  const isPlanResult = Boolean(result.plan) || result.columns.some((column) => column.name.toUpperCase().includes('QUERY PLAN'))
  const hasTabularResult = result.columns.length > 0

  async function copyResult() {
    try {
      await writeClipboardText(mode === 'table' ? csvText(result) : jsonText(result))
      setCopyError('')
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1400)
    } catch (error) {
      setCopied(false)
      setCopyError(error instanceof Error ? error.message : 'The query result could not be copied.')
    }
  }

  return <div className="flex h-full min-h-0 flex-col overflow-hidden">
    <div className="flex min-h-[var(--toolbar-height)] shrink-0 flex-wrap items-center gap-2 border-b border-border bg-surface/55 px-3 py-1.5">
      <Tabs value={mode} onValueChange={(value) => onModeChange(value as QueryResultMode)} className="gap-0"><TabsList className="h-[var(--control-height-sm)]"><TabsTrigger value="table" className="h-[var(--control-height-xs)] px-2.5"><Table2 />Table</TabsTrigger><TabsTrigger value="json" className="h-[var(--control-height-xs)] px-2.5"><FileJson2 />JSON</TabsTrigger><TabsTrigger value="tree" className="h-[var(--control-height-xs)] px-2.5"><GitBranch />Tree</TabsTrigger><TabsTrigger value="chart" disabled={!hasNumeric} className="h-[var(--control-height-xs)] px-2.5"><BarChart3 />Chart</TabsTrigger>{isPlanResult ? <TabsTrigger value="plan" className="h-[var(--control-height-xs)] px-2.5"><CodeXml />Plan</TabsTrigger> : null}</TabsList></Tabs>
      <div className="hidden items-center gap-2 text-[length:var(--font-size-meta)] text-muted-foreground md:flex"><span className="size-1.5 rounded-full bg-emerald-400" /><span>{result.rows.length.toLocaleString()} rows</span><span>·</span><span>{result.durationMs} ms</span>{result.rowsAffected ? <><span>·</span><span>{result.rowsAffected} affected</span></> : null}</div>
      {result.truncated ? <Badge variant="warning">Truncated{result.limits ? ` · ${result.limits.rowsRead.toLocaleString()} read` : ''}</Badge> : null}
      {result.notices?.length ? <Badge variant="outline">{result.notices.length} notice{result.notices.length === 1 ? '' : 's'}</Badge> : null}
      <div className="ml-auto flex items-center gap-1">
        <Tooltip><TooltipTrigger asChild><IconButton label="Refresh result" size="icon-xs" onClick={onRefresh} disabled={refreshing || !canRefresh}><RefreshCw className={refreshing ? 'animate-spin' : ''} /></IconButton></TooltipTrigger><TooltipContent>{canRefresh ? 'Run query again' : 'Connect this profile to refresh'}</TooltipContent></Tooltip>
        <Button size="xs" variant="ghost" onClick={copyResult}>{copied ? <Check /> : <Copy />}{copied ? 'Copied' : 'Copy'}</Button>
        <Button size="xs" variant="ghost" onClick={() => download(mode === 'json' ? 'query-result.json' : 'query-result.csv', mode === 'json' ? jsonText(result) : csvText(result), mode === 'json' ? 'application/json' : 'text/csv')}><Download />Export</Button>
      </div>
    </div>
    {copyError ? <div role="alert" className="flex shrink-0 items-center gap-2 border-b border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-meta)] text-destructive"><AlertCircle className="size-3.5 shrink-0" /><span className="min-w-0 flex-1">{copyError}</span><Button size="xs" variant="ghost" onClick={() => setCopyError('')}>Dismiss</Button></div> : null}
    <div className="min-h-0 flex-1 overflow-hidden">
      {mode === 'table' && hasTabularResult ? <QueryResultGrid connectionId={connectionId} result={result} refreshing={refreshing} onRefresh={onRefresh} /> : null}
      {mode === 'table' && !hasTabularResult ? <EmptyState className="h-full" icon={<CheckCircle2 />} title={result.message ?? 'Statement completed'} description={`${result.rowsAffected.toLocaleString()} row${result.rowsAffected === 1 ? '' : 's'} affected in ${result.durationMs} ms.`} /> : null}
      {mode === 'json' ? <JsonView result={result} /> : null}
      {mode === 'tree' ? <TreeView result={result} /> : null}
      {mode === 'chart' ? <Suspense fallback={<div className="grid h-full place-items-center text-[length:var(--font-size-ui)] text-muted-foreground"><BarChart3 className="mb-2 size-5" />Preparing chart…</div>}><LazyChart result={result} /></Suspense> : null}
      {mode === 'plan' ? <PlanView result={result} /> : null}
    </div>
  </div>
}

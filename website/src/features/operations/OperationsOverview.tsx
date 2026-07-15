import { Activity, CircleGauge, DatabaseZap, HardDrive, LockKeyhole, ServerCog, UsersRound, Zap } from 'lucide-react'
import { Area, AreaChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { DatabaseDashboard, DatabaseMetric, OperationsWindow } from '@/entities/database-object'
import { Badge, EmptyState, Skeleton, WorkspacePanel } from '@/shared/ui'

type OperationsOverviewProps = {
  dashboard?: DatabaseDashboard
  loading?: boolean
  sessionsCount: number
  sessionsAvailable?: boolean
  waitingLocks: number
  locksAvailable?: boolean
  slowQueryCount: number
  performanceAvailable?: boolean
  performanceLoaded?: boolean
  source: 'mock' | 'api'
}

const metricIcons = [UsersRound, Activity, CircleGauge, HardDrive]
const chartColors = ['var(--primary)', 'var(--success-foreground)', 'var(--warning-foreground)', 'var(--info-foreground)']

function seriesData(series: Array<{ timestamp: string; value: number }> = []) {
  return series.map((point) => ({ ...point, label: new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(new Date(point.timestamp)) }))
}

function windowLabel(window?: OperationsWindow) {
  if (window === '5m') return '5-minute window'
  if (window === '15m') return '15-minute window'
  if (window === '1h') return '1-hour window'
  if (window === '6h') return '6-hour window'
  if (window === '24h') return '24-hour window'
  return 'current collection window'
}

function collectedLabel(value?: string) {
  if (!value) return 'Collection time unavailable'
  return `Collected ${new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value))}`
}

function formatBytes(value: number) {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  if (!Number.isFinite(value) || value <= 0) return { value: '0', unit: 'B' }
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
  return { value: (value / 1024 ** index).toFixed(index ? 1 : 0), unit: units[index] }
}

function formatMetric(metric: DatabaseMetric) {
  if (typeof metric.value !== 'number') return { value: metric.value, unit: metric.unit }
  if (metric.unit === 'bytes') return formatBytes(metric.value)
  const value = Number.isInteger(metric.value) ? metric.value.toLocaleString() : metric.value.toLocaleString(undefined, { maximumFractionDigits: 2 })
  if (metric.unit === 'percent') return { value, unit: '%' }
  return { value, unit: metric.unit }
}

function signalValue(available: boolean | undefined, value: number) {
  return available ? value.toLocaleString() : '—'
}

export function OperationsOverview({ dashboard, loading, sessionsCount, sessionsAvailable, waitingLocks, locksAvailable, slowQueryCount, performanceAvailable, performanceLoaded, source }: OperationsOverviewProps) {
  if (loading) return <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">{Array.from({ length: 4 }, (_, index) => <Skeleton key={index} className="h-40" />)}</div>
  if (!dashboard?.available) return <EmptyState icon={<DatabaseZap />} title="Monitoring is unavailable" description={dashboard?.message ?? 'The selected adapter did not return database metrics.'} />
  const metrics = dashboard.metrics.slice(0, 4)
  const primaryMetric = dashboard.metrics.find((metric) => metric.key === 'queries_per_second') ?? dashboard.metrics.find((metric) => metric.series?.length)
  const secondaryMetric = dashboard.metrics.find((metric) => metric.key === 'cache_hit') ?? dashboard.metrics.find((metric) => metric !== primaryMetric && metric.series?.length)
  const primaryData = seriesData(primaryMetric?.series)
  const secondaryData = seriesData(secondaryMetric?.series)
  const primaryValue = primaryMetric ? formatMetric(primaryMetric) : undefined
  const secondaryValue = secondaryMetric ? formatMetric(secondaryMetric) : undefined
  const collectionDescription = source === 'mock' ? `${windowLabel(dashboard.window)} · sample telemetry` : `${windowLabel(dashboard.window)} · ${collectedLabel(dashboard.collectedAt)}`

  return <div className="space-y-4">
    {metrics.length ? <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">{metrics.map((metric, index) => { const Icon = metricIcons[index] ?? CircleGauge; const data = seriesData(metric.series); const formatted = formatMetric(metric); return <article key={metric.key} className="min-w-0 overflow-hidden rounded-xl border border-border bg-surface shadow-control"><div className="flex items-start justify-between gap-3 p-4 pb-0"><span className="grid size-9 place-items-center rounded-lg bg-accent text-primary"><Icon className="size-4" /></span><Badge variant={metric.trend === 'up' ? 'success' : metric.trend === 'down' ? 'destructive' : 'outline'}>{metric.detail || (source === 'mock' ? 'Sample' : 'Snapshot')}</Badge></div><div className="px-4 pt-3"><p className="text-[length:var(--font-size-meta)] font-medium text-muted-foreground">{metric.label}</p><p className="mt-0.5 text-2xl font-semibold tracking-[-0.04em] tabular-nums">{formatted.value}<span className="ml-1 text-[length:var(--font-size-ui)] font-normal text-muted-foreground">{formatted.unit}</span></p></div><div className="mt-1 h-12">{data.length > 1 ? <ResponsiveContainer width="100%" height="100%"><AreaChart data={data}><defs><linearGradient id={`metric-${index}`} x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor={chartColors[index]} stopOpacity={0.35} /><stop offset="100%" stopColor={chartColors[index]} stopOpacity={0} /></linearGradient></defs><Area type="monotone" dataKey="value" stroke={chartColors[index]} strokeWidth={1.5} fill={`url(#metric-${index})`} isAnimationActive={false} /></AreaChart></ResponsiveContainer> : <div className="mx-4 mt-5 border-t border-dashed border-border" />}</div></article> })}</div> : <EmptyState compact icon={<CircleGauge />} title="No dashboard metrics collected" description="The adapter is available but returned no metric values." />}
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1.45fr)_minmax(18rem,0.55fr)]">
      <WorkspacePanel title={primaryMetric?.label ?? 'Metric timeline'} description={collectionDescription} actions={<Badge variant={source === 'mock' ? 'accent' : 'success'}>{source === 'mock' ? 'Sample data' : `${primaryData.length} point${primaryData.length === 1 ? '' : 's'}`}</Badge>}>
        {primaryData.length > 1 ? <div className="h-64" role="img" aria-label={`${primaryMetric?.label ?? 'Database metric'} trend chart`}><ResponsiveContainer width="100%" height="100%"><LineChart data={primaryData} margin={{ top: 10, right: 10, left: -14, bottom: 0 }}><CartesianGrid stroke="var(--grid-line)" strokeDasharray="3 4" vertical={false} /><XAxis dataKey="label" tick={{ fill: 'var(--muted-foreground)', fontSize: 11 }} tickLine={false} axisLine={false} minTickGap={32} /><YAxis tick={{ fill: 'var(--muted-foreground)', fontSize: 11 }} tickLine={false} axisLine={false} /><Tooltip contentStyle={{ background: 'var(--popover)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12 }} labelStyle={{ color: 'var(--muted-foreground)' }} itemStyle={{ color: 'var(--foreground)' }} /><Line type="monotone" dataKey="value" name={primaryMetric?.label} stroke="var(--primary)" strokeWidth={2} dot={false} activeDot={{ r: 4, fill: 'var(--primary)' }} isAnimationActive={false} /></LineChart></ResponsiveContainer></div> : primaryMetric ? <div className="grid h-64 place-items-center rounded-lg border border-dashed border-border bg-background/35 text-center"><div><p className="text-4xl font-semibold tracking-[-0.04em] tabular-nums text-foreground">{primaryValue?.value}<span className="ml-1 text-base font-normal text-muted-foreground">{primaryValue?.unit}</span></p><p className="mt-2 text-[length:var(--font-size-ui)] text-muted-foreground">Only the current snapshot was returned, so no trend is inferred.</p></div></div> : <EmptyState compact icon={<Activity />} title="No time-series metric returned" description="Refresh after the adapter has collected metric points." />}
      </WorkspacePanel>
      <WorkspacePanel title="Operational signals" description="Values from independent live feeds">
        <div className="space-y-2.5"><div className="flex items-center gap-3 rounded-lg border border-border bg-background/60 p-3"><span className="grid size-8 place-items-center rounded-lg bg-info text-info-foreground"><ServerCog className="size-4" /></span><span className="min-w-0 flex-1"><span className="block text-[length:var(--font-size-ui)] font-medium">Database sessions</span><span className="text-[length:var(--font-size-meta)] text-muted-foreground">{sessionsAvailable ? 'Rows returned by the session feed' : 'Session feed unavailable'}</span></span><strong className="font-mono text-base tabular-nums">{signalValue(sessionsAvailable, sessionsCount)}</strong></div><div className="flex items-center gap-3 rounded-lg border border-border bg-background/60 p-3"><span className={`grid size-8 place-items-center rounded-lg ${locksAvailable && waitingLocks ? 'bg-warning text-warning-foreground' : 'bg-success text-success-foreground'}`}><LockKeyhole className="size-4" /></span><span className="min-w-0 flex-1"><span className="block text-[length:var(--font-size-ui)] font-medium">Waiting locks</span><span className="text-[length:var(--font-size-meta)] text-muted-foreground">{locksAvailable ? 'Blocked lock records' : 'Lock feed unavailable'}</span></span><strong className="font-mono text-base tabular-nums">{signalValue(locksAvailable, waitingLocks)}</strong></div><div className="flex items-center gap-3 rounded-lg border border-border bg-background/60 p-3"><span className="grid size-8 place-items-center rounded-lg bg-accent text-primary"><Zap className="size-4" /></span><span className="min-w-0 flex-1"><span className="block text-[length:var(--font-size-ui)] font-medium">Tracked statements</span><span className="text-[length:var(--font-size-meta)] text-muted-foreground">{performanceLoaded ? performanceAvailable ? 'Statement fingerprints returned' : 'Performance feed unavailable' : 'Open Performance to load statement metrics'}</span></span><strong className="font-mono text-base tabular-nums">{signalValue(performanceLoaded ? performanceAvailable : false, slowQueryCount)}</strong></div></div>
      </WorkspacePanel>
    </div>
    {secondaryMetric ? <WorkspacePanel title={`${secondaryMetric.label} samples`} description={collectionDescription}><div className="grid items-center gap-5 md:grid-cols-[13rem_minmax(0,1fr)]"><div><p className="text-3xl font-semibold tracking-[-0.04em] tabular-nums">{secondaryValue?.value}<span className="ml-1 text-sm font-normal text-muted-foreground">{secondaryValue?.unit}</span></p><p className="mt-2 text-[length:var(--font-size-ui)] leading-5 text-muted-foreground">Latest collected value. DataDock does not infer health or thresholds from this sample.</p><Badge className="mt-3" variant="outline">{secondaryData.length} collected point{secondaryData.length === 1 ? '' : 's'}</Badge></div>{secondaryData.length > 1 ? <div className="h-36" role="img" aria-label={`${secondaryMetric.label} trend chart`}><ResponsiveContainer width="100%" height="100%"><AreaChart data={secondaryData} margin={{ left: -20, right: 8, top: 8 }}><CartesianGrid stroke="var(--grid-line)" strokeDasharray="3 4" vertical={false} /><XAxis dataKey="label" hide /><YAxis domain={['dataMin - 1', 'dataMax + 1']} tick={{ fill: 'var(--muted-foreground)', fontSize: 11 }} tickLine={false} axisLine={false} /><Tooltip contentStyle={{ background: 'var(--popover)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12 }} /><Area type="monotone" dataKey="value" stroke="var(--success-foreground)" strokeWidth={2} fill="var(--success)" isAnimationActive={false} /></AreaChart></ResponsiveContainer></div> : <div className="grid h-36 place-items-center rounded-lg border border-dashed border-border text-center text-[length:var(--font-size-ui)] text-muted-foreground">More collected points are required for a trend.</div>}</div></WorkspacePanel> : null}
  </div>
}

import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, AlertTriangle, Clock3, DatabaseZap, Gauge, LockKeyhole, Pause, Play, RefreshCw, ServerCog, TimerReset } from 'lucide-react'
import type { Connection } from '@/entities/connection'
import { useDataDockGateway } from '@/app/providers'
import { LocksPanel } from '@/features/operations/LocksPanel'
import { OperationsOverview } from '@/features/operations/OperationsOverview'
import { SessionsPanel } from '@/features/operations/SessionsPanel'
import { PerformancePanel, SlowQueriesPanel } from '@/features/operations/SlowQueriesPanel'
import { APP_CONFIG } from '@/shared/config/constants'
import {
  Badge,
  Button,
  EmptyState,
  Select,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  WorkspaceHeader,
  WorkspacePage,
  WorkspaceToolbar,
} from '@/shared/ui'

type OperationsPageProps = {
  connection: Connection
  onOpenQuery?: (sql: string) => void
}

type OperationsArea = 'overview' | 'sessions' | 'locks' | 'slow' | 'performance'

export function OperationsPage({ connection, onOpenQuery }: OperationsPageProps) {
  const gateway = useDataDockGateway()
  const queryClient = useQueryClient()
  const [pollMs, setPollMs] = useState<number>(APP_CONFIG.operations.dashboardRefreshMs)
  const [activeArea, setActiveArea] = useState<OperationsArea>('overview')
  const needsDashboard = activeArea === 'overview'
  const needsSessions = activeArea === 'overview' || activeArea === 'sessions' || activeArea === 'locks'
  const needsLocks = activeArea === 'overview' || activeArea === 'locks'
  const needsPerformance = activeArea === 'slow' || activeArea === 'performance'
  const dashboard = useQuery({ queryKey: ['dashboard', connection.id], queryFn: ({ signal }) => gateway.getDashboard(connection.id, signal), enabled: needsDashboard, refetchInterval: needsDashboard && pollMs ? pollMs : false })
  const sessions = useQuery({ queryKey: ['sessions', connection.id], queryFn: ({ signal }) => gateway.getSessions(connection.id, signal), enabled: needsSessions, refetchInterval: needsSessions && pollMs ? pollMs : false })
  const locks = useQuery({ queryKey: ['locks', connection.id], queryFn: ({ signal }) => gateway.getLocks(connection.id, signal), enabled: needsLocks, refetchInterval: needsLocks && pollMs ? pollMs : false })
  const performance = useQuery({ queryKey: ['performance', connection.id], queryFn: ({ signal }) => gateway.getPerformance(connection.id, signal), enabled: needsPerformance, refetchInterval: needsPerformance && pollMs ? Math.max(pollMs, 30_000) : false })
  const sessionAction = useMutation({
    mutationFn: ({ sessionId, force }: { sessionId: string; force: boolean }) => gateway.cancelSession(connection.id, sessionId, force),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['sessions', connection.id] }),
        queryClient.invalidateQueries({ queryKey: ['locks', connection.id] }),
      ])
    },
  })
  const isLive = gateway.source === 'api'
  const dashboardFeed = { available: dashboard.data?.available, loaded: Boolean(dashboard.data) || dashboard.isError, updatedAt: dashboard.dataUpdatedAt, fetching: dashboard.isFetching, error: dashboard.error, refetch: () => dashboard.refetch() }
  const sessionsFeed = { available: sessions.data?.available, loaded: Boolean(sessions.data) || sessions.isError, updatedAt: sessions.dataUpdatedAt, fetching: sessions.isFetching, error: sessions.error, refetch: () => sessions.refetch() }
  const locksFeed = { available: locks.data?.available, loaded: Boolean(locks.data) || locks.isError, updatedAt: locks.dataUpdatedAt, fetching: locks.isFetching, error: locks.error, refetch: () => locks.refetch() }
  const performanceFeed = { available: performance.data?.available, loaded: Boolean(performance.data) || performance.isError, updatedAt: performance.dataUpdatedAt, fetching: performance.isFetching, error: performance.error, refetch: () => performance.refetch() }
  const activeFeeds = activeArea === 'overview'
    ? [dashboardFeed, sessionsFeed, locksFeed]
    : activeArea === 'sessions'
      ? [sessionsFeed]
      : activeArea === 'locks'
        ? [locksFeed, sessionsFeed]
        : [performanceFeed]
  const loadedFeedCount = activeFeeds.filter((feed) => feed.loaded).length
  const availableFeedCount = activeFeeds.filter((feed) => feed.available).length
  const allAvailable = activeFeeds.length > 0 && availableFeedCount === activeFeeds.length
  const refreshedAt = Math.max(0, ...activeFeeds.map((feed) => feed.updatedAt))
  const refreshing = activeFeeds.some((feed) => feed.fetching)
  const waitingLocks = locks.data?.items.filter((lock) => !lock.granted).length ?? 0
  const failedFeeds = activeFeeds.filter((feed) => feed.error)
  const firstError = activeFeeds.map((feed) => feed.error).find((error): error is Error => error instanceof Error)
  const sourceLabel = !isLive ? 'Demo data' : loadedFeedCount < activeFeeds.length ? 'Loading live feeds' : allAvailable ? 'Live API' : availableFeedCount ? 'Partially available' : 'Unavailable'
  const lastUpdated = useMemo(() => refreshedAt ? new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(refreshedAt) : 'Not refreshed', [refreshedAt])

  async function refreshActive() {
    await Promise.all(activeFeeds.map((feed) => feed.refetch()))
  }

  return <WorkspacePage>
    <WorkspaceHeader
      icon={<DatabaseZap />}
      eyebrow="Operations"
      title="Database health"
      description={`${connection.name} · ${dashboard.data?.version ?? connection.engine}`}
      meta={<Badge variant={isLive && allAvailable ? 'success' : isLive ? 'warning' : 'accent'}>{sourceLabel}</Badge>}
      actions={<><div className="hidden items-center gap-1.5 text-[length:var(--font-size-meta)] text-muted-foreground lg:flex"><span className={`size-1.5 rounded-full ${isLive && allAvailable ? 'bg-emerald-400' : 'bg-amber-400'}`} /><span>Updated {lastUpdated}</span></div><Button size="sm" variant="outline" disabled={refreshing} onClick={refreshActive}><RefreshCw className={refreshing ? 'animate-spin' : ''} />Refresh</Button></>}
    />
    {failedFeeds.length ? <div role="alert" className="flex shrink-0 flex-wrap items-center gap-3 border-b border-destructive/25 bg-destructive/10 px-5 py-2.5 text-[length:var(--font-size-ui)]"><AlertTriangle className="size-4 text-destructive" /><div className="min-w-0 flex-1"><p className="font-medium text-foreground">{failedFeeds.length === 1 ? 'One operations feed failed to load' : `${failedFeeds.length} operations feeds failed to load`}</p><p className="truncate text-[length:var(--font-size-meta)] text-muted-foreground">{firstError?.message ?? 'The selected adapter did not return telemetry. Loaded panels remain available.'}</p></div><Button size="xs" variant="outline" disabled={refreshing} onClick={refreshActive}><RefreshCw className={refreshing ? 'animate-spin' : ''} />Retry failed feeds</Button></div> : null}
    <Tabs value={activeArea} onValueChange={(value) => setActiveArea(value as OperationsArea)} className="min-h-0 flex-1 gap-0 overflow-hidden">
      <WorkspaceToolbar className="overflow-x-auto">
        <TabsList className="h-auto min-w-max border-0 bg-transparent p-0"><TabsTrigger value="overview"><Gauge />Overview</TabsTrigger><TabsTrigger value="sessions"><ServerCog />Sessions <Badge variant="outline" className="ml-1 px-1.5">{sessions.data?.items.length ?? 0}</Badge></TabsTrigger><TabsTrigger value="locks"><LockKeyhole />Locks {waitingLocks ? <Badge variant="warning" className="ml-1 px-1.5">{waitingLocks}</Badge> : null}</TabsTrigger><TabsTrigger value="slow"><TimerReset />Slow queries</TabsTrigger><TabsTrigger value="performance"><Activity />Performance</TabsTrigger></TabsList>
        <div className="ml-auto flex shrink-0 items-center gap-2"><Badge variant="outline" className="hidden sm:inline-flex">{isLive ? 'System catalog' : 'Sample telemetry'}</Badge><div className="flex items-center gap-1.5 text-[length:var(--font-size-meta)] text-muted-foreground">{pollMs ? <Play className="size-3.5 text-success-foreground" /> : <Pause className="size-3.5" />}<Select aria-label="Polling interval" className="h-[var(--control-height-sm)] w-32" value={pollMs} onChange={(event) => setPollMs(Number(event.target.value))}><option value={0}>Polling paused</option><option value={5_000}>Every 5s</option><option value={15_000}>Every 15s</option><option value={30_000}>Every 30s</option><option value={60_000}>Every minute</option></Select></div></div>
      </WorkspaceToolbar>
      <div className="shrink-0 border-b border-border bg-surface/30 px-5 py-2 text-[length:var(--font-size-meta)] text-muted-foreground"><Clock3 className="mr-1.5 inline size-3.5 text-primary" />{isLive ? 'Metrics are read from the selected database adapter. Availability depends on engine permissions and extensions.' : 'Every value in this workspace is sample data for UI review. No database telemetry is being queried.'}</div>
      <TabsContent value="overview" className="min-h-0 overflow-auto p-4 lg:p-5">{dashboard.isError && !dashboard.data ? <EmptyState icon={<DatabaseZap />} title="Dashboard feed unavailable" description={dashboard.error.message} actions={<Button variant="outline" onClick={() => dashboard.refetch()}><RefreshCw />Retry dashboard</Button>} /> : <OperationsOverview dashboard={dashboard.data} loading={dashboard.isLoading} sessionsCount={sessions.data?.items.length ?? 0} sessionsAvailable={sessions.data?.available} waitingLocks={waitingLocks} locksAvailable={locks.data?.available} slowQueryCount={performance.data?.slowQueries.length ?? 0} performanceAvailable={performance.data?.available} performanceLoaded={performance.isFetched} source={gateway.source} />}</TabsContent>
      <TabsContent value="sessions" className="min-h-0 overflow-auto p-4 lg:p-5">{sessions.isError && !sessions.data ? <EmptyState icon={<ServerCog />} title="Session feed unavailable" description={sessions.error.message} actions={<Button variant="outline" onClick={() => sessions.refetch()}><RefreshCw />Retry sessions</Button>} /> : <SessionsPanel sessions={sessions.data} loading={sessions.isLoading} source={gateway.source} onCancel={(sessionId) => sessionAction.mutateAsync({ sessionId, force: false })} onTerminate={(sessionId) => sessionAction.mutateAsync({ sessionId, force: true })} />}</TabsContent>
      <TabsContent value="locks" className="min-h-0 overflow-auto p-4 lg:p-5">{locks.isError && !locks.data ? <EmptyState icon={<LockKeyhole />} title="Lock feed unavailable" description={locks.error.message} actions={<Button variant="outline" onClick={() => locks.refetch()}><RefreshCw />Retry locks</Button>} /> : <LocksPanel locks={locks.data} sessions={sessions.data?.items} loading={locks.isLoading} source={gateway.source} />}</TabsContent>
      <TabsContent value="slow" className="min-h-0 overflow-auto p-4 lg:p-5">{performance.isError && !performance.data ? <EmptyState icon={<TimerReset />} title="Performance feed unavailable" description={performance.error.message} actions={<Button variant="outline" onClick={() => performance.refetch()}><RefreshCw />Retry performance</Button>} /> : <SlowQueriesPanel performance={performance.data} loading={performance.isLoading} source={gateway.source} onOpenQuery={onOpenQuery} />}</TabsContent>
      <TabsContent value="performance" className="min-h-0 overflow-auto p-4 lg:p-5">{performance.isError && !performance.data ? <EmptyState icon={<Activity />} title="Performance feed unavailable" description={performance.error.message} actions={<Button variant="outline" onClick={() => performance.refetch()}><RefreshCw />Retry performance</Button>} /> : <PerformancePanel performance={performance.data} loading={performance.isLoading} source={gateway.source} />}</TabsContent>
    </Tabs>
  </WorkspacePage>
}

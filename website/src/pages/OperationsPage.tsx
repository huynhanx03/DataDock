import { useQuery } from '@tanstack/react-query'
import { Activity, Clock3, DatabaseZap, Gauge, HardDrive, LockKeyhole, ServerCog, UsersRound } from 'lucide-react'
import type { Connection } from '@/entities/connection'
import { useDataDockGateway } from '@/app/providers'
import { Badge, Skeleton } from '@/shared/ui'

const metricIcons = [UsersRound, Activity, Gauge, HardDrive]

export function OperationsPage({ connection }: { connection: Connection }) {
  const gateway = useDataDockGateway()
  const dashboard = useQuery({ queryKey: ['dashboard', connection.id], queryFn: ({ signal }) => gateway.getDashboard(connection.id, signal) })
  const sessions = useQuery({ queryKey: ['sessions', connection.id], queryFn: ({ signal }) => gateway.getSessions(connection.id, signal) })
  const locks = useQuery({ queryKey: ['locks', connection.id], queryFn: ({ signal }) => gateway.getLocks(connection.id, signal) })
  return (
    <section className="h-full overflow-y-auto bg-background px-6 py-6 lg:px-8">
      <header className="flex items-center gap-4">
        <div className="grid size-11 place-items-center rounded-xl border border-primary/25 bg-accent text-accent-foreground"><DatabaseZap className="size-5" /></div>
        <div>
          <p className="text-[length:var(--font-size-meta)] font-semibold tracking-[0.08em] text-primary uppercase">Live operations</p>
          <h1 className="mt-0.5 text-[1.625rem] font-semibold tracking-[-0.035em]">Database health</h1>
          <p className="mt-1 text-[length:var(--font-size-ui)] text-muted-foreground">{connection.name} · {dashboard.data?.version ?? connection.engine}</p>
        </div>
        <Badge variant="success" className="ml-auto text-[length:var(--font-size-meta)]">All systems operational</Badge>
      </header>
      <div className="mt-7 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        {dashboard.isLoading ? Array.from({ length: 4 }, (_, index) => <Skeleton key={index} className="h-32" />) : dashboard.data?.metrics.slice(0, 4).map((metric, index) => {
          const Icon = metricIcons[index] ?? Gauge
          return (
            <article key={metric.key} className="rounded-xl border border-border bg-surface/75 p-4 shadow-control">
              <div className="flex items-center justify-between">
                <span className="grid size-9 place-items-center rounded-lg bg-accent text-accent-foreground"><Icon className="size-4" /></span>
                <Badge variant={metric.trend === 'up' ? 'success' : 'outline'} className="text-[length:var(--font-size-meta)]">{metric.detail ?? 'Live'}</Badge>
              </div>
              <p className="mt-5 text-[length:var(--font-size-meta)] font-medium text-muted-foreground">{metric.label}</p>
              <p className="mt-1 text-2xl font-semibold tracking-[-0.035em]">{metric.value}<span className="ml-1 text-[length:var(--font-size-ui)] font-normal text-muted-foreground">{metric.unit}</span></p>
            </article>
          )
        })}
      </div>
      <div className="mt-4 grid gap-4 xl:grid-cols-[1.4fr_0.8fr]">
        <article className="overflow-hidden rounded-xl border border-border bg-surface/70">
          <header className="flex h-[var(--toolbar-height)] items-center gap-2 border-b border-border px-4">
            <ServerCog className="size-4 text-primary" />
            <h2 className="text-sm font-semibold">Active sessions</h2>
            <Badge variant="outline" className="ml-auto text-[length:var(--font-size-meta)]">{sessions.data?.items.length ?? 0}</Badge>
          </header>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px] border-collapse text-[length:var(--font-size-data)]">
              <thead>
                <tr>{['PID', 'User', 'State', 'Duration', 'Query'].map((label) => <th key={label} className="h-[var(--data-row-height)] border-b border-grid-line bg-grid-header px-3 text-left text-muted-foreground">{label}</th>)}</tr>
              </thead>
              <tbody>
                {sessions.data?.items.map((session) => (
                  <tr key={session.id} className="hover:bg-accent/35">
                    <td className="h-[var(--data-row-height)] border-b border-grid-line px-3 font-mono">{session.id}</td>
                    <td className="border-b border-grid-line px-3">{session.user}</td>
                    <td className="border-b border-grid-line px-3"><Badge variant={session.state === 'active' ? 'success' : 'outline'} className="text-[length:var(--font-size-meta)]">{session.state}</Badge></td>
                    <td className="border-b border-grid-line px-3 font-mono text-muted-foreground">{session.durationMs ?? 0} ms</td>
                    <td className="max-w-96 truncate border-b border-grid-line px-3 font-mono text-muted-foreground">{session.query ?? '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </article>
        <article className="rounded-xl border border-border bg-surface/70">
          <header className="flex h-[var(--toolbar-height)] items-center gap-2 border-b border-border px-4">
            <LockKeyhole className="size-4 text-amber-400" />
            <h2 className="text-sm font-semibold">Lock activity</h2>
            <Badge variant={locks.data?.items.length ? 'warning' : 'success'} className="ml-auto text-[length:var(--font-size-meta)]">{locks.data?.items.length ?? 0}</Badge>
          </header>
          <div className="space-y-2 p-3">
            {locks.data?.items.slice(0, 5).map((lock) => (
              <div key={lock.id} className="rounded-lg border border-border bg-background/45 p-3">
                <div className="flex items-center justify-between">
                  <span className="text-[length:var(--font-size-data)] font-medium">{lock.type}</span>
                  <Badge variant={lock.granted ? 'success' : 'warning'} className="text-[length:var(--font-size-meta)]">{lock.granted ? 'Granted' : 'Waiting'}</Badge>
                </div>
                <p className="mt-2 truncate font-mono text-[length:var(--font-size-data)] text-muted-foreground">{lock.object ?? lock.query ?? 'Database lock'}</p>
              </div>
            ))}
            {!locks.data?.items.length ? (
              <div className="grid min-h-40 place-items-center text-center">
                <div>
                  <LockKeyhole className="mx-auto size-5 text-emerald-400" />
                  <p className="mt-2 text-sm font-medium">No blocking locks</p>
                  <p className="mt-1 text-[length:var(--font-size-meta)] text-muted-foreground"><Clock3 className="mr-1 inline size-3" />Checked just now</p>
                </div>
              </div>
            ) : null}
          </div>
        </article>
      </div>
    </section>
  )
}

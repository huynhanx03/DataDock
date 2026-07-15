import { ArrowRight, Braces, Check, Clock3, Database, Play, Plus, ShieldCheck, Sparkles, Zap } from 'lucide-react'
import type { Connection, DatabaseEngine } from '@/entities/connection'
import { APP_CONFIG } from '@/shared/config/constants'
import type { DataSource } from '@/shared/config/env'
import { cn } from '@/shared/lib/cn'
import { Badge, Button } from '@/shared/ui'

type ConnectionOnboardingProps = {
  connections: Connection[]
  source: DataSource
  onNewConnection: (engine?: DatabaseEngine) => void
  onSelectConnection: (connection: Connection) => void
}

const ENGINE_CARDS: Array<{
  engine: DatabaseEngine
  label: string
  mark: string
  className: string
  description: string
}> = [
  {
    engine: 'postgresql',
    label: 'PostgreSQL',
    mark: 'PG',
    className: 'border-sky-400/20 bg-sky-400/8 text-sky-300',
    description: 'Schemas, explain plans, locks',
  },
  {
    engine: 'mysql',
    label: 'MySQL',
    mark: 'MY',
    className: 'border-amber-400/20 bg-amber-400/8 text-amber-300',
    description: 'Fast browsing and editing',
  },
  {
    engine: 'mariadb',
    label: 'MariaDB',
    mark: 'MA',
    className: 'border-teal-400/20 bg-teal-400/8 text-teal-300',
    description: 'Native MariaDB workflows',
  },
]

function WorkspacePreview() {
  return (
    <div className="relative min-h-[260px] overflow-hidden rounded-2xl border border-border bg-[linear-gradient(145deg,color-mix(in_oklab,var(--surface-elevated)_96%,transparent),color-mix(in_oklab,var(--background)_95%,black))] shadow-[0_28px_70px_color-mix(in_oklab,var(--primitive-black)_30%,transparent)]">
      <div className="flex h-[var(--toolbar-height)] items-center gap-2 border-b border-border px-3.5">
        <div className="flex gap-1.5">
          <span className="size-2 rounded-full bg-red-400/70" />
          <span className="size-2 rounded-full bg-amber-400/70" />
          <span className="size-2 rounded-full bg-emerald-400/70" />
        </div>
        <span className="ml-2 font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{APP_CONFIG.query.fileName}</span>
        <Badge variant="accent" className="ml-auto text-[length:var(--font-size-meta)]">Preview</Badge>
      </div>
      <div className="grid grid-cols-[42px_1fr] border-b border-border/80 bg-background/40 font-mono text-[length:var(--font-size-data)] leading-6">
        <div className="border-r border-border/70 py-3 pr-2 text-right text-muted-foreground/60">1<br />2<br />3</div>
        <pre className="overflow-hidden py-3 pl-3.5 text-foreground"><span className="text-fuchsia-300">SELECT</span> id, email, plan{`\n`}<span className="text-fuchsia-300">FROM</span> public.users{`\n`}<span className="text-fuchsia-300">ORDER BY</span> created_at <span className="text-sky-300">DESC</span>;</pre>
      </div>
      <div className="grid grid-cols-[1.15fr_1.6fr_0.8fr] text-[length:var(--font-size-data)]">
        {['id', 'email', 'plan'].map((column) => (
          <div key={column} className="border-r border-b border-border bg-muted/35 px-3 py-2 font-mono font-semibold text-muted-foreground last:border-r-0">{column}</div>
        ))}
        {[
          ['usr_00042', 'avery.nguyen@example.com', 'Scale'],
          ['usr_00041', 'mia.tran@example.com', 'Pro'],
          ['usr_00040', 'noah.patel@example.com', 'Enterprise'],
        ].flatMap((row) => row.map((cell, index) => (
          <div key={`${cell}-${index}`} className="truncate border-r border-b border-border/65 px-3 py-2 font-mono text-foreground/85 last:border-r-0">{cell}</div>
        )))}
      </div>
      <div className="absolute right-3 bottom-3 flex items-center gap-1.5 rounded-full border border-emerald-400/20 bg-emerald-400/10 px-2.5 py-1 text-[length:var(--font-size-meta)] font-medium text-emerald-300 backdrop-blur-md">
        <Check className="size-3" />
        128 rows ready
      </div>
    </div>
  )
}

export function ConnectionOnboarding({ connections, source, onNewConnection, onSelectConnection }: ConnectionOnboardingProps) {
  const featuredConnection = connections.find((connection) => connection.status === 'connected') || connections[0]

  return (
    <div className="relative h-full overflow-y-auto bg-background">
      <div className="pointer-events-none absolute inset-0 opacity-35 [background-image:linear-gradient(to_right,color-mix(in_oklab,var(--border)_45%,transparent)_1px,transparent_1px),linear-gradient(to_bottom,color-mix(in_oklab,var(--border)_45%,transparent)_1px,transparent_1px)] [background-size:32px_32px] [mask-image:linear-gradient(to_bottom,black,transparent_72%)]" />
      <div className="pointer-events-none absolute top-[-10rem] left-[16%] size-[32rem] rounded-full bg-primary/10 blur-[110px]" />
      <div className="relative mx-auto flex min-h-full w-full max-w-[1420px] flex-col px-6 py-8 sm:px-8 lg:px-10 lg:py-12 2xl:px-14">
        <div className="grid items-center gap-10 xl:grid-cols-[minmax(0,0.92fr)_minmax(480px,1.08fr)] xl:gap-16">
          <section>
            <Badge variant="accent" className="mb-5 gap-1.5 px-2.5 py-1 text-[length:var(--font-size-meta)]">
              <Sparkles className="size-3" />
              Self-hosted database workspace
            </Badge>
            <h1 className="max-w-2xl text-[clamp(2rem,4vw,3.65rem)] leading-[1.04] font-semibold tracking-[-0.055em] text-foreground">
              Your databases,
              <span className="block bg-gradient-to-r from-primary via-indigo-300 to-sky-300 bg-clip-text text-transparent">beautifully in focus.</span>
            </h1>
            <p className="mt-5 max-w-xl text-[length:var(--font-size-ui)] leading-6 text-muted-foreground">
              Explore schemas, edit data, run SQL, and inspect performance from one fast workspace built for everyday database work.
            </p>
            <div className="mt-7 flex flex-wrap gap-3">
              <Button size="lg" onClick={() => onNewConnection()} className="shadow-[0_12px_32px_color-mix(in_oklab,var(--primary)_24%,transparent)]">
                <Plus />
                New connection
              </Button>
              {featuredConnection ? (
                <Button size="lg" variant="outline" onClick={() => onSelectConnection(featuredConnection)}>
                  <Play />
                  {source === 'mock' ? 'Explore demo workspace' : 'Browse saved connection'}
                </Button>
              ) : null}
            </div>
            <div className="mt-7 flex flex-wrap gap-x-5 gap-y-2 text-[length:var(--font-size-meta)] text-muted-foreground">
              <span className="flex items-center gap-1.5"><ShieldCheck className="size-3.5 text-emerald-400" />Credentials stay encrypted</span>
              <span className="flex items-center gap-1.5"><Zap className="size-3.5 text-amber-400" />{source === 'mock' ? 'Mock-first, instant preview' : 'Live API, persisted profiles'}</span>
            </div>
          </section>
          <WorkspacePreview />
        </div>

        <section className="mt-12 lg:mt-16">
          <div className="mb-4 flex items-end justify-between gap-4">
            <div>
              <p className="text-[length:var(--font-size-meta)] font-semibold tracking-[0.09em] text-primary uppercase">Quick start</p>
              <h2 className="mt-1 text-lg font-semibold tracking-[-0.02em] text-foreground">Connect your first database</h2>
            </div>
            <span className="hidden text-[length:var(--font-size-meta)] text-muted-foreground sm:block">More engines are coming through the same adapter layer</span>
          </div>
          <div className="grid gap-3 md:grid-cols-3">
            {ENGINE_CARDS.map((engine) => (
              <button
                type="button"
                key={engine.engine}
                onClick={() => onNewConnection(engine.engine)}
                className="group flex min-h-24 items-center gap-3.5 rounded-xl border border-border bg-surface/78 p-4 text-left shadow-control outline-none transition-[border-color,background-color,box-shadow] duration-200 hover:border-primary/45 hover:bg-surface-elevated hover:shadow-panel focus-visible:ring-2 focus-visible:ring-ring/45"
              >
                <span className={cn('grid size-11 shrink-0 place-items-center rounded-xl border font-mono text-[length:var(--font-size-meta)] font-bold', engine.className)}>{engine.mark}</span>
                <span className="min-w-0 flex-1">
                  <span className="block text-[length:var(--font-size-ui)] font-semibold text-foreground">{engine.label}</span>
                  <span className="mt-1 block truncate text-[length:var(--font-size-meta)] text-muted-foreground">{engine.description}</span>
                </span>
                <ArrowRight className="size-4 -translate-x-1 text-muted-foreground opacity-0 transition-[opacity,transform] group-hover:translate-x-0 group-hover:opacity-100" />
              </button>
            ))}
          </div>
        </section>

        {connections.length ? (
          <section className="mt-10 pb-6">
            <div className="mb-4 flex items-center justify-between">
              <div>
                <p className="text-[length:var(--font-size-meta)] font-semibold tracking-[0.09em] text-muted-foreground uppercase">Recent</p>
                <h2 className="mt-1 text-base font-semibold text-foreground">Saved connections</h2>
              </div>
              <Button variant="ghost" size="sm">View all <ArrowRight /></Button>
            </div>
            <div className="overflow-hidden rounded-xl border border-border bg-surface/72 shadow-control">
              {connections.slice(0, 3).map((connection, index) => (
                <button
                  type="button"
                  key={connection.id}
                  onClick={() => onSelectConnection(connection)}
                  className={cn('group flex w-full items-center gap-3 px-4 py-3 text-left outline-none transition-colors hover:bg-accent/55 focus-visible:bg-accent', index > 0 && 'border-t border-border')}
                >
                  <span className="grid size-9 shrink-0 place-items-center rounded-lg border border-border bg-muted/45 font-mono text-[length:var(--font-size-meta)] font-bold text-foreground">{connection.engine.slice(0, 2).toUpperCase()}</span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[length:var(--font-size-ui)] font-medium text-foreground">{connection.name}</span>
                    <span className="mt-0.5 block truncate font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{connection.database} · {connection.username}</span>
                  </span>
                  <span className="hidden items-center gap-1.5 text-[length:var(--font-size-meta)] text-muted-foreground sm:flex"><Clock3 className="size-3.5" />Recently used</span>
                  <span className={cn('size-1.5 rounded-full bg-muted-foreground', connection.status === 'connected' && 'bg-emerald-400', connection.status === 'error' && 'bg-red-400')} />
                  <ArrowRight className="size-4 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
                </button>
              ))}
            </div>
          </section>
        ) : null}
      </div>
    </div>
  )
}

import { useRef, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Braces, Clock3, Code2, Play, Save, Sparkles, Table2 } from 'lucide-react'
import type { Connection } from '@/entities/connection'
import type { QueryResult } from '@/entities/query'
import { useDataDockGateway } from '@/app/providers'
import { QueryResultGrid } from '@/features/query/QueryResultGrid'
import { APP_CONFIG } from '@/shared/config/constants'
import { Badge, Button, Tabs, TabsList, TabsTrigger } from '@/shared/ui'

const DEFAULT_SQL = `SELECT
  id,
  email,
  full_name,
  role,
  plan,
  mrr,
  created_at
FROM public.users
WHERE status = 'active'
ORDER BY created_at DESC
LIMIT 100;`

export function QueryPage({ connection }: { connection: Connection }) {
  const gateway = useDataDockGateway()
  const [sql, setSql] = useState(DEFAULT_SQL)
  const [result, setResult] = useState<QueryResult>()
  const executionSequence = useRef(0)
  const execution = useMutation({
    mutationFn: async ({ sqlText, sequence }: { sqlText: string; sequence: number }) => ({ sequence, result: await gateway.executeQuery({ connectionId: connection.id, sql: sqlText, timeoutSeconds: APP_CONFIG.query.defaultTimeoutSeconds }) }),
    onMutate: () => setResult(undefined),
    onSuccess: (completed) => { if (completed.sequence === executionSequence.current) setResult(completed.result) },
  })

  function runQuery() {
    if (!sql.trim() || execution.isPending) return
    executionSequence.current += 1
    execution.mutate({ sqlText: sql, sequence: executionSequence.current })
  }

  return <section className="flex h-full min-h-0 flex-col bg-background">
    <header className="flex h-[76px] shrink-0 items-center gap-4 border-b border-border px-6 lg:px-8">
      <div className="grid size-10 place-items-center rounded-xl border border-primary/25 bg-accent text-accent-foreground"><Braces className="size-5" /></div>
      <div><h1 className="text-lg font-semibold tracking-[-0.02em]">Query workspace</h1><p className="mt-0.5 flex items-center gap-2 text-[length:var(--font-size-meta)] text-muted-foreground"><span className="size-1.5 rounded-full bg-emerald-400" />{connection.name}<span>·</span>{connection.database}</p></div>
      <div className="ml-auto flex gap-2"><Button size="sm" variant="outline"><Sparkles />Explain</Button><Button size="sm" variant="outline"><Save />Save</Button><Button size="sm" onClick={runQuery} disabled={!sql.trim() || execution.isPending}><Play />{execution.isPending ? 'Running…' : 'Run query'}<kbd className="rounded bg-black/15 px-1.5 py-0.5 font-mono text-[length:var(--font-size-meta)]">⌘↵</kbd></Button></div>
    </header>
    <div className="grid min-h-0 min-w-0 flex-1 grid-cols-[minmax(0,1fr)] grid-rows-[minmax(230px,0.92fr)_minmax(260px,1.08fr)] overflow-hidden">
      <div className="min-h-0 min-w-0 overflow-hidden border-b border-border bg-[linear-gradient(180deg,color-mix(in_oklab,var(--surface)_74%,transparent),var(--background))]">
        <div className="flex h-[var(--toolbar-height)] items-center gap-2 border-b border-border px-4 text-[length:var(--font-size-meta)] text-muted-foreground"><Code2 className="size-4 text-primary" /><span className="font-medium text-foreground">SQL</span><Badge variant="outline" className="text-[length:var(--font-size-meta)]">PostgreSQL</Badge><span className="ml-auto font-mono">Ln {sql.split('\n').length}</span></div>
        <textarea value={sql} onChange={(event) => setSql(event.target.value)} onKeyDown={(event) => { if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') { event.preventDefault(); runQuery() } }} spellCheck={false} className="h-[calc(100%-var(--toolbar-height))] w-full resize-none bg-transparent px-6 py-5 font-mono text-[13px] leading-[24px] text-foreground outline-none selection:bg-primary/30" />
      </div>
      <div className="flex min-h-0 min-w-0 flex-col overflow-hidden">
        <div className="flex h-[var(--toolbar-height)] shrink-0 items-center border-b border-border bg-surface/70 px-4"><Tabs defaultValue="results"><TabsList><TabsTrigger value="results"><Table2 />Results</TabsTrigger><TabsTrigger value="history"><Clock3 />History</TabsTrigger></TabsList></Tabs>{result ? <div className="ml-auto flex items-center gap-2 text-[length:var(--font-size-meta)] text-muted-foreground"><span className="size-1.5 rounded-full bg-emerald-400" />Completed in {result.durationMs} ms<span>·</span>{result.rows.length} rows</div> : null}</div>
        {execution.isError ? <div className="m-4 rounded-lg border border-destructive/30 bg-destructive/10 p-4 text-[length:var(--font-size-ui)] text-destructive">{execution.error.message}</div> : null}
        {!result && !execution.isPending ? <div className="grid flex-1 place-items-center text-center"><div><div className="mx-auto grid size-12 place-items-center rounded-xl border border-border bg-surface text-primary"><Play className="size-5" /></div><p className="mt-3 font-medium">Ready to run</p><p className="mt-1 text-[length:var(--font-size-ui)] text-muted-foreground">Press ⌘ Enter or use Run query.</p></div></div> : null}
        {execution.isPending ? <div className="grid flex-1 place-items-center text-[length:var(--font-size-ui)] text-muted-foreground">Executing query against {connection.name}…</div> : null}
        {result ? <QueryResultGrid connectionId={connection.id} result={result} refreshing={execution.isPending} onRefresh={runQuery} /> : null}
      </div>
    </div>
  </section>
}

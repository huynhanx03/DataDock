import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertCircle, Braces, CheckCircle2, CircleStop, DatabaseZap, Play, Save } from 'lucide-react'
import type { Connection } from '@/entities/connection'
import type { DatabaseObject } from '@/entities/database-object'
import type { CreateSavedQueryInput, QueryExecutionKind, QueryResult, QueryResultMode, SavedQuery, TransactionAction, TransactionState } from '@/entities/query'
import { useDataDockGateway } from '@/app/providers'
import { GatewayError } from '@/data/gateway'
import { QueryResultViews } from '@/features/query/QueryResultViews'
import { QueryToolbar } from '@/features/query/QueryToolbar'
import { SqlEditor, type SqlCompletionItem } from '@/features/query/SqlEditor'
import { APP_CONFIG } from '@/shared/config/constants'
import { Badge, Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, EmptyState, Input, PropertyField, WorkspaceHeader, WorkspacePage } from '@/shared/ui'

type RunState = 'idle' | 'running' | 'success' | 'error' | 'cancelled'
type QueryPageProps = {
  connection: Connection
  sql: string
  onSqlChange: (sql: string) => void
  onBlockingStateChange?: (blocked: boolean) => void
  onCleanupRegistration?: (cleanup: (() => Promise<void>) | null) => void
  onOpenConnections?: () => void
}

const MAX_COMPLETION_ITEMS = 600

function createExecutionId() {
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const value = [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('')
  return `${value.slice(0, 8)}-${value.slice(8, 12)}-${value.slice(12, 16)}-${value.slice(16, 20)}-${value.slice(20)}`
}

function flattenCatalog(items: DatabaseObject[]): SqlCompletionItem[] {
  const output: SqlCompletionItem[] = []
  const seen = new Set<string>()
  const visit = (item: DatabaseObject) => {
    if (output.length >= MAX_COMPLETION_ITEMS) return
    const identity = `${item.kind}:${item.qualifiedName || item.name}`.toLowerCase()
    if (item.kind !== 'group' && !seen.has(identity)) {
      seen.add(identity)
      const kind: SqlCompletionItem['kind'] = item.kind === 'materialized-view' ? 'view' : item.kind === 'procedure' || item.kind === 'trigger' ? 'function' : item.kind === 'extension' || item.kind === 'sequence' ? 'keyword' : item.kind
      output.push({ label: item.qualifiedName || item.name, detail: `${item.kind}${item.dataType ? ` · ${item.dataType}` : ''}`, kind })
    }
    item.children?.forEach(visit)
  }
  items.forEach(visit)
  return output
}

function formatSql(value: string) {
  if (/\$[A-Za-z_][A-Za-z0-9_]*\$|\$\$/g.test(value)) return value.trimEnd()
  const clauses = ['SELECT', 'FROM', 'WHERE', 'LEFT JOIN', 'RIGHT JOIN', 'INNER JOIN', 'FULL JOIN', 'JOIN', 'GROUP BY', 'ORDER BY', 'HAVING', 'LIMIT', 'OFFSET', 'RETURNING', 'VALUES', 'SET']
  const transformCode = (code: string) => {
    let next = code.replace(/[ \t]+/g, ' ')
    for (const clause of clauses) next = next.replace(new RegExp(`\\b${clause.replace(' ', '\\s+')}\\b`, 'gi'), clause)
    for (const clause of clauses.slice(1).filter((item) => item !== 'JOIN')) next = next.replace(new RegExp(`\\s+${clause.replace(' ', '\\s+')}\\s+`, 'g'), `\n${clause} `)
    return next
  }
  let output = ''
  let code = ''
  let protectedText = ''
  let state: 'code' | 'single' | 'double' | 'backtick' | 'line-comment' | 'block-comment' = 'code'
  const flushCode = () => { output += transformCode(code); code = '' }
  const flushProtected = () => { output += protectedText; protectedText = '' }
  for (let index = 0; index < value.length; index += 1) {
    const char = value[index]
    const next = value[index + 1]
    if (state === 'code') {
      if (char === "'" || char === '"' || char === '`' || char === '-' && next === '-' || char === '/' && next === '*') {
        flushCode()
        if (char === "'") state = 'single'
        else if (char === '"') state = 'double'
        else if (char === '`') state = 'backtick'
        else if (char === '-') state = 'line-comment'
        else state = 'block-comment'
        protectedText += char
        if ((char === '-' && next === '-') || (char === '/' && next === '*')) {
          protectedText += next
          index += 1
        }
      } else code += char
      continue
    }
    protectedText += char
    if (state === 'line-comment' && char === '\n') {
      flushProtected()
      state = 'code'
    } else if (state === 'block-comment' && char === '*' && next === '/') {
      protectedText += next
      index += 1
      flushProtected()
      state = 'code'
    } else if (state === 'single' && char === "'" && next === "'") {
      protectedText += next
      index += 1
    } else if (state === 'double' && char === '"' && next === '"') {
      protectedText += next
      index += 1
    } else if ((state === 'single' && char === "'" || state === 'double' && char === '"' || state === 'backtick' && char === '`') && value[index - 1] !== '\\') {
      flushProtected()
      state = 'code'
    }
  }
  flushCode()
  flushProtected()
  return output.split('\n').map((line) => line.trimEnd()).join('\n').trim()
}

function canAnalyzeSql(value: string) {
  const normalized = value
    .replace(/^\s*(?:(?:--[^\n]*(?:\n|$))|(?:\/\*[\s\S]*?\*\/))*/g, '')
    .trimStart()
    .toLowerCase()
  if (/^(select|values|table|show|describe|desc)\b/.test(normalized)) return true
  if (!normalized.startsWith('with')) return false
  if (/\b(insert|replace|update|delete|merge)\b/.test(normalized)) return false
  return /\b(select|values|table)\b/.test(normalized)
}

export function QueryPage({ connection, sql, onSqlChange, onBlockingStateChange, onCleanupRegistration, onOpenConnections }: QueryPageProps) {
  const gateway = useDataDockGateway()
  const queryClient = useQueryClient()
  const [result, setResult] = useState<QueryResult>()
  const [resultMode, setResultMode] = useState<QueryResultMode>('table')
  const [runState, setRunState] = useState<RunState>('idle')
  const [executionError, setExecutionError] = useState('')
  const [elapsedSeconds, setElapsedSeconds] = useState(0)
  const [timeoutSeconds, setTimeoutSeconds] = useState<number>(APP_CONFIG.query.defaultTimeoutSeconds)
  const [selection, setSelection] = useState('')
  const [lastRun, setLastRun] = useState<{ sql: string; kind: QueryExecutionKind }>()
  const [saveOpen, setSaveOpen] = useState(false)
  const [saveTitle, setSaveTitle] = useState('')
  const [saveFolder, setSaveFolder] = useState('General')
  const [saveTags, setSaveTags] = useState('')
  const [savedNotice, setSavedNotice] = useState('')
  const [transaction, setTransaction] = useState<TransactionState>()
  const executionSequence = useRef(0)
  const activeExecutionId = useRef<string | undefined>(undefined)
  const activeController = useRef<AbortController | undefined>(undefined)
  const transactionController = useRef<AbortController | undefined>(undefined)
  const transactionRef = useRef<TransactionState | undefined>(undefined)
  const executionStartedAt = useRef(0)
  const blockingCallbackRef = useRef(onBlockingStateChange)
  const cancelledSequences = useRef(new Set<number>())
  const cancellationFailures = useRef(new Map<number, string>())
  const catalog = useQuery({ queryKey: ['catalog', connection.id], queryFn: ({ signal }) => gateway.listCatalog(connection.id, signal) })
  const executable = connection.status === 'connected'
  const cleanupSession = useCallback(async () => {
    const sequence = executionSequence.current
    const executionId = activeExecutionId.current
    cancelledSequences.current.add(sequence)
    if (executionId) {
      const cancellationRequest = gateway.cancelQuery(executionId)
      activeController.current?.abort('navigation')
      try {
        await cancellationRequest
      } catch (error) {
        if (!(error instanceof GatewayError && error.code === 'not_found')) {
          cancellationFailures.current.set(sequence, error instanceof Error ? error.message : 'Query cancellation failed')
        }
      }
    }
    activeController.current?.abort('navigation')
    activeController.current = undefined
    activeExecutionId.current = undefined
    transactionController.current?.abort('navigation')
    transactionController.current = undefined
    const activeTransaction = transactionRef.current
    if (activeTransaction?.state === 'active') {
      await gateway.transactionAction(activeTransaction.id, 'rollback')
      transactionRef.current = undefined
      setTransaction(undefined)
    }
    blockingCallbackRef.current?.(false)
  }, [gateway])

  const completions = useMemo(() => {
    const catalogItems = flattenCatalog(catalog.data?.databases ?? [])
    const keywords: SqlCompletionItem[] = ['SELECT', 'FROM', 'WHERE', 'JOIN', 'LEFT JOIN', 'GROUP BY', 'ORDER BY', 'LIMIT', 'INSERT INTO', 'UPDATE', 'DELETE FROM', 'EXPLAIN ANALYZE'].map((label) => ({ label, detail: 'SQL keyword', kind: 'keyword' }))
    return [...catalogItems, ...keywords]
  }, [catalog.data])

  useEffect(() => {
    if (runState !== 'running') return
    const updateElapsed = () => setElapsedSeconds(Math.max(0, Math.floor((performance.now() - executionStartedAt.current) / 1_000)))
    updateElapsed()
    const timer = window.setInterval(updateElapsed, 250)
    return () => window.clearInterval(timer)
  }, [runState])

  useEffect(() => { blockingCallbackRef.current = onBlockingStateChange }, [onBlockingStateChange])
  useEffect(() => {
    onCleanupRegistration?.(cleanupSession)
    return () => onCleanupRegistration?.(null)
  }, [cleanupSession, onCleanupRegistration])
  useEffect(() => { transactionRef.current = transaction }, [transaction])
  useEffect(() => {
    const executionId = activeExecutionId.current
    if (executionId) void gateway.cancelQuery(executionId).catch(() => undefined)
    activeController.current?.abort('navigation')
    activeExecutionId.current = undefined
    executionSequence.current += 1
    setResult(undefined)
    setRunState('idle')
    setExecutionError('')
    setElapsedSeconds(0)
    setLastRun(undefined)
    setTransaction(undefined)
    transactionRef.current = undefined
    return () => {
      const pendingExecutionId = activeExecutionId.current
      if (pendingExecutionId) void gateway.cancelQuery(pendingExecutionId).catch(() => undefined)
      activeController.current?.abort('navigation')
      activeController.current = undefined
      activeExecutionId.current = undefined
      transactionController.current?.abort('navigation')
      transactionController.current = undefined
      const activeTransaction = transactionRef.current
      transactionRef.current = undefined
      if (activeTransaction?.state === 'active') void gateway.transactionAction(activeTransaction.id, 'rollback').catch(() => undefined)
      blockingCallbackRef.current?.(false)
    }
  }, [connection.id, gateway])

  const execution = useMutation({
    mutationFn: async ({ sqlText, executionId, sequence, controller, kind }: { sqlText: string; executionId: string; sequence: number; controller: AbortController; kind: QueryExecutionKind }) => {
      const input = { executionId, connectionId: connection.id, sql: sqlText, timeoutSeconds, transactionId: transaction?.state === 'active' ? transaction.id : undefined }
      const result = kind === 'explain' || kind === 'explain-analyze'
        ? await gateway.explainQuery({ ...input, analyze: kind === 'explain-analyze' }, controller.signal)
        : await gateway.executeQuery(input, controller.signal)
      return { sequence, kind, result }
    },
    onMutate: () => {
      executionStartedAt.current = performance.now()
      setRunState('running')
      setExecutionError('')
      setElapsedSeconds(0)
    },
    onSuccess: (completed) => {
      if (completed.sequence !== executionSequence.current) return
      setResult(completed.result)
      setResultMode(completed.kind === 'explain' || completed.kind === 'explain-analyze' ? 'plan' : 'table')
      setRunState('success')
    },
    onError: (error, variables) => {
      if (variables.sequence !== executionSequence.current) return
      const code = error instanceof GatewayError ? error.code : ''
      const cancellationFailure = cancellationFailures.current.get(variables.sequence)
      if (cancellationFailure) {
        setRunState('error')
        setExecutionError(`The cancellation endpoint failed: ${cancellationFailure}. The browser request was stopped.`)
      } else if (cancelledSequences.current.has(variables.sequence) || code === 'query_cancelled') {
        setRunState('cancelled')
        setExecutionError('Query cancelled by user. No result was applied.')
      } else if (code === 'query_timeout') {
        setRunState('error')
        setExecutionError(`Query exceeded the ${timeoutSeconds} second timeout.`)
      } else if (code === 'request_timeout') {
        setRunState('error')
        setExecutionError('DataDock API did not respond before the request deadline.')
      } else {
        setRunState('error')
        setExecutionError(error instanceof Error ? error.message : 'Query execution failed')
      }
    },
    onSettled: (_, __, variables) => {
      if (variables.sequence === executionSequence.current) {
        activeController.current = undefined
        activeExecutionId.current = undefined
      }
      queryClient.invalidateQueries({ queryKey: ['query-history'] })
    },
  })

  const cancellation = useMutation({ mutationFn: (executionId: string) => gateway.cancelQuery(executionId) })

  const saveQuery = useMutation({
    mutationFn: (input: CreateSavedQueryInput) => gateway.createSavedQuery(input),
    onSuccess: (created) => {
      queryClient.setQueryData<SavedQuery[]>(['saved-queries'], (current) => current ? [created, ...current] : [created])
      queryClient.invalidateQueries({ queryKey: ['saved-queries'] })
      setSaveOpen(false)
      setSavedNotice(`Saved as “${created.title}”`)
      window.setTimeout(() => setSavedNotice(''), 2200)
    },
  })

  const beginTransaction = useMutation({
    mutationFn: () => {
      const controller = new AbortController()
      transactionController.current = controller
      return gateway.beginTransaction(connection.id, controller.signal)
    },
    onSuccess: setTransaction,
    onSettled: () => { transactionController.current = undefined },
  })
  const transactionAction = useMutation({
    mutationFn: ({ action, name }: { action: TransactionAction; name?: string }) => {
      const controller = new AbortController()
      transactionController.current = controller
      return gateway.transactionAction(transaction!.id, action, name, controller.signal)
    },
    onSuccess: (next) => setTransaction(next.state === 'active' ? next : undefined),
    onSettled: () => { transactionController.current = undefined },
  })

  const analyzeSql = selection || sql
  const explainAnalyzeEnabled = canAnalyzeSql(analyzeSql)

  useEffect(() => {
    blockingCallbackRef.current?.(runState === 'running' || transaction?.state === 'active' || beginTransaction.isPending || transactionAction.isPending)
  }, [beginTransaction.isPending, runState, transaction?.state, transactionAction.isPending])

  function run(sqlText: string, kind: QueryExecutionKind) {
    const statement = sqlText.trim()
    if (!statement || runState === 'running' || !executable) return
    executionSequence.current += 1
    const sequence = executionSequence.current
    const executionId = createExecutionId()
    const controller = new AbortController()
    activeExecutionId.current = executionId
    activeController.current = controller
    cancelledSequences.current.delete(sequence)
    cancellationFailures.current.clear()
    setLastRun({ sql: statement, kind })
    execution.mutate({ sqlText: statement, executionId, sequence, controller, kind })
  }

  async function cancel() {
    const sequence = executionSequence.current
    const executionId = activeExecutionId.current
    const controller = activeController.current
    if (!executionId || !controller || cancellation.isPending) return
    cancelledSequences.current.add(sequence)
    const cancellationRequest = cancellation.mutateAsync(executionId)
    controller.abort('cancelled')
    try {
      await cancellationRequest
    } catch (error) {
      if (!(error instanceof GatewayError && error.code === 'not_found')) {
        const message = error instanceof Error ? error.message : 'Query cancellation failed'
        cancellationFailures.current.set(sequence, message)
        if (executionSequence.current === sequence) {
          setRunState('error')
          setExecutionError(`The cancellation endpoint failed: ${message}. The browser request was stopped.`)
        }
      }
    } finally {
      if (activeExecutionId.current === executionId) activeExecutionId.current = undefined
    }
  }

  function openSaveDialog() {
    const firstLine = sql.split('\n').find((line) => line.trim())?.trim().slice(0, 48) ?? ''
    setSaveTitle(firstLine.replace(/^(select|with|update|insert into|delete from)\s+/i, '') || 'Untitled query')
    setSaveOpen(true)
  }

  function submitSave() {
    if (!saveTitle.trim() || !sql.trim()) return
    saveQuery.mutate({ connectionId: connection.id, title: saveTitle.trim(), folder: saveFolder.trim() || 'General', tags: saveTags.split(',').map((tag) => tag.trim().replace(/^#/, '')).filter(Boolean), sql })
  }

  return <WorkspacePage>
    <WorkspaceHeader compact icon={<Braces />} eyebrow="SQL Studio" title="Query workspace" description={<span className="flex items-center gap-2"><span className={`size-1.5 rounded-full ${connection.status === 'connected' ? 'bg-emerald-400' : connection.status === 'connecting' ? 'animate-pulse bg-amber-400' : connection.status === 'error' ? 'bg-destructive' : 'bg-muted-foreground'}`} />{connection.name}<span>·</span>{connection.database}<span>·</span>{connection.status}</span>} meta={<><Badge variant={gateway.source === 'mock' ? 'accent' : 'success'}>{gateway.source === 'mock' ? 'Demo data' : 'Live data'}</Badge>{connection.readOnly ? <Badge variant="warning">Read only</Badge> : null}{savedNotice ? <span className="inline-flex items-center gap-1 text-[length:var(--font-size-meta)] text-emerald-400"><CheckCircle2 className="size-3.5" />{savedNotice}</span> : null}</>} />
    <QueryToolbar
      running={runState === 'running'}
      executable={executable}
      elapsedSeconds={elapsedSeconds}
      timeoutSeconds={timeoutSeconds}
      hasSelection={Boolean(selection)}
      transaction={transaction}
      transactionBusy={beginTransaction.isPending || transactionAction.isPending}
      onTimeoutChange={setTimeoutSeconds}
      onRun={() => run(sql, 'query')}
      onRunSelection={() => run(selection, 'selection')}
      onCancel={cancel}
      onExplain={() => run(selection || sql, 'explain')}
      canExplainAnalyze={explainAnalyzeEnabled}
      onExplainAnalyze={() => run(analyzeSql, 'explain-analyze')}
      onSave={openSaveDialog}
      onBeginTransaction={() => beginTransaction.mutate()}
      onCommit={() => transactionAction.mutate({ action: 'commit' })}
      onRollback={() => transactionAction.mutate({ action: 'rollback' })}
      onSavepoint={() => transactionAction.mutate({ action: 'savepoint', name: `sp_${(transaction?.savepoints.length ?? 0) + 1}` })}
      onRollbackTo={(name) => transactionAction.mutate({ action: 'rollback_to', name })}
    />
    {!executable ? <div role="status" className="flex shrink-0 items-center gap-2 border-b border-warning/30 bg-warning/10 px-4 py-2 text-[length:var(--font-size-ui)] text-amber-300"><AlertCircle className="size-3.5 shrink-0" /><span className="min-w-0 flex-1">{connection.name} is {connection.status}. The SQL is safe to edit, but execution and transactions are unavailable until it connects.</span>{onOpenConnections ? <Button size="xs" variant="outline" onClick={onOpenConnections}>Open connections</Button> : null}</div> : null}
    {beginTransaction.isError || transactionAction.isError ? <div role="alert" className="flex shrink-0 items-center gap-2 border-b border-destructive/30 bg-destructive/10 px-4 py-2 text-[length:var(--font-size-ui)] text-destructive"><AlertCircle className="size-3.5 shrink-0" /><span className="min-w-0 flex-1 truncate">{(transactionAction.error ?? beginTransaction.error)?.message ?? 'Transaction action failed'}</span><Button size="xs" variant="ghost" onClick={() => { beginTransaction.reset(); transactionAction.reset() }}>Dismiss</Button></div> : null}
    <div className="grid min-h-0 min-w-0 flex-1 grid-cols-[minmax(0,1fr)] grid-rows-[minmax(230px,0.92fr)_minmax(250px,1.08fr)] overflow-hidden">
      <div className="min-h-0 min-w-0 overflow-hidden border-b border-border">
        <SqlEditor value={sql} dialect={connection.engine === 'postgresql' ? 'PostgreSQL' : connection.engine === 'mariadb' ? 'MariaDB' : 'MySQL'} completionItems={completions} onChange={onSqlChange} onRun={() => run(sql, 'query')} onRunSelection={(value) => run(value, 'selection')} onFormat={() => onSqlChange(formatSql(sql))} onSelectionChange={setSelection} />
      </div>
      <div className="flex min-h-0 min-w-0 flex-col overflow-hidden bg-background">
        {runState === 'running' ? <EmptyState className="flex-1" icon={<DatabaseZap className="animate-pulse" />} title={`Running on ${connection.name}`} description={`Executing with a ${timeoutSeconds}s timeout${transaction?.state === 'active' ? ' inside the active transaction' : ''}.`} actions={<Button size="sm" variant="destructive" onClick={cancel}><CircleStop />Cancel query</Button>} /> : null}
        {(runState === 'error' || runState === 'cancelled') && !result ? <EmptyState className="flex-1" icon={<AlertCircle />} title={runState === 'cancelled' ? 'Query cancelled' : 'Execution failed'} description={executionError} actions={<Button size="sm" variant="outline" onClick={() => run(lastRun?.sql ?? sql, lastRun?.kind ?? 'query')}><Play />Run again</Button>} /> : null}
        {(runState === 'error' || runState === 'cancelled') && result ? <div role="alert" className={`flex shrink-0 items-center gap-2 border-b px-3 py-2 text-[length:var(--font-size-ui)] ${runState === 'cancelled' ? 'border-warning/30 bg-warning/10 text-amber-300' : 'border-destructive/30 bg-destructive/10 text-destructive'}`}><AlertCircle className="size-4 shrink-0" /><span className="min-w-0 flex-1 truncate">{executionError} Previous result is still displayed below.</span><Button size="xs" variant="outline" onClick={() => run(lastRun?.sql ?? sql, lastRun?.kind ?? 'query')}><Play />Run again</Button></div> : null}
        {runState !== 'running' && result ? <div className="min-h-0 flex-1"><QueryResultViews connectionId={connection.id} result={result} mode={resultMode} refreshing={execution.isPending} canRefresh={executable} onModeChange={setResultMode} onRefresh={() => run(lastRun?.sql ?? sql, lastRun?.kind ?? 'query')} /></div> : null}
        {runState === 'idle' && !result ? <EmptyState className="flex-1" icon={<Play />} title={executable ? 'Ready to run' : 'Connection required'} description={executable ? 'Use ⌘ Enter to run the current statement. Select SQL first to execute only that selection.' : `Connect ${connection.name} before executing this statement.`} actions={executable ? <Button size="sm" onClick={() => run(sql, 'query')}><Play />Run query</Button> : onOpenConnections ? <Button size="sm" variant="outline" onClick={onOpenConnections}>Open connections</Button> : undefined} /> : null}
      </div>
    </div>
    <Dialog open={saveOpen} onOpenChange={setSaveOpen}>
      <DialogContent>
        <DialogHeader><DialogTitle>Save query</DialogTitle><DialogDescription>Add this SQL to the reusable query library. It will appear immediately in Saved Queries.</DialogDescription></DialogHeader>
        <div className="space-y-4">
          <PropertyField label="Title" htmlFor="saved-query-title" required error={saveTitle.trim() ? undefined : 'Enter a title'}><Input id="saved-query-title" autoFocus value={saveTitle} onChange={(event) => setSaveTitle(event.target.value)} placeholder="Monthly revenue by channel" /></PropertyField>
          <PropertyField label="Folder" htmlFor="saved-query-folder" description="A new folder is created automatically when needed."><Input id="saved-query-folder" value={saveFolder} onChange={(event) => setSaveFolder(event.target.value)} placeholder="General" /></PropertyField>
          <PropertyField label="Tags" htmlFor="saved-query-tags" description="Separate tags with commas."><Input id="saved-query-tags" value={saveTags} onChange={(event) => setSaveTags(event.target.value)} placeholder="revenue, monthly, dashboard" /></PropertyField>
          <div className="max-h-32 overflow-auto rounded-lg border border-border bg-[#070a11] p-3 font-mono text-[11px] leading-5 text-muted-foreground">{sql}</div>
          {saveQuery.isError ? <p role="alert" className="text-[length:var(--font-size-ui)] text-destructive">{saveQuery.error.message}</p> : null}
        </div>
        <DialogFooter><Button variant="outline" onClick={() => setSaveOpen(false)}>Cancel</Button><Button disabled={!saveTitle.trim() || !sql.trim() || saveQuery.isPending} onClick={submitSave}><Save />{saveQuery.isPending ? 'Saving…' : 'Save query'}</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </WorkspacePage>
}

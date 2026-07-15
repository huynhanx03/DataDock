import { Activity, BookmarkPlus, ChevronDown, CircleStop, Clock3, GitCommitHorizontal, Play, RotateCcw, Save, Sparkles, TimerReset } from 'lucide-react'
import type { TransactionState } from '@/entities/query'
import { Badge, Button, DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger, Select, Tooltip, TooltipContent, TooltipTrigger } from '@/shared/ui'

type QueryToolbarProps = {
  running: boolean
  executable: boolean
  elapsedSeconds: number
  timeoutSeconds: number
  hasSelection: boolean
  transaction?: TransactionState
  transactionBusy?: boolean
  onTimeoutChange: (value: number) => void
  onRun: () => void
  onRunSelection: () => void
  onCancel: () => void
  onExplain: () => void
  canExplainAnalyze: boolean
  onExplainAnalyze: () => void
  onSave: () => void
  onBeginTransaction: () => void
  onCommit: () => void
  onRollback: () => void
  onSavepoint: () => void
  onRollbackTo: (name: string) => void
}

export function QueryToolbar({ running, executable, elapsedSeconds, timeoutSeconds, hasSelection, transaction, transactionBusy, onTimeoutChange, onRun, onRunSelection, onCancel, onExplain, canExplainAnalyze, onExplainAnalyze, onSave, onBeginTransaction, onCommit, onRollback, onSavepoint, onRollbackTo }: QueryToolbarProps) {
  const activeTransaction = transaction?.state === 'active'

  return <div className="relative flex min-h-[var(--toolbar-height)] shrink-0 flex-wrap items-center gap-1.5 border-b border-border bg-surface/55 px-3 py-1.5">
    <DropdownMenu>
      <DropdownMenuTrigger asChild><Button size="sm" variant="outline" disabled={!executable}><Sparkles />Explain<ChevronDown className="ml-0.5 size-3" /></Button></DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        <DropdownMenuItem disabled={running || !executable} onSelect={onExplain}><Sparkles />Explain plan<span className="ml-auto font-mono text-[10px] text-muted-foreground">cost</span></DropdownMenuItem>
        <DropdownMenuItem disabled={running || !executable || !canExplainAnalyze} onSelect={onExplainAnalyze}><Activity />Explain analyze<span className="ml-auto font-mono text-[10px] text-muted-foreground">{canExplainAnalyze ? 'runtime' : 'read-only only'}</span></DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
    <Button size="sm" variant="outline" onClick={onSave}><Save />Save</Button>
    <div className="mx-0.5 h-5 w-px bg-border" />
    <Tooltip><TooltipTrigger asChild><div className="flex items-center gap-1.5"><Clock3 className="size-3.5 text-muted-foreground" /><Select aria-label="Query timeout" value={timeoutSeconds} disabled={running} onChange={(event) => onTimeoutChange(Number(event.target.value))} className="h-[var(--control-height-sm)] w-[5.5rem] pr-7 text-[length:var(--font-size-meta)]"><option value={5}>5 sec</option><option value={15}>15 sec</option><option value={30}>30 sec</option><option value={60}>60 sec</option></Select></div></TooltipTrigger><TooltipContent>Cancel the query after this timeout</TooltipContent></Tooltip>
    <div className="ml-auto flex items-center gap-1.5">
      {activeTransaction ? <div className="mr-1 flex flex-wrap items-center gap-1"><Badge variant="warning" className="h-6"><span className="size-1.5 rounded-full bg-amber-400" />Transaction open</Badge><Button size="xs" variant="ghost" disabled={transactionBusy || running} onClick={onSavepoint}><BookmarkPlus />Savepoint</Button>{transaction.savepoints.length ? <DropdownMenu><DropdownMenuTrigger asChild><Button size="xs" variant="ghost" disabled={transactionBusy || running}><RotateCcw />Savepoints {transaction.savepoints.length}<ChevronDown className="size-3" /></Button></DropdownMenuTrigger><DropdownMenuContent align="end">{[...transaction.savepoints].reverse().map((name) => <DropdownMenuItem key={name} onSelect={() => onRollbackTo(name)}><RotateCcw />Rollback to <span className="ml-auto font-mono text-[10px]">{name}</span></DropdownMenuItem>)}</DropdownMenuContent></DropdownMenu> : null}<Button size="xs" variant="ghost" disabled={transactionBusy || running} onClick={onRollback}><RotateCcw />Rollback</Button><Button size="xs" variant="outline" disabled={transactionBusy || running} onClick={onCommit}><GitCommitHorizontal />Commit</Button></div> : <Button size="xs" variant="ghost" disabled={transactionBusy || running || !executable} onClick={onBeginTransaction}><TimerReset />Begin transaction</Button>}
      {running ? <Button size="sm" variant="destructive" onClick={onCancel}><CircleStop />Cancel<span className="font-mono text-[10px] opacity-80">{elapsedSeconds}s</span></Button> : <div className="flex overflow-hidden rounded-md shadow-control"><Button size="sm" className="rounded-r-none shadow-none" disabled={!executable} onClick={onRun}><Play />Run query<kbd className="rounded bg-black/15 px-1 py-0.5 font-mono text-[10px]">⌘↵</kbd></Button><DropdownMenu><DropdownMenuTrigger asChild><Button aria-label="Run options" size="icon-sm" disabled={!executable} className="rounded-l-none border-l border-primary-foreground/15 px-0 shadow-none"><ChevronDown className="size-3.5" /></Button></DropdownMenuTrigger><DropdownMenuContent align="end"><DropdownMenuItem disabled={!executable} onSelect={onRun}><Play />Run all<span className="ml-auto font-mono text-[10px] text-muted-foreground">⌘↵</span></DropdownMenuItem><DropdownMenuItem disabled={!hasSelection || !executable} onSelect={onRunSelection}><Play />Run selection<span className="ml-auto text-[10px] text-muted-foreground">selected SQL</span></DropdownMenuItem><DropdownMenuSeparator /><DropdownMenuItem disabled={!executable} onSelect={onExplain}><Sparkles />Explain plan</DropdownMenuItem><DropdownMenuItem disabled={!executable || !canExplainAnalyze} onSelect={onExplainAnalyze}><Activity />Explain analyze<span className="ml-auto text-[10px] text-muted-foreground">{canExplainAnalyze ? 'runtime' : 'read-only only'}</span></DropdownMenuItem></DropdownMenuContent></DropdownMenu></div>}
    </div>
    {running ? <div className="absolute inset-x-0 bottom-0 h-0.5 overflow-hidden bg-primary/10"><div className="h-full w-1/3 bg-primary [animation:datadock-indeterminate_1.15s_ease-in-out_infinite]" /></div> : null}
  </div>
}

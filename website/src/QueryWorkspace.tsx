import { useMemo, useState } from 'react'
import { AlertCircle, CheckCircle2, Clock3, Code2, Copy, Database, FileJson2, History, LoaderCircle, Play, RotateCcw, Search, Table2, X, Save, BarChart3, GitBranch } from 'lucide-react'
import { datadockApi, type Connection, type QueryHistoryItem, type QueryResult, type SavedQuery, type TransactionState } from './api'

type Props = { connection?: Connection }

const initialSQL = 'SELECT\n  id,\n  email,\n  full_name,\n  role,\n  created_at\nFROM users\nORDER BY created_at DESC\nLIMIT 100;'

function valueOf(value: unknown) {
  if (value === null) return 'NULL'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function normalizeResult(result: QueryResult) {
  const raw = result as QueryResult & { columns?: Array<QueryResult['columns'][number] | string>; rows?: Array<unknown[] | Record<string, unknown>> }
  const columns = (raw.columns || []).map((column) => typeof column === 'string' ? { name: column } : column)
  const rows = (raw.rows || []).map((row) => Array.isArray(row) ? row : columns.map((column) => row[column.name]))
  return { ...result, columns, rows }
}

function ResultGrid({ result }: { result: QueryResult }) {
  const normalized = normalizeResult(result)
  if (!normalized.columns.length) return <div className="query-empty-result"><CheckCircle2 size={21} /><strong>Query completed</strong><span>{normalized.message || `${normalized.rowsAffected || 0} row${normalized.rowsAffected === 1 ? '' : 's'} affected`}</span></div>
  return <div className="query-grid-wrap"><table className="query-grid"><thead><tr><th className="row-number">#</th>{normalized.columns.map((column) => <th key={column.name}><span className="query-type">{column.type || 'ANY'}</span>{column.name}</th>)}</tr></thead><tbody>{normalized.rows.map((row, rowIndex) => <tr key={rowIndex}><td className="row-number">{rowIndex + 1}</td>{row.map((value, columnIndex) => <td key={columnIndex} className={value === null ? 'is-null' : ''}>{valueOf(value)}</td>)}</tr>)}</tbody></table></div>
}
function ResultChart({ result }: { result: QueryResult }) { const n=normalizeResult(result); const bars=n.rows.map((row,i)=>Number(row.find(v=>typeof v==='number')??0)).slice(0,30); const max=Math.max(1,...bars); return <div className="query-chart">{bars.map((value,i)=><div key={i}><span style={{height:`${Math.max(3,value/max*100)}%`}}/><small>{i+1}</small></div>)}</div> }

export function QueryWorkspace({ connection }: Props) {
  const [sql, setSQL] = useState(initialSQL)
  const [result, setResult] = useState<QueryResult>()
  const [error, setError] = useState('')
  const [running, setRunning] = useState(false)
  const [view, setView] = useState<'table' | 'json' | 'chart'>('table')
  const [historyOpen, setHistoryOpen] = useState(true)
  const [history, setHistory] = useState<QueryHistoryItem[]>([])
  const [historySearch, setHistorySearch] = useState('')
  const [transaction, setTransaction] = useState<TransactionState>()
  const [saved, setSaved] = useState<SavedQuery[]>([])
  const displayRows = useMemo(() => normalizeResult(result || { columns: [], rows: [] }).rows, [result])
  const displayColumns = useMemo(() => normalizeResult(result || { columns: [], rows: [] }).columns, [result])
  const filteredHistory = history.filter((item) => item.sqlText.toLowerCase().includes(historySearch.toLowerCase()))

  async function execute() {
    if (!connection || !sql.trim() || running) return
    const startedAt = Date.now()
    setRunning(true); setError('')
    try {
      const response = await datadockApi.executeQuery({ connectionId: connection.id, sql, timeoutSeconds: 30, transactionId: transaction?.id } as {connectionId:string;sql:string;timeoutSeconds:number;transactionId?:string})
      const normalized = normalizeResult(response)
      setResult(normalized)
		setHistory((current) => [{ id: crypto.randomUUID(), sqlText: sql.trim(), status: 'success' as const, durationMs: normalized.durationMs || Date.now() - startedAt, rowCount: normalized.rows.length || normalized.rowsAffected || 0, executedAt: new Date().toISOString() }, ...current].slice(0, 30))
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : 'Unable to execute the query'
      setError(message)
		setHistory((current) => [{ id: crypto.randomUUID(), sqlText: sql.trim(), status: 'error' as const, durationMs: Date.now() - startedAt, rowCount: 0, executedAt: new Date().toISOString() }, ...current].slice(0, 30))
    } finally { setRunning(false) }
  }

  async function copyResults() {
    if (!result) return
    const content = [displayColumns.map((column) => column.name).join('\t'), ...displayRows.map((row) => row.map(valueOf).join('\t'))].join('\n')
    await navigator.clipboard.writeText(content)
  }
  async function toggleTransaction() { if(!connection)return; if(transaction){await datadockApi.transactionAction(transaction.id,'commit');setTransaction(undefined)}else{setTransaction(await datadockApi.beginTransaction(connection.id))} }
  async function rollback() { if(!transaction)return;await datadockApi.transactionAction(transaction.id,'rollback');setTransaction(undefined) }
  async function saveQuery(){if(!sql.trim())return;const title=window.prompt('Saved query name',sql.trim().split('\n')[0].slice(0,60));if(!title)return;const item=await datadockApi.createSavedQuery({connectionId:connection?.id,folder:'General',title,sql,tags:[]});setSaved(items=>[item,...items])}

  return <section className="query-workspace">
    <div className="query-heading"><div><div className="eyebrow">SQL WORKSPACE</div><h1>Query 1</h1><p>{connection ? <><span className="query-online" />{connection.name} · {connection.database || connection.host}{transaction && ' · transaction active'}</> : 'Select a connection to start querying'}</p></div><div className="query-heading-actions"><button className="button secondary" onClick={() => void saveQuery()}><Save size={15}/>Save</button><button className="button secondary" disabled={!connection} onClick={() => void toggleTransaction()}>{transaction ? 'Commit' : 'Begin'}</button>{transaction && <button className="button secondary" onClick={() => void rollback()}>Rollback</button>}<button className="button secondary" onClick={() => setSQL('')}><RotateCcw size={15} />Clear</button><button className="button primary query-run" disabled={!connection || !sql.trim() || running} onClick={() => void execute()}>{running ? <LoaderCircle size={15} className="spin" /> : <Play size={15} fill="currentColor" />}{running ? 'Running…' : 'Run query'}<kbd>⌘ ↵</kbd></button></div></div>
    <div className="query-layout">
      <div className="query-main">
        <div className="query-editor-card"><header className="query-editor-toolbar"><div><Code2 size={15} /><span>SQL</span><span className="query-editor-connection"><Database size={12} />{connection?.engine || 'No connection'}</span></div><span>Ln {sql.split('\n').length}, Col {sql.length - sql.lastIndexOf('\n')}</span></header><div className="query-editor"><div className="line-numbers">{sql.split('\n').map((_, index) => <span key={index}>{index + 1}</span>)}</div><textarea aria-label="SQL query" spellCheck="false" value={sql} onChange={(event) => setSQL(event.target.value)} onKeyDown={(event) => { if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') { event.preventDefault(); void execute() } }} placeholder="Write a SQL query…" /></div><footer><span><span className="query-keyword">⌘ Enter</span> to run</span><span>{connection?.readOnly ? 'Read-only connection' : 'Changes are allowed'}</span></footer></div>
        <div className="query-result-card"><header className="query-result-toolbar"><div className="query-result-title"><Table2 size={15} /><strong>Results</strong>{result && <span className="query-count">{displayRows.length} rows</span>}</div><div className="query-result-actions"><button className={view === 'table' ? 'result-view active' : 'result-view'} onClick={() => setView('table')}><Table2 size={14} />Table</button><button className={view === 'json' ? 'result-view active' : 'result-view'} onClick={() => setView('json')}><FileJson2 size={14} />JSON</button><button className={view === 'chart' ? 'result-view active' : 'result-view'} onClick={() => setView('chart')}><BarChart3 size={14}/>Chart</button><button className="result-icon" title="Copy results" onClick={() => void copyResults()} disabled={!result}><Copy size={14} /></button></div></header>{running && <div className="query-running"><LoaderCircle size={17} className="spin" /><span>Executing query against {connection?.name}…</span></div>}{error && <div className="query-error"><AlertCircle size={17} /><div><strong>Query failed</strong><span>{error}</span></div><button onClick={() => setError('')}><X size={15} /></button></div>}{!running && !error && !result && <div className="query-placeholder"><div><Play size={19} /></div><strong>Ready to run</strong><span>Write a query, then press <kbd>⌘ Enter</kbd> or Run query.</span></div>}{!running && !error && result && (view === 'table' ? <ResultGrid result={result} /> : view === 'chart' ? <ResultChart result={result}/> : <pre className="query-json">{JSON.stringify(displayRows.map((row) => Object.fromEntries(displayColumns.map((column, index) => [column.name, row[index]]))), null, 2)}</pre>)}{result && <footer className="query-result-footer"><span><CheckCircle2 size={13} />Completed in {result.durationMs || 0} ms</span><span>{displayRows.length || result.rowsAffected || 0} row{(displayRows.length || result.rowsAffected || 0) === 1 ? '' : 's'} returned</span></footer>}</div>
      </div>
      <aside className={historyOpen ? 'query-history' : 'query-history closed'}><header><button className="query-history-title" onClick={() => setHistoryOpen(!historyOpen)}><History size={15} /><strong>Recent queries</strong></button><button className="result-icon" onClick={() => setHistory([])} title="Clear history"><X size={14} /></button></header>{historyOpen && <><div className="query-history-search"><Search size={13} /><input value={historySearch} onChange={(event) => setHistorySearch(event.target.value)} placeholder="Search history" /></div><div className="query-history-list">{!filteredHistory.length && <div className="history-empty"><Clock3 size={18} /><span>Executed queries appear here.</span></div>}{filteredHistory.map((item) => <button key={item.id} className="history-item" onClick={() => setSQL(item.sqlText)}><span className={item.status === 'success' ? 'history-status success' : 'history-status error'}>{item.status === 'success' ? <CheckCircle2 size={13} /> : <AlertCircle size={13} />}</span><div><strong>{item.sqlText.replace(/\s+/g, ' ').slice(0, 52)}</strong><small>{item.durationMs} ms · {item.rowCount} rows</small></div></button>)}</div></>}</aside>
    </div>
  </section>
}

import { useEffect, useMemo, useState } from 'react'
import { Check, Clipboard, Code2, Download, FileCode2, GitCompareArrows, Sparkles } from 'lucide-react'
import type { DatabaseEngine } from '@/entities/connection'
import { Badge, Button, EmptyState, Tabs, TabsContent, TabsList, TabsTrigger, WorkspacePanel } from '@/shared/ui'

type DdlPanelProps = {
  table: string
  ddl: string
  generatedSql: string
  engine: DatabaseEngine
  source: 'mock' | 'api'
  loading?: boolean
}

function SqlBlock({ sql }: { sql: string }) {
  const lines = useMemo(() => sql.split('\n'), [sql])
  return <div className="grid min-h-0 grid-cols-[3.25rem_minmax(0,1fr)] overflow-auto bg-[linear-gradient(180deg,color-mix(in_oklab,var(--surface)_45%,transparent),transparent)]"><div aria-hidden="true" className="select-none border-r border-grid-line bg-grid-header/45 py-4 text-right font-mono text-[length:var(--font-size-meta)] leading-6 text-muted-foreground/55">{lines.map((_, index) => <span key={index} className="block pr-3">{index + 1}</span>)}</div><pre className="m-0 min-w-max p-4 font-mono text-[13px] leading-6 text-foreground"><code>{sql}</code></pre></div>
}

export function DdlPanel({ table, ddl, generatedSql, engine, source, loading }: DdlPanelProps) {
  const [copied, setCopied] = useState<'base' | 'draft'>()
  useEffect(() => {
    if (!copied) return
    const timeout = window.setTimeout(() => setCopied(undefined), 1600)
    return () => window.clearTimeout(timeout)
  }, [copied])

  async function copySql(kind: 'base' | 'draft', sql: string) {
    await navigator.clipboard.writeText(sql)
    setCopied(kind)
  }

  function downloadSql(sql: string) {
    const file = new Blob([sql], { type: 'text/sql;charset=utf-8' })
    const url = URL.createObjectURL(file)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `${table.replaceAll('.', '_')}.sql`
    anchor.click()
    URL.revokeObjectURL(url)
  }

  if (loading) return <div className="grid min-h-80 place-items-center text-[length:var(--font-size-ui)] text-muted-foreground">Loading table definition…</div>

  return <Tabs defaultValue={generatedSql ? 'draft' : 'base'} className="min-h-0 flex-1 gap-4">
    <div className="flex flex-wrap items-center justify-between gap-3"><TabsList><TabsTrigger value="base"><FileCode2 />Current DDL</TabsTrigger><TabsTrigger value="draft"><GitCompareArrows />Generated SQL{generatedSql ? <Badge variant="accent" className="ml-1 px-1.5">Preview</Badge> : null}</TabsTrigger></TabsList><div className="flex items-center gap-2 text-[length:var(--font-size-meta)] text-muted-foreground"><Code2 className="size-3.5 text-primary" /><span className="font-mono">{table}</span><Badge variant="outline">{engine}</Badge></div></div>
    <TabsContent value="base" className="min-h-0"><WorkspacePanel noPadding className="flex min-h-[28rem] flex-col" title="Current database definition" description={source === 'api' ? 'Returned by the selected database adapter' : 'Sample definition from the mock data source'} actions={<><Button size="xs" variant="outline" onClick={() => copySql('base', ddl)}>{copied === 'base' ? <Check /> : <Clipboard />}{copied === 'base' ? 'Copied' : 'Copy'}</Button><Button size="xs" variant="outline" onClick={() => downloadSql(ddl)}><Download />Download</Button></>}><div className="min-h-0 flex-1">{ddl ? <SqlBlock sql={ddl} /> : <EmptyState icon={<FileCode2 />} title="DDL is unavailable" description="The selected adapter did not return a table definition." />}</div></WorkspacePanel></TabsContent>
    <TabsContent value="draft" className="min-h-0"><WorkspacePanel noPadding className="flex min-h-[28rem] flex-col" title="Schema migration preview" description={generatedSql ? `${engine} SQL generated and hashed by the selected adapter` : 'Generated SQL appears after staged actions pass server validation'} actions={generatedSql ? <><Badge variant="warning"><Sparkles />Not applied</Badge><Button size="xs" variant="outline" onClick={() => copySql('draft', generatedSql)}>{copied === 'draft' ? <Check /> : <Clipboard />}{copied === 'draft' ? 'Copied' : 'Copy SQL'}</Button><Button size="xs" variant="outline" onClick={() => downloadSql(generatedSql)}><Download />Download</Button></> : null}><div className="min-h-0 flex-1">{generatedSql ? <SqlBlock sql={generatedSql} /> : <EmptyState icon={<GitCompareArrows />} title="Schema draft is clean" description="Add, edit, or remove a schema object to request a dialect-aware preview." />}</div></WorkspacePanel></TabsContent>
  </Tabs>
}

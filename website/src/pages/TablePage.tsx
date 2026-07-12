import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Download, MoreHorizontal, Plus, Table2 } from 'lucide-react'
import type { Connection } from '@/entities/connection'
import { useDataDockGateway } from '@/app/providers'
import { DataGrid } from '@/features/data-grid/DataGrid'
import { APP_CONFIG } from '@/shared/config/constants'
import { Button, IconButton } from '@/shared/ui'

type TablePageProps = {
  connection: Connection
  table: string
}

export function TablePage({ connection, table }: TablePageProps) {
  const gateway = useDataDockGateway()
  const [search, setSearch] = useState('')
  const [querySearch, setQuerySearch] = useState('')
  const [page, setPage] = useState(1)
  const [sortBy, setSortBy] = useState<string>()
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('asc')
  const pageSize = APP_CONFIG.table.defaultPageSize

  useEffect(() => {
    const timeout = window.setTimeout(() => setQuerySearch(search), 220)
    return () => window.clearTimeout(timeout)
  }, [search])

  const result = useQuery({
    queryKey: ['table-rows', connection.id, table, page, querySearch, sortBy, sortDirection],
    queryFn: ({ signal }) => gateway.getTableRows(connection.id, table, { page, pageSize, search: querySearch, sortBy, sortDirection }, signal),
  })
  const tableName = table.split('.').at(-1) ?? table
  const schemaName = table.includes('.') ? table.split('.')[0] : connection.database
  const total = result.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const rangeStart = total ? (page - 1) * pageSize + 1 : 0
  const rangeEnd = Math.min(page * pageSize, total)

  function toggleSort(column: string) {
    if (sortBy === column) setSortDirection((value) => value === 'asc' ? 'desc' : 'asc')
    else {
      setSortBy(column)
      setSortDirection('asc')
    }
    setPage(1)
  }

  return (
    <section className="flex h-full min-h-0 flex-col bg-background">
      <header className="flex shrink-0 items-end justify-between gap-5 border-b border-border px-6 py-5 lg:px-8">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-[length:var(--font-size-meta)] font-semibold tracking-[0.08em] text-primary uppercase"><Table2 className="size-3.5" />Table</div>
          <h1 className="mt-1 truncate text-[1.625rem] leading-8 font-semibold tracking-[-0.035em] text-foreground">{tableName}</h1>
          <div className="mt-1 flex items-center gap-2 text-[length:var(--font-size-ui)] text-muted-foreground"><span className="font-mono text-[length:var(--font-size-data)]">{schemaName}.{tableName}</span><span>·</span><span>{total.toLocaleString()} rows</span><span>·</span><span>{result.data?.durationMs ?? 0} ms</span></div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm"><Download />Export</Button>
          <IconButton label="More table actions" variant="outline"><MoreHorizontal /></IconButton>
          <Button size="sm"><Plus />New row</Button>
        </div>
      </header>

      <DataGrid
        key={`${connection.id}:${table}`}
        connectionId={connection.id}
        tableName={table}
        columns={result.data?.columns ?? []}
        rows={result.data?.rows ?? []}
        page={page}
        pageSize={pageSize}
        search={search}
        sortBy={sortBy}
        sortDirection={sortDirection}
        loading={result.isLoading}
        fetching={result.isFetching}
        error={result.error}
        onSearchChange={(value) => { setSearch(value); setPage(1) }}
        onSort={toggleSort}
        onRefresh={() => result.refetch()}
        onRetry={() => result.refetch()}
      />

      <footer className="flex h-[var(--toolbar-height)] shrink-0 items-center border-t border-border bg-surface/70 px-4 text-[length:var(--font-size-meta)] text-muted-foreground lg:px-6">
        <span>{result.data ? `${rangeStart.toLocaleString()}–${rangeEnd.toLocaleString()} of ${total.toLocaleString()}` : 'Loading rows…'}</span>
        <div className="ml-auto flex items-center gap-2"><Button size="xs" variant="outline" disabled={page <= 1} onClick={() => setPage((value) => value - 1)}>Previous</Button><span className="px-2 font-mono">{page} / {totalPages}</span><Button size="xs" variant="outline" disabled={page >= totalPages} onClick={() => setPage((value) => value + 1)}>Next</Button></div>
      </footer>
    </section>
  )
}

import { useMemo, useState } from 'react'
import type { QueryResult } from '@/entities/query'
import { DataGrid } from '@/features/data-grid/DataGrid'

type QueryResultGridProps = {
  connectionId: string
  result: QueryResult
  refreshing: boolean
  onRefresh: () => void
}

function compareValues(left: unknown, right: unknown) {
  if (left === right) return 0
  if (left === null || left === undefined) return 1
  if (right === null || right === undefined) return -1
  if (typeof left === 'number' && typeof right === 'number') return left - right
  return String(left).localeCompare(String(right), undefined, { numeric: true, sensitivity: 'base' })
}

export function QueryResultGrid({ connectionId, result, refreshing, onRefresh }: QueryResultGridProps) {
  const [search, setSearch] = useState('')
  const [sortBy, setSortBy] = useState<string>()
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('asc')
  const resultKey = result.columns.map((column) => `${column.key ?? column.name}:${column.type}`).join('|')
  const rows = useMemo(() => {
    const normalizedSearch = search.trim().toLowerCase()
    const filtered = normalizedSearch ? result.rows.filter((row) => result.columns.some((column) => String(row[column.key ?? column.name] ?? '').toLowerCase().includes(normalizedSearch))) : result.rows
    if (!sortBy) return filtered
    return [...filtered].sort((left, right) => compareValues(left[sortBy], right[sortBy]) * (sortDirection === 'asc' ? 1 : -1))
  }, [result, search, sortBy, sortDirection])

  function toggleSort(column: string) {
    if (sortBy === column) setSortDirection((value) => value === 'asc' ? 'desc' : 'asc')
    else {
      setSortBy(column)
      setSortDirection('asc')
    }
  }

  return (
    <DataGrid
      key={resultKey}
      connectionId={connectionId}
      tableName={`query-result:${resultKey}`}
      columns={result.columns}
      rows={rows}
      page={1}
      pageSize={Math.max(1, result.rows.length)}
      search={search}
      searchPlaceholder="Search query results…"
      sortBy={sortBy}
      sortDirection={sortDirection}
      loading={false}
      fetching={refreshing}
      onSearchChange={setSearch}
      onSort={toggleSort}
      onRefresh={onRefresh}
      onRetry={onRefresh}
    />
  )
}

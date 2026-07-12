import { memo, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import {
  Braces,
  ChevronRight,
  Columns3,
  Copy,
  Database,
  Eye,
  FileCode2,
  FileText,
  Folder,
  ListTree,
  MoreHorizontal,
  Play,
  RefreshCw,
  Search,
  Table2,
  Workflow,
} from 'lucide-react'
import type { Connection } from '@/entities/connection'
import type { CatalogTree, DatabaseObject, DatabaseObjectKind } from '@/entities/database-object'
import { APP_CONFIG } from '@/shared/config/constants'
import { cn } from '@/shared/lib/cn'
import {
  Badge,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  IconButton,
} from '@/shared/ui'

type ObjectTreeProps = {
  connections: Connection[]
  activeConnectionId?: string
  catalog?: CatalogTree
  search: string
  selectedObjectId?: string
  onConnectionSelect: (connection: Connection) => void
  onObjectSelect: (object: DatabaseObject) => void
  onOpenObject: (object: DatabaseObject) => void
  onRefresh: () => void
}

type ConnectionTreeRow = {
  type: 'connection'
  id: string
  depth: number
  connection: Connection
  expandable: boolean
  expanded: boolean
}

type ObjectTreeRow = {
  type: 'object'
  id: string
  depth: number
  object: DatabaseObject
  expandable: boolean
  expanded: boolean
}

type TreeRow = ConnectionTreeRow | ObjectTreeRow

const OBJECT_ICONS: Record<DatabaseObjectKind, typeof Database> = {
  database: Database,
  schema: Folder,
  group: ListTree,
  table: Table2,
  view: Eye,
  'materialized-view': Columns3,
  function: Braces,
  procedure: Workflow,
  sequence: FileCode2,
  extension: Copy,
  trigger: Play,
  column: Columns3,
}

const ENGINE_LABELS: Record<string, string> = {
  postgresql: 'PG',
  mysql: 'MY',
  mariadb: 'MA',
  sqlite: 'SQ',
  sqlserver: 'MS',
  oracle: 'OR',
  clickhouse: 'CH',
  redis: 'RD',
  mongodb: 'MO',
}

function objectMatches(node: DatabaseObject, search: string): boolean {
  if (!search) return true
  if (`${node.name} ${node.qualifiedName} ${node.kind}`.toLowerCase().includes(search)) return true
  return node.children?.some((child) => objectMatches(child, search)) ?? false
}

function appendObjectRows(
  rows: TreeRow[],
  nodes: DatabaseObject[],
  depth: number,
  expandedIds: Set<string>,
  search: string,
) {
  for (const node of nodes) {
    if (!objectMatches(node, search)) continue
    const expandable = Boolean(node.children?.length)
    const expanded = search ? expandable : expandedIds.has(node.id)
    rows.push({ type: 'object', id: node.id, depth, object: node, expandable, expanded })
    if (expanded && node.children?.length) {
      appendObjectRows(rows, node.children, depth + 1, expandedIds, search)
    }
  }
}

function createRows(
  connections: Connection[],
  activeConnectionId: string | undefined,
  catalog: CatalogTree | undefined,
  expandedIds: Set<string>,
  search: string,
) {
  const rows: TreeRow[] = []

  for (const connection of connections) {
    const matchesConnection = `${connection.name} ${connection.engine} ${connection.database}`.toLowerCase().includes(search)
    const activeCatalog = connection.id === catalog?.connectionId ? catalog : undefined
    const matchesCatalog = activeCatalog?.databases.some((database) => objectMatches(database, search)) ?? false
    if (search && !matchesConnection && !matchesCatalog) continue

    const expandable = Boolean(activeCatalog?.databases.length) || connection.id === activeConnectionId
    const expanded = search ? Boolean(activeCatalog) : expandedIds.has(connection.id)
    rows.push({ type: 'connection', id: connection.id, depth: 0, connection, expandable, expanded })
    if (expanded && activeCatalog) {
      appendObjectRows(rows, activeCatalog.databases, 1, expandedIds, search)
    }
  }

  return rows
}

const TreeRowView = memo(function TreeRowView({
  row,
  selected,
  onToggle,
  onConnectionSelect,
  onObjectSelect,
  onOpenObject,
  onRefresh,
}: {
  row: TreeRow
  selected: boolean
  onToggle: (id: string) => void
  onConnectionSelect: (connection: Connection) => void
  onObjectSelect: (object: DatabaseObject) => void
  onOpenObject: (object: DatabaseObject) => void
  onRefresh: () => void
}) {
  const isConnection = row.type === 'connection'
  const label = isConnection ? row.connection.name : row.object.name
  const Icon = isConnection ? Database : OBJECT_ICONS[row.object.kind]
  const status = isConnection ? row.connection.status : undefined
  const isLeaf = row.type === 'object' && !row.expandable

  const select = () => {
    if (row.type === 'connection') {
      onConnectionSelect(row.connection)
      if (row.expandable) onToggle(row.id)
      return
    }
    onObjectSelect(row.object)
    if (row.expandable) onToggle(row.id)
    if (isLeaf) onOpenObject(row.object)
  }

  return (
    <DropdownMenu>
      <div
        className={cn(
          'group flex h-full items-center gap-1.5 rounded-md pr-1 text-[length:var(--font-size-data)] text-muted-foreground outline-none transition-colors hover:bg-accent/65 hover:text-foreground focus-within:bg-accent/65',
          selected && 'bg-accent text-accent-foreground',
        )}
        style={{ paddingLeft: `${6 + row.depth * 16}px` }}
      >
        <button
          type="button"
          aria-label={row.expanded ? `Collapse ${label}` : `Expand ${label}`}
          tabIndex={row.expandable ? 0 : -1}
          onClick={(event) => {
            event.stopPropagation()
            if (row.expandable) onToggle(row.id)
          }}
          className={cn('grid size-5 shrink-0 place-items-center rounded text-muted-foreground', !row.expandable && 'pointer-events-none opacity-0')}
        >
          <ChevronRight className={cn('size-3.5 transition-transform duration-150', row.expanded && 'rotate-90')} />
        </button>
        <button type="button" onClick={select} onDoubleClick={() => row.type === 'object' && onOpenObject(row.object)} className="flex min-w-0 flex-1 items-center gap-2 text-left outline-none">
          <Icon className={cn('size-3.5 shrink-0 text-muted-foreground', isLeaf && 'text-primary/85')} strokeWidth={1.8} />
          <span className={cn('min-w-0 flex-1 truncate', isConnection && 'font-medium text-foreground')}>{label}</span>
          {row.type === 'object' && row.object.count !== undefined ? (
            <span className="font-mono text-[length:var(--font-size-meta)] tabular-nums text-muted-foreground">{row.object.count}</span>
          ) : null}
          {isConnection ? (
            <>
              <Badge variant="outline" className="h-5 px-1.5 font-mono text-[length:var(--font-size-meta)]">{ENGINE_LABELS[row.connection.engine] ?? row.connection.engine.slice(0, 2).toUpperCase()}</Badge>
              <span
                className={cn(
                  'size-1.5 rounded-full bg-muted-foreground',
                  status === 'connected' && 'bg-emerald-400',
                  status === 'connecting' && 'animate-pulse bg-amber-400',
                  status === 'error' && 'bg-red-400',
                )}
                aria-label={status}
              />
            </>
          ) : null}
        </button>
        <DropdownMenuTrigger asChild>
          <IconButton label={`Actions for ${label}`} size="icon-xs" className="opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 data-[state=open]:opacity-100">
            <MoreHorizontal />
          </IconButton>
        </DropdownMenuTrigger>
      </div>
      <DropdownMenuContent align="start" side="right" className="w-52">
        {row.type === 'object' ? (
          <>
            <DropdownMenuItem onSelect={() => onOpenObject(row.object)}>
              <Table2 />
              Open data
            </DropdownMenuItem>
            <DropdownMenuItem>
              <FileText />
              View DDL
            </DropdownMenuItem>
            <DropdownMenuItem>
              <Copy />
              Copy qualified name
            </DropdownMenuItem>
          </>
        ) : (
          <>
            <DropdownMenuItem onSelect={() => onConnectionSelect(row.connection)}>
              <Database />
              Open connection
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={onRefresh}>
              <RefreshCw />
              Refresh catalog
            </DropdownMenuItem>
          </>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem>
          <Braces />
          New query
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
})

export function ObjectTree({
  connections,
  activeConnectionId,
  catalog,
  search,
  selectedObjectId,
  onConnectionSelect,
  onObjectSelect,
  onOpenObject,
  onRefresh,
}: ObjectTreeProps) {
  const parentRef = useRef<HTMLDivElement>(null)
  const [expandedIds, setExpandedIds] = useState<Set<string>>(() => new Set())
  const deferredSearch = useDeferredValue(search.trim().toLowerCase())

  useEffect(() => {
    if (!catalog) return
    const initialIds = new Set<string>([catalog.connectionId])
    for (const database of catalog.databases) {
      initialIds.add(database.id)
      for (const schema of database.children ?? []) {
        initialIds.add(schema.id)
        const tables = schema.children?.find((child) => child.name === 'Tables')
        if (tables) initialIds.add(tables.id)
      }
    }
    setExpandedIds((current) => new Set([...current, ...initialIds]))
  }, [catalog])

  const rows = useMemo(
    () => createRows(connections, activeConnectionId, catalog, expandedIds, deferredSearch),
    [connections, activeConnectionId, catalog, expandedIds, deferredSearch],
  )
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => APP_CONFIG.ui.densities[APP_CONFIG.ui.defaultDensity].treeRowHeight,
    overscan: APP_CONFIG.explorer.overscan,
  })

  const toggle = (id: string) => {
    setExpandedIds((current) => {
      const next = new Set(current)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  if (!rows.length) {
    return (
      <div className="grid h-full place-items-center px-6 text-center">
        <div>
          <Search className="mx-auto size-5 text-muted-foreground" />
          <p className="mt-3 text-[length:var(--font-size-ui)] font-medium text-foreground">No objects found</p>
          <p className="mt-1 text-[length:var(--font-size-meta)] leading-5 text-muted-foreground">Try a table, schema, or function name.</p>
        </div>
      </div>
    )
  }

  return (
    <div ref={parentRef} className="h-full overflow-auto px-1.5 py-1.5" role="tree" aria-label="Database objects">
      <div className="relative w-full" style={{ height: `${virtualizer.getTotalSize()}px` }}>
        {virtualizer.getVirtualItems().map((virtualRow) => {
          const row = rows[virtualRow.index]
          return (
            <div
              key={row.id}
              ref={virtualizer.measureElement}
              data-index={virtualRow.index}
              role="treeitem"
              aria-level={row.depth + 1}
              aria-expanded={row.expandable ? row.expanded : undefined}
              className="absolute top-0 left-0 h-[var(--tree-row-height)] w-full"
              style={{ transform: `translateY(${virtualRow.start}px)` }}
            >
              <TreeRowView
                row={row}
                selected={selectedObjectId === row.id || (row.type === 'connection' && activeConnectionId === row.connection.id)}
                onToggle={toggle}
                onConnectionSelect={onConnectionSelect}
                onObjectSelect={onObjectSelect}
                onOpenObject={onOpenObject}
                onRefresh={onRefresh}
              />
            </div>
          )
        })}
      </div>
    </div>
  )
}

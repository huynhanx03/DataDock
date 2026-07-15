import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertCircle,
  ArrowLeft,
  BookMarked,
  Check,
  Copy,
  Edit3,
  FilePlus2,
  Folder,
  FolderHeart,
  FolderPlus,
  FolderX,
  Link2,
  Link2Off,
  List,
  LoaderCircle,
  LockKeyhole,
  MoreHorizontal,
  PanelRight,
  Play,
  Search,
  Share2,
  Star,
  Tag,
  Trash2,
  X,
} from 'lucide-react'
import { useDataDockGateway } from '@/app/providers'
import type { Connection } from '@/entities/connection'
import type {
  CreateSavedQueryInput,
  PublicSavedQuery,
  SavedQuery,
  SavedQueryFolder,
  UpdateSavedQueryInput,
} from '@/entities/query'
import { APP_ROUTES } from '@/shared/config/routes'
import { writeClipboardText } from '@/shared/lib/clipboard'
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  EmptyState,
  IconButton,
  Input,
  PropertyField,
  PropertyGrid,
  Select,
  Textarea,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  WorkspaceHeader,
  WorkspacePage,
  WorkspaceToolbar,
} from '@/shared/ui'

type QueryDraft = {
  title: string
  folder: string
  tags: string
  sql: string
}

type FolderView = {
  name: string
  count: number
  position: number
  record?: SavedQueryFolder
}

type SaveMutationInput =
  | { mode: 'create'; input: CreateSavedQueryInput }
  | { mode: 'update'; id: string; input: UpdateSavedQueryInput }

type FolderMutationInput =
  | { mode: 'create'; name: string; position: number }
  | { mode: 'rename'; id: string; previousName: string; name: string }

const FILTER_ALL = 'all'
const FILTER_FAVORITES = 'favorites'
const EMPTY_SAVED_QUERIES: SavedQuery[] = []
const DATE_FORMATTER = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
const DATE_TIME_FORMATTER = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })

const queryKeys = Object.freeze({
  saved: ['saved-queries'] as const,
  folders: ['saved-query-folders'] as const,
  connections: ['connections'] as const,
  share: (id: string) => ['saved-query-share', id] as const,
  publicShare: (code: string) => ['shared-query', code] as const,
})

function folderFilter(name: string) {
  return `folder:${name}`
}

function folderNameFromFilter(filter: string) {
  return filter.startsWith('folder:') ? filter.slice(7) : undefined
}

function dateLabel(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return DATE_FORMATTER.format(date)
}

function dateTimeLabel(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return DATE_TIME_FORMATTER.format(date)
}

function queryDraft(item?: SavedQuery, folder = 'General'): QueryDraft {
  return item
    ? { title: item.title, folder: item.folder, tags: item.tags.join(', '), sql: item.sql }
    : { title: '', folder, tags: '', sql: 'SELECT\n  *\nFROM public.users\nLIMIT 100;' }
}

function parseTags(value: string) {
  return Array.from(new Set(value.split(',').map((tag) => tag.trim().replace(/^#/, '').toLowerCase()).filter(Boolean)))
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback
}

function shareUrl(code: string) {
  const url = new URL(APP_ROUTES.savedQueries, window.location.origin)
  url.searchParams.set('share', code)
  return url.toString()
}

function replaceSavedQuery(current: SavedQuery[] | undefined, updated: SavedQuery) {
  if (!current) return [updated]
  const exists = current.some((item) => item.id === updated.id)
  return exists ? current.map((item) => item.id === updated.id ? updated : item) : [updated, ...current]
}

function PublicShareDetail({
  query,
  connection,
  copied,
  copyError,
  onBack,
  onCopy,
  onOpenQuery,
}: {
  query: PublicSavedQuery
  connection?: Connection
  copied: boolean
  copyError: string
  onBack: () => void
  onCopy: () => void
  onOpenQuery: (sql: string, connectionId?: string) => void
}) {
  return <div className="mx-auto flex min-h-full max-w-5xl flex-col">
    <Button size="xs" variant="ghost" className="mb-3 w-fit" onClick={onBack}><ArrowLeft />Back to query library</Button>
    <div className="flex flex-wrap items-start gap-3">
      <div className="grid size-10 shrink-0 place-items-center rounded-xl border border-info/30 bg-info/10 text-info-foreground"><Link2 className="size-[1.125rem]" /></div>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2"><h2 className="truncate text-lg font-semibold tracking-[-0.025em]">{query.title}</h2><Badge variant="info"><Share2 />Public link</Badge></div>
        <p className="mt-1 text-[length:var(--font-size-meta)] text-muted-foreground">Shared {dateTimeLabel(query.sharedAt)}{connection ? ` · opens on ${connection.name}` : ' · read-only shared view'}</p>
      </div>
    </div>
    <div className="mt-4 flex flex-wrap gap-1.5">{query.tags.map((tag) => <Badge key={tag} variant="outline">#{tag}</Badge>)}</div>
    <div className="mt-4 flex min-h-[18rem] flex-1 flex-col overflow-hidden rounded-xl border border-border bg-[#070a11]">
      <div className="flex h-9 shrink-0 items-center gap-2 border-b border-border px-3 text-[length:var(--font-size-meta)] text-muted-foreground"><BookMarked className="size-3.5 text-primary" /><span>Shared SQL</span><span className="ml-auto font-mono">{query.sql.split('\n').length} lines</span></div>
      <pre className="min-h-0 flex-1 overflow-auto p-4 font-mono text-[length:var(--font-size-data)] leading-6 text-foreground"><code>{query.sql}</code></pre>
    </div>
    <PropertyGrid className="mt-4">
      <PropertyField label="Created"><div className="rounded-md border border-border bg-surface px-3 py-2 text-[length:var(--font-size-ui)]">{dateLabel(query.createdAt)}</div></PropertyField>
      <PropertyField label="Shared"><div className="rounded-md border border-border bg-surface px-3 py-2 text-[length:var(--font-size-ui)]">{dateTimeLabel(query.sharedAt)}</div></PropertyField>
      <PropertyField label="Expiry"><div className="rounded-md border border-border bg-surface px-3 py-2 text-[length:var(--font-size-ui)]">{query.expiresAt ? dateTimeLabel(query.expiresAt) : 'No expiry'}</div></PropertyField>
    </PropertyGrid>
    {copyError ? <div role="alert" className="mt-4 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-ui)] text-destructive">{copyError}</div> : null}
    <div className="mt-4 flex flex-wrap items-center gap-2">{connection ? <Button size="sm" onClick={() => onOpenQuery(query.sql, connection.id)}><Play />Open in SQL</Button> : null}<Button size="sm" variant="outline" onClick={onCopy}>{copied ? <Check /> : <Copy />}{copied ? 'Copied' : 'Copy SQL'}</Button></div>
  </div>
}

export function SavedQueriesPage({ connection, onOpenQuery }: { connection?: Connection; onOpenQuery: (sql: string, connectionId?: string) => void }) {
  const gateway = useDataDockGateway()
  const queryClient = useQueryClient()
  const [selectedId, setSelectedId] = useState('')
  const [activeFolder, setActiveFolder] = useState(FILTER_ALL)
  const [tagFilter, setTagFilter] = useState('all')
  const [sort, setSort] = useState<'updated' | 'title'>('updated')
  const [search, setSearch] = useState('')
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState('')
  const [editorMode, setEditorMode] = useState<'create' | 'edit' | null>(null)
  const [draft, setDraft] = useState<QueryDraft>(queryDraft())
  const [deleteQueryOpen, setDeleteQueryOpen] = useState(false)
  const [folderDialog, setFolderDialog] = useState<'create' | 'rename' | null>(null)
  const [folderTarget, setFolderTarget] = useState<SavedQueryFolder | null>(null)
  const [folderName, setFolderName] = useState('')
  const [deleteFolderTarget, setDeleteFolderTarget] = useState<FolderView | null>(null)
  const [deleteFolderPolicy, setDeleteFolderPolicy] = useState<'reject' | 'move'>('reject')
  const [deleteFolderDestination, setDeleteFolderDestination] = useState('')
  const [shareInfoOpen, setShareInfoOpen] = useState(false)
  const [shareCopied, setShareCopied] = useState<'link' | 'sql' | null>(null)
  const [shareCopyError, setShareCopyError] = useState('')
  const [shareToken, setShareToken] = useState(() => new URLSearchParams(window.location.search).get('share'))
  const [compactPane, setCompactPane] = useState<'list' | 'detail'>(() => shareToken ? 'detail' : 'list')

  const saved = useQuery({ queryKey: queryKeys.saved, queryFn: ({ signal }) => gateway.listSavedQueries(signal), enabled: !shareToken })
  const folderRecords = useQuery({ queryKey: queryKeys.folders, queryFn: ({ signal }) => gateway.listSavedQueryFolders(signal), enabled: !shareToken })
  const connectionProfiles = useQuery({ queryKey: queryKeys.connections, queryFn: ({ signal }) => gateway.listConnections(undefined, signal), enabled: !shareToken })
  const publicShare = useQuery({
    queryKey: queryKeys.publicShare(shareToken ?? ''),
    queryFn: ({ signal }) => gateway.getSharedQuery(shareToken ?? '', signal),
    enabled: Boolean(shareToken),
    retry: false,
  })

  const allQueries = saved.data ?? EMPTY_SAVED_QUERIES
  const folders = useMemo<FolderView[]>(() => {
    const counts = new Map<string, number>()
    allQueries.forEach((item) => counts.set(item.folder, (counts.get(item.folder) ?? 0) + 1))
    const result = new Map<string, FolderView>()
    folderRecords.data?.forEach((item) => result.set(item.name, { name: item.name, count: counts.get(item.name) ?? item.queryCount, position: item.position, record: item }))
    counts.forEach((count, name) => {
      if (!result.has(name)) result.set(name, { name, count, position: Number.MAX_SAFE_INTEGER })
    })
    return Array.from(result.values()).sort((left, right) => left.position - right.position || left.name.localeCompare(right.name))
  }, [allQueries, folderRecords.data])
  const tags = useMemo(() => {
    const counts = new Map<string, number>()
    allQueries.forEach((item) => item.tags.forEach((tag) => counts.set(tag, (counts.get(tag) ?? 0) + 1)))
    return Array.from(counts.entries()).sort((left, right) => right[1] - left[1] || left[0].localeCompare(right[0]))
  }, [allQueries])
  const selectedFolderName = folderNameFromFilter(activeFolder)
  const records = useMemo(() => allQueries.filter((item) => {
    const matchesFolder = activeFolder === FILTER_ALL || activeFolder === FILTER_FAVORITES ? activeFolder !== FILTER_FAVORITES || item.favorite : item.folder === selectedFolderName
    const needle = search.trim().toLowerCase()
    const matchesSearch = !needle || [item.title, item.folder, item.tags.join(' '), item.sql].join(' ').toLowerCase().includes(needle)
    const matchesTag = tagFilter === 'all' || item.tags.includes(tagFilter)
    return matchesFolder && matchesSearch && matchesTag
  }).sort((left, right) => sort === 'title' ? left.title.localeCompare(right.title) : right.updatedAt.localeCompare(left.updatedAt)), [activeFolder, allQueries, search, selectedFolderName, sort, tagFilter])
  const selected = shareToken ? undefined : records.find((item) => item.id === selectedId) ?? records[0]
  const selectedShareId = selected?.id ?? ''
  const selectedShare = useQuery({
    queryKey: queryKeys.share(selectedShareId),
    queryFn: ({ signal }) => gateway.getSavedQueryShare(selectedShareId, signal),
    enabled: Boolean(selectedShareId),
    retry: false,
  })
  const selectedConnection = selected?.connectionId ? connectionProfiles.data?.find((item) => item.id === selected.connectionId) : undefined
  const selectedConnectionLabel = !selected?.connectionId
    ? connection ? `${connection.name} · no connection pinned` : 'No connection pinned'
    : selected.connectionId === connection?.id
      ? `${connection.name} · current connection`
      : selectedConnection
        ? `${selectedConnection.name} · opens on saved connection`
        : `${selected.connectionId} · connection profile unavailable`
  const currentFolderLabel = activeFolder === FILTER_ALL ? 'All queries' : activeFolder === FILTER_FAVORITES ? 'Favorites' : selectedFolderName ?? 'Folder'
  const destinationFolders = useMemo(() => Array.from(new Set([...folders.map((item) => item.name), 'General'])).filter((name) => name !== deleteFolderTarget?.name).sort((left, right) => left.localeCompare(right)), [deleteFolderTarget?.name, folders])

  useEffect(() => {
    if (shareToken) return
    if (selected && selected.id !== selectedId) setSelectedId(selected.id)
    if (!selected && selectedId) setSelectedId('')
  }, [selected, selectedId, shareToken])

  const saveQuery = useMutation({
    mutationFn: (variables: SaveMutationInput) => variables.mode === 'create'
      ? gateway.createSavedQuery(variables.input)
      : gateway.updateSavedQuery(variables.id, variables.input),
    onSuccess: (updated, variables) => {
      queryClient.setQueryData<SavedQuery[]>(queryKeys.saved, (current) => replaceSavedQuery(current, updated))
      void queryClient.invalidateQueries({ queryKey: queryKeys.saved })
      void queryClient.invalidateQueries({ queryKey: queryKeys.folders })
      setSelectedId(updated.id)
      if (variables.mode === 'create' || updated.folder !== selectedFolderName) setActiveFolder(FILTER_ALL)
      setEditorMode(null)
      setCompactPane('detail')
    },
  })
  const deleteQuery = useMutation({
    mutationFn: (id: string) => gateway.deleteSavedQuery(id),
    onSuccess: (_, id) => {
      queryClient.setQueryData<SavedQuery[]>(queryKeys.saved, (current) => current?.filter((item) => item.id !== id))
      void queryClient.invalidateQueries({ queryKey: queryKeys.saved })
      void queryClient.invalidateQueries({ queryKey: queryKeys.folders })
      queryClient.removeQueries({ queryKey: queryKeys.share(id) })
      setDeleteQueryOpen(false)
      setSelectedId('')
      setCompactPane('list')
    },
  })
  const duplicateQuery = useMutation({
    mutationFn: (id: string) => gateway.duplicateSavedQuery(id),
    onSuccess: (created) => {
      queryClient.setQueryData<SavedQuery[]>(queryKeys.saved, (current) => replaceSavedQuery(current, created))
      void queryClient.invalidateQueries({ queryKey: queryKeys.saved })
      void queryClient.invalidateQueries({ queryKey: queryKeys.folders })
      setActiveFolder(FILTER_ALL)
      setSelectedId(created.id)
      setCompactPane('detail')
    },
  })
  const favoriteQuery = useMutation({
    mutationFn: ({ id, favorite }: { id: string; favorite: boolean }) => gateway.setSavedQueryFavorite(id, favorite),
    onSuccess: (updated) => {
      queryClient.setQueryData<SavedQuery[]>(queryKeys.saved, (current) => replaceSavedQuery(current, updated))
      void queryClient.invalidateQueries({ queryKey: queryKeys.saved })
    },
  })
  const mutateFolder = useMutation({
    mutationFn: (variables: FolderMutationInput) => variables.mode === 'create'
      ? gateway.createSavedQueryFolder({ name: variables.name, position: variables.position })
      : gateway.updateSavedQueryFolder(variables.id, { name: variables.name }),
    onSuccess: async (updated, variables) => {
      queryClient.setQueryData<SavedQueryFolder[]>(queryKeys.folders, (current) => {
        if (!current) return [updated]
        const exists = current.some((item) => item.id === updated.id)
        return exists ? current.map((item) => item.id === updated.id ? updated : item) : [...current, updated]
      })
      await queryClient.invalidateQueries({ queryKey: queryKeys.folders })
      if (variables.mode === 'rename') {
        await queryClient.invalidateQueries({ queryKey: queryKeys.saved })
        if (activeFolder === folderFilter(variables.previousName)) setActiveFolder(folderFilter(updated.name))
      } else {
        setActiveFolder(folderFilter(updated.name))
      }
      setCompactPane('list')
      setFolderDialog(null)
      setFolderTarget(null)
      setFolderName('')
    },
  })
  const deleteFolder = useMutation({
    mutationFn: ({ id, policy, destination }: { id: string; name: string; policy: 'reject' | 'move'; destination: string }) => gateway.deleteSavedQueryFolder(id, policy, destination),
    onSuccess: async (_, variables) => {
      queryClient.setQueryData<SavedQueryFolder[]>(queryKeys.folders, (current) => current?.filter((item) => item.id !== variables.id))
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.folders }),
        queryClient.invalidateQueries({ queryKey: queryKeys.saved }),
      ])
      if (activeFolder === folderFilter(variables.name)) setActiveFolder(FILTER_ALL)
      setDeleteFolderTarget(null)
    },
  })
  const enableShare = useMutation({
    mutationFn: (id: string) => gateway.shareSavedQuery(id),
    onSuccess: (share) => queryClient.setQueryData(queryKeys.share(share.savedQueryId), share),
  })
  const revokeShare = useMutation({
    mutationFn: (id: string) => gateway.revokeSavedQueryShare(id),
    onSuccess: (_, id) => queryClient.setQueryData(queryKeys.share(id), null),
  })

  function openCreate() {
    saveQuery.reset()
    const initialFolder = selectedFolderName ?? 'General'
    setDraft(queryDraft(undefined, initialFolder))
    setEditorMode('create')
  }

  function openEdit() {
    if (!selected) return
    saveQuery.reset()
    setDraft(queryDraft(selected))
    setEditorMode('edit')
  }

  function submitQuery() {
    const input = {
      folder: draft.folder.trim() || 'General',
      title: draft.title.trim(),
      sql: draft.sql.trim(),
      tags: parseTags(draft.tags),
    }
    if (!input.title || !input.sql) return
    if (editorMode === 'create') saveQuery.mutate({ mode: 'create', input: connection ? { ...input, connectionId: connection.id } : input })
    else if (selected) saveQuery.mutate({ mode: 'update', id: selected.id, input })
  }

  function openFolderCreate() {
    mutateFolder.reset()
    setFolderTarget(null)
    setFolderName('')
    setFolderDialog('create')
  }

  function openFolderRename(record: SavedQueryFolder) {
    mutateFolder.reset()
    setFolderTarget(record)
    setFolderName(record.name)
    setFolderDialog('rename')
  }

  function submitFolder() {
    const name = folderName.trim()
    if (!name) return
    if (folderDialog === 'create') mutateFolder.mutate({ mode: 'create', name, position: folderRecords.data?.length ?? 0 })
    else if (folderTarget) mutateFolder.mutate({ mode: 'rename', id: folderTarget.id, previousName: folderTarget.name, name })
  }

  function openFolderDelete(target: FolderView) {
    if (!target.record) return
    deleteFolder.reset()
    const destinations = Array.from(new Set([...folders.map((item) => item.name), 'General'])).filter((name) => name !== target.name).sort((left, right) => left.localeCompare(right))
    setDeleteFolderTarget(target)
    setDeleteFolderPolicy(target.count ? 'move' : 'reject')
    setDeleteFolderDestination(destinations.includes('General') ? 'General' : destinations[0] ?? '')
  }

  function submitFolderDelete() {
    const record = deleteFolderTarget?.record
    if (!record) return
    if (deleteFolderPolicy === 'reject' && deleteFolderTarget.count > 0) return
    if (deleteFolderPolicy === 'move' && !deleteFolderDestination) return
    deleteFolder.mutate({ id: record.id, name: deleteFolderTarget.name, policy: deleteFolderPolicy, destination: deleteFolderPolicy === 'move' ? deleteFolderDestination : '' })
  }

  function openShare() {
    enableShare.reset()
    revokeShare.reset()
    setShareCopyError('')
    setShareInfoOpen(true)
  }

  async function copySql(sql: string) {
    try {
      await writeClipboardText(sql)
      setCopyError('')
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1_400)
    } catch (error) {
      setCopyError(errorMessage(error, 'SQL could not be copied.'))
    }
  }

  async function copyShare(value: string, kind: 'link' | 'sql') {
    try {
      await writeClipboardText(value)
      setShareCopyError('')
      setShareCopied(kind)
      window.setTimeout(() => setShareCopied(null), 1_400)
    } catch (error) {
      setShareCopyError(errorMessage(error, 'The content could not be copied.'))
    }
  }

  function closePublicShare() {
    window.history.replaceState(null, '', APP_ROUTES.savedQueries)
    setShareToken(null)
    setCompactPane('list')
  }

  const actionError = duplicateQuery.error ?? favoriteQuery.error
  const refreshError = saved.data && saved.error ? saved.error : undefined
  const activeShareUrl = selectedShare.data ? shareUrl(selectedShare.data.shareCode) : ''

  return <WorkspacePage>
    <WorkspaceHeader
      compact
      icon={<FolderHeart />}
      eyebrow="Query library"
      title="Saved queries"
      description="Reusable SQL organized by workflow, folder and tag"
      meta={<Badge variant={gateway.source === 'mock' ? 'accent' : 'success'}>{gateway.source === 'mock' ? 'Mock data · resets on reload' : 'Live API · persisted'}</Badge>}
      actions={!shareToken ? <Button size="sm" onClick={openCreate}><FilePlus2 />New saved query</Button> : undefined}
    />
    {!shareToken ? <WorkspaceToolbar>
      <div className="relative min-w-56 max-w-xl flex-1 max-md:min-w-0 max-md:basis-full"><Search className="pointer-events-none absolute top-1/2 left-3 size-3.5 -translate-y-1/2 text-muted-foreground" /><Input aria-label="Search saved queries" value={search} onChange={(event) => { setSearch(event.target.value); setCompactPane('list') }} className="h-[var(--control-height-sm)] pl-8" placeholder="Search titles, SQL, folders or tags…" /></div>
      <div className="min-w-36 flex-1 lg:hidden"><Select aria-label="Saved query folder" value={activeFolder} onChange={(event) => { setActiveFolder(event.target.value); setCompactPane('list') }} className="h-[var(--control-height-sm)]"><option value={FILTER_ALL}>All queries ({allQueries.length})</option><option value={FILTER_FAVORITES}>Favorites ({allQueries.filter((item) => item.favorite).length})</option>{folders.map((item) => <option key={item.name} value={folderFilter(item.name)}>{item.name} ({item.count})</option>)}</Select></div>
      <div className="max-md:min-w-32 max-md:flex-1"><Select aria-label="Tag filter" value={tagFilter} onChange={(event) => { setTagFilter(event.target.value); setCompactPane('list') }} className="h-[var(--control-height-sm)] w-40 max-md:w-full"><option value="all">All tags</option>{tags.map(([tag, count]) => <option key={tag} value={tag}>#{tag} ({count})</option>)}</Select></div>
      <div className="max-md:min-w-32 max-md:flex-1"><Select aria-label="Sort saved queries" value={sort} onChange={(event) => setSort(event.target.value as 'updated' | 'title')} className="h-[var(--control-height-sm)] w-36 max-md:w-full"><option value="updated">Recently updated</option><option value="title">Title A–Z</option></Select></div>
      <Badge variant="outline" className="ml-auto max-md:hidden">{records.length} queries</Badge>
      <div className="grid w-full grid-cols-2 gap-1 rounded-lg border border-border bg-background/65 p-1 md:hidden" role="group" aria-label="Saved query view"><Button size="xs" variant={compactPane === 'list' ? 'secondary' : 'ghost'} aria-pressed={compactPane === 'list'} onClick={() => setCompactPane('list')}><List />Queries <Badge variant="outline" className="ml-1 px-1.5">{records.length}</Badge></Button><Button size="xs" variant={compactPane === 'detail' ? 'secondary' : 'ghost'} aria-pressed={compactPane === 'detail'} disabled={!selected && !shareToken} onClick={() => setCompactPane('detail')}><PanelRight />Details</Button></div>
    </WorkspaceToolbar> : null}
    {!shareToken && (actionError || refreshError) ? <div role="alert" className="flex shrink-0 items-center gap-2 border-b border-destructive/30 bg-destructive/10 px-4 py-2 text-[length:var(--font-size-ui)] text-destructive"><AlertCircle className="size-3.5 shrink-0" /><span className="min-w-0 flex-1">{errorMessage(actionError ?? refreshError, 'The query library could not be refreshed.')}</span><Button size="xs" variant="ghost" onClick={() => { duplicateQuery.reset(); favoriteQuery.reset(); if (refreshError) saved.refetch() }}>{refreshError ? 'Retry' : 'Dismiss'}</Button></div> : null}
    <div className={`grid min-h-0 min-w-0 flex-1 overflow-hidden ${shareToken ? 'grid-cols-1' : 'grid-cols-[11.75rem_minmax(18rem,23rem)_minmax(0,1fr)] max-xl:grid-cols-[10.5rem_minmax(17rem,20rem)_minmax(0,1fr)] max-lg:grid-cols-[minmax(17rem,20rem)_minmax(0,1fr)] max-lg:[&>aside]:hidden max-md:grid-cols-1'}`}>
      <aside className={`${shareToken ? 'hidden' : 'flex'} min-h-0 flex-col overflow-hidden border-r border-border bg-surface/35`}>
        <div className="flex h-9 shrink-0 items-center gap-2 border-b border-border px-3"><span className="text-[length:var(--font-size-meta)] font-semibold tracking-[0.08em] text-muted-foreground uppercase">Folders</span><Tooltip><TooltipTrigger asChild><IconButton label="Create folder" size="icon-xs" className="ml-auto" onClick={openFolderCreate}><FolderPlus /></IconButton></TooltipTrigger><TooltipContent>Create folder</TooltipContent></Tooltip></div>
        <div className="min-h-0 flex-1 overflow-y-auto p-2">
          <button type="button" onClick={() => { setActiveFolder(FILTER_ALL); setCompactPane('list') }} className={`flex h-8 w-full items-center gap-2 rounded-md px-2 text-[length:var(--font-size-ui)] transition-colors ${activeFolder === FILTER_ALL ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:bg-accent/55 hover:text-foreground'}`}><BookMarked className="size-3.5" /><span>All queries</span><span className="ml-auto font-mono text-[length:var(--font-size-meta)]">{allQueries.length}</span></button>
          <button type="button" onClick={() => { setActiveFolder(FILTER_FAVORITES); setCompactPane('list') }} className={`mt-0.5 flex h-8 w-full items-center gap-2 rounded-md px-2 text-[length:var(--font-size-ui)] transition-colors ${activeFolder === FILTER_FAVORITES ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:bg-accent/55 hover:text-foreground'}`}><Star className="size-3.5" /><span>Favorites</span><span className="ml-auto font-mono text-[length:var(--font-size-meta)]">{allQueries.filter((item) => item.favorite).length}</span></button>
          <div className="my-2 h-px bg-border" />
          {folderRecords.isLoading && !folderRecords.data ? <div className="flex items-center gap-2 px-2 py-3 text-[length:var(--font-size-meta)] text-muted-foreground"><LoaderCircle className="size-3.5 animate-spin" />Loading folders…</div> : null}
          {folderRecords.isError && !folderRecords.data ? <div className="rounded-md border border-destructive/25 bg-destructive/8 p-2 text-[length:var(--font-size-meta)] text-destructive"><p>{errorMessage(folderRecords.error, 'Folders unavailable')}</p><Button size="xs" variant="ghost" className="mt-1" onClick={() => folderRecords.refetch()}>Retry</Button></div> : null}
          {folders.map((item) => <div key={item.name} className="group flex items-center gap-0.5"><button type="button" onClick={() => { setActiveFolder(folderFilter(item.name)); setCompactPane('list') }} className={`flex h-8 min-w-0 flex-1 items-center gap-2 rounded-md px-2 text-[length:var(--font-size-ui)] transition-colors ${activeFolder === folderFilter(item.name) ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:bg-accent/55 hover:text-foreground'}`}><Folder className="size-3.5 shrink-0" /><span className="truncate">{item.name}</span><span className="ml-auto font-mono text-[length:var(--font-size-meta)]">{item.count}</span></button>{item.record ? <DropdownMenu><DropdownMenuTrigger asChild><IconButton label={`Manage ${item.name}`} size="icon-xs" className="opacity-0 group-hover:opacity-100 focus-visible:opacity-100 data-[state=open]:opacity-100"><MoreHorizontal /></IconButton></DropdownMenuTrigger><DropdownMenuContent align="end"><DropdownMenuItem onSelect={() => openFolderRename(item.record!)}><Edit3 />Rename folder</DropdownMenuItem><DropdownMenuSeparator /><DropdownMenuItem tone="destructive" onSelect={() => openFolderDelete(item)}><FolderX />Delete folder</DropdownMenuItem></DropdownMenuContent></DropdownMenu> : null}</div>)}
        </div>
        <div className="border-t border-border p-3"><p className="text-[length:var(--font-size-meta)] font-semibold text-muted-foreground">Popular tags</p><div className="mt-2 flex flex-wrap gap-1">{tags.slice(0, 6).map(([tag]) => <button key={tag} type="button" onClick={() => setTagFilter(tag)} className={`rounded-md border px-1.5 py-0.5 text-[10px] transition-colors ${tagFilter === tag ? 'border-primary/40 bg-accent text-primary' : 'border-border text-muted-foreground hover:text-foreground'}`}>#{tag}</button>)}</div></div>
      </aside>
      <section className={`${shareToken ? 'hidden' : 'flex'} min-h-0 flex-col overflow-hidden border-r border-border bg-surface/15 max-md:border-r-0 ${compactPane === 'detail' ? 'max-md:hidden' : ''}`}>
        <div className="flex h-9 shrink-0 items-center gap-2 border-b border-border px-3 text-[length:var(--font-size-meta)] text-muted-foreground"><span>{currentFolderLabel}</span>{tagFilter !== 'all' ? <button type="button" onClick={() => setTagFilter('all')} className="inline-flex items-center gap-1 rounded bg-accent px-1.5 py-0.5 text-primary">#{tagFilter}<X className="size-3" /></button> : null}</div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {saved.isLoading ? <EmptyState compact icon={<BookMarked />} title="Loading library" description={gateway.source === 'mock' ? 'Preparing the opt-in mock query library…' : 'Reading persisted saved queries from DataDock API…'} /> : null}
          {saved.isError && !saved.data ? <EmptyState compact icon={<AlertCircle />} title="Saved queries unavailable" description={errorMessage(saved.error, 'The library could not be loaded.')} actions={<Button size="sm" variant="outline" onClick={() => saved.refetch()}>Retry</Button>} /> : null}
          {!saved.isLoading && !saved.isError && !records.length ? <EmptyState compact icon={<FolderHeart />} title="No matching queries" description="Try another folder or create a saved query." actions={<Button size="sm" onClick={openCreate}><FilePlus2 />New query</Button>} /> : null}
          <div className="divide-y divide-grid-line">{records.map((item) => <button key={item.id} type="button" onClick={() => { setSelectedId(item.id); setCompactPane('detail') }} className={`group block w-full px-3.5 py-3 text-left outline-none transition-colors hover:bg-accent/35 focus-visible:ring-2 focus-visible:ring-ring/30 ${selected?.id === item.id ? 'bg-accent/65' : ''}`}><div className="flex items-start gap-2"><div className="min-w-0 flex-1"><div className="flex items-center gap-1.5"><Star className={`size-3.5 shrink-0 ${item.favorite ? 'fill-amber-400 text-amber-400' : 'text-muted-foreground/45'}`} /><p className="truncate text-[length:var(--font-size-ui)] font-semibold text-foreground">{item.title}</p></div><p className="mt-1.5 line-clamp-2 font-mono text-[length:var(--font-size-meta)] leading-5 text-muted-foreground">{item.sql}</p></div><Badge variant="outline" className="max-w-24 truncate">{item.folder}</Badge></div><div className="mt-2 flex items-center gap-1"><div className="flex min-w-0 flex-1 gap-1 overflow-hidden">{item.tags.slice(0, 3).map((tag) => <span key={tag} className="shrink-0 rounded bg-background/75 px-1.5 py-0.5 text-[10px] text-muted-foreground">#{tag}</span>)}</div><span className="shrink-0 text-[10px] text-muted-foreground">{dateLabel(item.updatedAt)}</span></div></button>)}</div>
        </div>
      </section>
      <main className={`min-h-0 min-w-0 overflow-y-auto bg-background p-4 lg:p-5 ${compactPane === 'list' ? 'max-md:hidden' : ''}`}>
        {shareToken ? publicShare.isLoading ? <EmptyState icon={<LoaderCircle className="animate-spin" />} title="Opening shared query" description={gateway.source === 'mock' ? 'Resolving this link from the opt-in mock gateway…' : 'Validating this public link with DataDock API…'} /> : publicShare.isError ? <EmptyState icon={<Link2Off />} title="Shared query not found" description={errorMessage(publicShare.error, 'This link is invalid, expired, or belongs to another DataDock instance.')} actions={<Button size="sm" variant="outline" onClick={closePublicShare}><ArrowLeft />Back to query library</Button>} /> : publicShare.data ? <PublicShareDetail query={publicShare.data} connection={connection} copied={copied} copyError={copyError} onBack={closePublicShare} onCopy={() => copySql(publicShare.data.sql)} onOpenQuery={onOpenQuery} /> : null : selected ? <div className="mx-auto flex min-h-full max-w-5xl flex-col">
          <Button size="xs" variant="ghost" className="mb-3 w-fit md:hidden" onClick={() => setCompactPane('list')}><ArrowLeft />Back to saved queries</Button>
          <div className="flex flex-wrap items-start gap-3"><div className="grid size-10 shrink-0 place-items-center rounded-xl border border-primary/25 bg-accent text-primary"><Tag className="size-[1.125rem]" /></div><div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><h2 className="truncate text-lg font-semibold tracking-[-0.025em]">{selected.title}</h2><Badge variant="outline">{selected.folder}</Badge></div><p className="mt-1 text-[length:var(--font-size-meta)] text-muted-foreground">Updated {dateLabel(selected.updatedAt)} · {selectedConnectionLabel}</p></div><div className="flex gap-1"><Tooltip><TooltipTrigger asChild><IconButton label={selected.favorite ? 'Remove favorite' : 'Add favorite'} variant="outline" disabled={favoriteQuery.isPending} onClick={() => favoriteQuery.mutate({ id: selected.id, favorite: !selected.favorite })}><Star className={selected.favorite ? 'fill-amber-400 text-amber-400' : ''} /></IconButton></TooltipTrigger><TooltipContent>{selected.favorite ? 'Remove favorite' : 'Add favorite'}</TooltipContent></Tooltip><DropdownMenu><DropdownMenuTrigger asChild><IconButton label="Query actions" variant="outline"><MoreHorizontal /></IconButton></DropdownMenuTrigger><DropdownMenuContent align="end"><DropdownMenuItem onSelect={openEdit}><Edit3 />Edit query</DropdownMenuItem><DropdownMenuItem disabled={duplicateQuery.isPending} onSelect={() => duplicateQuery.mutate(selected.id)}><Copy />Duplicate</DropdownMenuItem><DropdownMenuSeparator /><DropdownMenuItem tone="destructive" onSelect={() => { deleteQuery.reset(); setDeleteQueryOpen(true) }}><Trash2 />Delete query</DropdownMenuItem></DropdownMenuContent></DropdownMenu></div></div>
          <div className="mt-4 flex flex-wrap gap-1.5">{selected.tags.map((tag) => <button key={tag} type="button" onClick={() => setTagFilter(tag)}><Badge variant={tagFilter === tag ? 'accent' : 'outline'}>#{tag}</Badge></button>)}</div>
          <div className="mt-4 flex min-h-[18rem] flex-1 flex-col overflow-hidden rounded-xl border border-border bg-[#070a11]"><div className="flex h-9 shrink-0 items-center gap-2 border-b border-border px-3 text-[length:var(--font-size-meta)] text-muted-foreground"><BookMarked className="size-3.5 text-primary" /><span>Saved SQL</span><span className="ml-auto font-mono">{selected.sql.split('\n').length} lines</span></div><pre className="min-h-0 flex-1 overflow-auto p-4 font-mono text-[length:var(--font-size-data)] leading-6 text-foreground"><code>{selected.sql}</code></pre></div>
          <PropertyGrid className="mt-4"><PropertyField label="Folder"><div className="rounded-md border border-border bg-surface px-3 py-2 text-[length:var(--font-size-ui)]">{selected.folder}</div></PropertyField><PropertyField label="Created"><div className="rounded-md border border-border bg-surface px-3 py-2 text-[length:var(--font-size-ui)]">{dateLabel(selected.createdAt)}</div></PropertyField><PropertyField label="Sharing"><button type="button" onClick={openShare} className="flex w-full items-center gap-2 rounded-md border border-border bg-surface px-3 py-2 text-left text-[length:var(--font-size-ui)] text-foreground">{selectedShare.isLoading ? <LoaderCircle className="size-3.5 animate-spin text-muted-foreground" /> : selectedShare.data ? <Link2 className="size-3.5 text-info-foreground" /> : <LockKeyhole className="size-3.5 text-muted-foreground" />}{selectedShare.isLoading ? 'Checking link…' : selectedShare.isError ? 'Sharing unavailable' : selectedShare.data ? 'Public link active' : 'Private'}</button></PropertyField></PropertyGrid>
          {copyError ? <div role="alert" className="mt-4 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-ui)] text-destructive">{copyError}</div> : null}
          <div className="mt-4 flex flex-wrap items-center gap-2"><Button size="sm" disabled={!selected.connectionId && !connection} onClick={() => onOpenQuery(selected.sql, selected.connectionId ?? connection?.id)}><Play />Open in SQL</Button><Button size="sm" variant="outline" onClick={() => copySql(selected.sql)}>{copied ? <Check /> : <Copy />}{copied ? 'Copied' : 'Copy SQL'}</Button><Button size="sm" variant="ghost" onClick={openEdit}><Edit3 />Edit</Button><Button size="sm" variant="ghost" onClick={openShare}><Share2 />Share</Button></div>
        </div> : <EmptyState icon={<FolderHeart />} title="Select a saved query" description="Preview, edit or open reusable SQL without leaving the library." />}
      </main>
    </div>

    <Dialog open={editorMode !== null} onOpenChange={(open) => { if (!open) { setEditorMode(null); saveQuery.reset() } }}>
      <DialogContent className="max-w-2xl">
        <DialogHeader><DialogTitle>{editorMode === 'create' ? 'Create saved query' : 'Edit saved query'}</DialogTitle><DialogDescription>{editorMode === 'create' && !connection ? `This query will be saved without a pinned connection by the active ${gateway.source === 'mock' ? 'mock gateway' : 'DataDock API instance'}.` : `Changes are persisted by the active ${gateway.source === 'mock' ? 'mock gateway for this browser session' : 'DataDock API instance'}.`}</DialogDescription></DialogHeader>
        <div className="grid gap-4 sm:grid-cols-2">
          <PropertyField label="Title" htmlFor="query-library-title" required error={draft.title.trim() ? undefined : 'Enter a title'}><Input id="query-library-title" autoFocus value={draft.title} onChange={(event) => setDraft((current) => ({ ...current, title: event.target.value }))} placeholder="Daily active accounts" /></PropertyField>
          <PropertyField label="Folder" htmlFor="query-library-folder"><Input id="query-library-folder" list="saved-query-folder-options" value={draft.folder} onChange={(event) => setDraft((current) => ({ ...current, folder: event.target.value }))} placeholder="General" /><datalist id="saved-query-folder-options">{Array.from(new Set(['General', ...folders.map((item) => item.name)])).map((name) => <option key={name} value={name} />)}</datalist></PropertyField>
          <PropertyField className="sm:col-span-2" label="Tags" htmlFor="query-library-tags" description="Comma separated. Tags are normalized and deduplicated before saving."><Input id="query-library-tags" value={draft.tags} onChange={(event) => setDraft((current) => ({ ...current, tags: event.target.value }))} placeholder="customers, retention, weekly" /></PropertyField>
          <PropertyField className="sm:col-span-2" label="SQL" htmlFor="query-library-sql" required error={draft.sql.trim() ? undefined : 'Enter SQL'}><Textarea id="query-library-sql" spellCheck={false} value={draft.sql} onChange={(event) => setDraft((current) => ({ ...current, sql: event.target.value }))} className="min-h-56 bg-[#070a11] font-mono text-[length:var(--font-size-data)] leading-6" /></PropertyField>
        </div>
        {saveQuery.isError ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-ui)] text-destructive">{errorMessage(saveQuery.error, 'The saved query could not be persisted.')}</div> : null}
        <DialogFooter><Button variant="outline" onClick={() => { setEditorMode(null); saveQuery.reset() }}>Cancel</Button><Button disabled={!draft.title.trim() || !draft.sql.trim() || saveQuery.isPending} onClick={submitQuery}>{saveQuery.isPending ? <LoaderCircle className="animate-spin" /> : editorMode === 'create' ? <FilePlus2 /> : <Check />}{saveQuery.isPending ? 'Saving…' : editorMode === 'create' ? 'Create query' : 'Save changes'}</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog open={deleteQueryOpen} onOpenChange={(open) => { setDeleteQueryOpen(open); if (!open) deleteQuery.reset() }}>
      <DialogContent><DialogHeader><DialogTitle>Delete “{selected?.title}”?</DialogTitle><DialogDescription>This removes the persisted saved query. Database data and query history are not affected.</DialogDescription></DialogHeader>{deleteQuery.isError ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-ui)] text-destructive">{errorMessage(deleteQuery.error, 'The saved query could not be deleted.')}</div> : null}<DialogFooter><Button variant="outline" onClick={() => { setDeleteQueryOpen(false); deleteQuery.reset() }}>Cancel</Button><Button variant="destructive" disabled={!selected || deleteQuery.isPending} onClick={() => selected && deleteQuery.mutate(selected.id)}>{deleteQuery.isPending ? <LoaderCircle className="animate-spin" /> : <Trash2 />}{deleteQuery.isPending ? 'Deleting…' : 'Delete query'}</Button></DialogFooter></DialogContent>
    </Dialog>

    <Dialog open={folderDialog !== null} onOpenChange={(open) => { if (!open) { setFolderDialog(null); setFolderTarget(null); mutateFolder.reset() } }}>
      <DialogContent><DialogHeader><DialogTitle>{folderDialog === 'create' ? 'Create folder' : `Rename “${folderTarget?.name ?? ''}”`}</DialogTitle><DialogDescription>{folderDialog === 'create' ? 'Empty folders remain available until you add or move a saved query into them.' : 'The API renames the folder and moves every query in one operation.'}</DialogDescription></DialogHeader><PropertyField label="Folder name" htmlFor="saved-query-folder-name" required error={folderName.trim() ? undefined : 'Enter a folder name'}><Input id="saved-query-folder-name" autoFocus value={folderName} onChange={(event) => setFolderName(event.target.value)} placeholder="Analytics" /></PropertyField>{mutateFolder.isError ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-ui)] text-destructive">{errorMessage(mutateFolder.error, 'The folder could not be saved.')}</div> : null}<DialogFooter><Button variant="outline" onClick={() => { setFolderDialog(null); setFolderTarget(null); mutateFolder.reset() }}>Cancel</Button><Button disabled={!folderName.trim() || mutateFolder.isPending} onClick={submitFolder}>{mutateFolder.isPending ? <LoaderCircle className="animate-spin" /> : folderDialog === 'create' ? <FolderPlus /> : <Edit3 />}{mutateFolder.isPending ? 'Saving…' : folderDialog === 'create' ? 'Create folder' : 'Rename folder'}</Button></DialogFooter></DialogContent>
    </Dialog>

    <Dialog open={Boolean(deleteFolderTarget)} onOpenChange={(open) => { if (!open) { setDeleteFolderTarget(null); deleteFolder.reset() } }}>
      <DialogContent>
        <DialogHeader><DialogTitle>Delete “{deleteFolderTarget?.name}”?</DialogTitle><DialogDescription>Choose how DataDock should handle the {deleteFolderTarget?.count ?? 0} saved queries currently assigned to this folder.</DialogDescription></DialogHeader>
        <div className="space-y-4">
          <PropertyField label="Delete policy" htmlFor="saved-query-folder-delete-policy"><Select id="saved-query-folder-delete-policy" value={deleteFolderPolicy} onChange={(event) => setDeleteFolderPolicy(event.target.value as 'reject' | 'move')}><option value="reject">Delete only when empty</option><option value="move">Move queries, then delete</option></Select></PropertyField>
          {deleteFolderPolicy === 'move' ? <PropertyField label="Move queries to" htmlFor="saved-query-folder-destination" error={deleteFolderDestination ? undefined : 'Create another folder before deleting this one'}><Select id="saved-query-folder-destination" value={deleteFolderDestination} onChange={(event) => setDeleteFolderDestination(event.target.value)}><option value="">Select destination</option>{destinationFolders.map((name) => <option key={name} value={name}>{name}</option>)}</Select></PropertyField> : null}
          {deleteFolderPolicy === 'reject' && (deleteFolderTarget?.count ?? 0) > 0 ? <div className="rounded-lg border border-warning/30 bg-warning/10 px-3 py-2 text-[length:var(--font-size-ui)] text-warning-foreground">This folder is not empty. Choose “Move queries, then delete” to continue safely.</div> : null}
          {deleteFolder.isError ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-ui)] text-destructive">{errorMessage(deleteFolder.error, 'The folder could not be deleted.')}</div> : null}
        </div>
        <DialogFooter><Button variant="outline" onClick={() => { setDeleteFolderTarget(null); deleteFolder.reset() }}>Cancel</Button><Button variant="destructive" disabled={deleteFolder.isPending || (deleteFolderPolicy === 'reject' && (deleteFolderTarget?.count ?? 0) > 0) || (deleteFolderPolicy === 'move' && !deleteFolderDestination)} onClick={submitFolderDelete}>{deleteFolder.isPending ? <LoaderCircle className="animate-spin" /> : <FolderX />}{deleteFolder.isPending ? 'Deleting…' : 'Delete folder'}</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog open={shareInfoOpen} onOpenChange={(open) => { setShareInfoOpen(open); if (!open) { setShareCopyError(''); enableShare.reset(); revokeShare.reset() } }}>
      <DialogContent>
        <DialogHeader><DialogTitle>Share “{selected?.title}”</DialogTitle><DialogDescription>{gateway.source === 'mock' ? 'Mock mode previews the share lifecycle only; session-created links do not survive reload or a new tab.' : 'The public link is created and revoked by DataDock API on this self-hosted instance.'}</DialogDescription></DialogHeader>
        <div className="space-y-4">
          {selectedShare.isLoading ? <div className="flex items-center gap-2 rounded-lg border border-border bg-surface p-3 text-[length:var(--font-size-ui)] text-muted-foreground"><LoaderCircle className="size-4 animate-spin" />Checking current share state…</div> : selectedShare.isError ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-[length:var(--font-size-ui)] text-destructive"><p>{errorMessage(selectedShare.error, 'Share state could not be loaded.')}</p><Button size="xs" variant="ghost" className="mt-2" onClick={() => selectedShare.refetch()}>Retry</Button></div> : selectedShare.data ? <><div className="flex items-center gap-2"><Badge variant="info"><Link2 />Public link active</Badge><span className="text-[length:var(--font-size-meta)] text-muted-foreground">created {dateTimeLabel(selectedShare.data.createdAt)}</span></div><PropertyField label="Public share link" htmlFor="saved-query-share-link"><div className="flex gap-2"><Input id="saved-query-share-link" readOnly value={activeShareUrl} className="font-mono text-[length:var(--font-size-meta)]" /><Button variant="outline" onClick={() => copyShare(activeShareUrl, 'link')}>{shareCopied === 'link' ? <Check /> : <Copy />}{shareCopied === 'link' ? 'Copied' : 'Copy'}</Button></div></PropertyField><p className="text-[length:var(--font-size-meta)] text-muted-foreground">{selectedShare.data.expiresAt ? `Expires ${dateTimeLabel(selectedShare.data.expiresAt)}` : 'This link does not expire automatically.'}</p></> : <div className="rounded-lg border border-border bg-surface p-4"><div className="flex items-start gap-3"><div className="grid size-9 shrink-0 place-items-center rounded-lg bg-accent text-primary"><LockKeyhole className="size-4" /></div><div><p className="text-[length:var(--font-size-ui)] font-semibold">Private saved query</p><p className="mt-1 text-[length:var(--font-size-meta)] leading-5 text-muted-foreground">Create a public read-only link for the title, tags and SQL text. It never grants database access or executes SQL.</p></div></div></div>}
          {shareCopyError || enableShare.isError || revokeShare.isError ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-ui)] text-destructive">{shareCopyError || errorMessage(enableShare.error ?? revokeShare.error, 'The share action failed.')}</div> : null}
          <div className="rounded-lg border border-warning/30 bg-warning/10 p-3 text-[length:var(--font-size-ui)] text-amber-200"><p className="flex items-center gap-2 font-medium"><LockKeyhole className="size-4" />No authentication protection</p><p className="mt-1.5 leading-5 text-amber-200/80">Anyone who can reach this DataDock host and has the link can read the shared SQL. Keep DataDock on a trusted network.</p></div>
        </div>
        <DialogFooter className="sm:justify-between"><div className="flex gap-2">{selectedShare.data ? <Button variant="outline" disabled={revokeShare.isPending || !selected} onClick={() => selected && revokeShare.mutate(selected.id)}>{revokeShare.isPending ? <LoaderCircle className="animate-spin" /> : <Link2Off />}{revokeShare.isPending ? 'Revoking…' : 'Revoke link'}</Button> : <Button disabled={enableShare.isPending || selectedShare.isLoading || selectedShare.isError || !selected} onClick={() => selected && enableShare.mutate(selected.id)}>{enableShare.isPending ? <LoaderCircle className="animate-spin" /> : <Link2 />}{enableShare.isPending ? 'Creating…' : 'Create public link'}</Button>}{selected ? <Button variant="outline" onClick={() => copyShare(selected.sql, 'sql')}>{shareCopied === 'sql' ? <Check /> : <Copy />}{shareCopied === 'sql' ? 'SQL copied' : 'Copy SQL'}</Button> : null}</div><Button variant="ghost" onClick={() => setShareInfoOpen(false)}>Done</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </WorkspacePage>
}

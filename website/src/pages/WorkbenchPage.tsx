import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, AlertTriangle, Braces, ChevronRight, Database, Moon, PanelRightClose, PanelRightOpen, Rows3, Search, Settings, Sun, Table2 } from 'lucide-react'
import type { Connection } from '@/entities/connection'
import type { DatabaseObject } from '@/entities/database-object'
import type { CreateWorkspaceInput, UpdateWorkspaceInput, Workspace } from '@/entities/workspace'
import { useDataDockGateway } from '@/app/providers'
import { ActivityRail } from '@/widgets/app-shell/ActivityRail'
import { StatusBar } from '@/widgets/app-shell/StatusBar'
import { WorkbenchTabs } from '@/widgets/app-shell/WorkbenchTabs'
import type { ActivityId, ActivityItem, InspectorContext, WorkbenchTab } from '@/widgets/app-shell/shell-types'
import { ExplorerPane } from '@/widgets/explorer/ExplorerPane'
import { InspectorPane } from '@/widgets/inspector/InspectorPane'
import { ConnectionOnboarding } from '@/widgets/onboarding/ConnectionOnboarding'
import { WorkspaceFormDialog } from '@/features/workspaces/WorkspaceFormDialog'
import { WORKSPACE_COLORS } from '@/features/workspaces/WorkspaceMark'
import { APP_CONFIG } from '@/shared/config/constants'
import type { DataSource } from '@/shared/config/env'
import { APP_ROUTES } from '@/shared/config/routes'
import { Badge, Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, EmptyState, IconButton, TooltipProvider } from '@/shared/ui'

const CommandPalette = lazy(() => import('@/features/command-palette/CommandPalette').then((module) => ({ default: module.CommandPalette })))
const ConnectionsPage = lazy(() => import('@/pages/ConnectionsPage').then((module) => ({ default: module.ConnectionsPage })))
const TablePage = lazy(() => import('@/pages/TablePage').then((module) => ({ default: module.TablePage })))
const SchemaPage = lazy(() => import('@/pages/SchemaPage').then((module) => ({ default: module.SchemaPage })))
const QueryPage = lazy(() => import('@/pages/QueryPage').then((module) => ({ default: module.QueryPage })))
const QueryHistoryPage = lazy(() => import('@/pages/QueryHistoryPage').then((module) => ({ default: module.QueryHistoryPage })))
const SavedQueriesPage = lazy(() => import('@/pages/SavedQueriesPage').then((module) => ({ default: module.SavedQueriesPage })))
const OperationsPage = lazy(() => import('@/pages/OperationsPage').then((module) => ({ default: module.OperationsPage })))

const tableTab: WorkbenchTab = { id: 'table', label: 'public.users', kind: 'table', icon: Table2, closable: false }
const queryTab: WorkbenchTab = { id: 'query', label: APP_CONFIG.query.fileName, kind: 'query', icon: Braces, closable: true }
const operationsTab: WorkbenchTab = { id: 'operations', label: 'Operations', kind: 'operations', icon: Activity, closable: true }

function isBrowsableObject(object?: DatabaseObject): object is DatabaseObject {
  return object?.kind === 'table' || object?.kind === 'view' || object?.kind === 'materialized-view'
}

function catalogObjects(nodes: DatabaseObject[]): DatabaseObject[] {
  return nodes.flatMap((node) => [node, ...catalogObjects(node.children ?? [])])
}

function connectionTransport(connection: Connection) {
  if (connection.sshTunnel.enabled) return 'SSH tunnel'
  if (connection.proxyUrl) return 'Proxy'
  if (connection.sslMode !== 'disable') return `TLS · ${connection.sslMode}`
  return 'Direct'
}

function initialQuerySql(source: DataSource) {
  return source === 'mock' ? APP_CONFIG.query.mockInitialSql : APP_CONFIG.query.liveInitialSql
}

function queryDraftStorageKey(connectionId: string, source: DataSource) {
  return `${APP_CONFIG.query.draftStorageKey}:${source}:${connectionId}`
}

function loadQueryDraft(connectionId: string, source: DataSource) {
  try {
    const key = queryDraftStorageKey(connectionId, source)
    const scopedDraft = window.localStorage.getItem(key)
    if (source === 'api') {
      window.localStorage.removeItem(APP_CONFIG.query.draftStorageKey)
      return scopedDraft
    }
    if (scopedDraft !== null) {
      window.localStorage.removeItem(APP_CONFIG.query.draftStorageKey)
      return scopedDraft
    }
    const legacyDraft = window.localStorage.getItem(APP_CONFIG.query.draftStorageKey)
    if (legacyDraft !== null) {
      window.localStorage.setItem(key, legacyDraft)
      window.localStorage.removeItem(APP_CONFIG.query.draftStorageKey)
    }
    return legacyDraft
  } catch {
    return null
  }
}

function storeQueryDraft(connectionId: string, source: DataSource, sql: string) {
  try {
    window.localStorage.setItem(queryDraftStorageKey(connectionId, source), sql)
  } catch {
    return
  }
}

function WorkbenchLoader({ label }: { label: string }) {
  return <div className="grid h-full place-items-center bg-background text-[length:var(--font-size-ui)] text-muted-foreground">{label}</div>
}

function workspaceErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'The workspace action could not be completed.'
}

function initialActivity(): ActivityId {
  const path = window.location.pathname
  if (path.startsWith(APP_ROUTES.connections)) return 'connections'
  if (path.startsWith(APP_ROUTES.query)) return 'query'
  if (path.startsWith(APP_ROUTES.history)) return 'history'
  if (path.startsWith(APP_ROUTES.savedQueries)) return 'saved'
  if (path.startsWith(APP_ROUTES.operations)) return 'operations'
  if (path.startsWith(APP_ROUTES.settings)) return 'settings'
  return 'explorer'
}

export function WorkbenchPage() {
  const gateway = useDataDockGateway()
  const queryClient = useQueryClient()
  const createWorkspaceMutation = useMutation({ mutationFn: (input: CreateWorkspaceInput) => gateway.createWorkspace(input) })
  const updateWorkspaceMutation = useMutation({ mutationFn: ({ id, input }: { id: string; input: UpdateWorkspaceInput }) => gateway.updateWorkspace(id, input) })
  const deleteWorkspaceMutation = useMutation({ mutationFn: (id: string) => gateway.deleteWorkspace(id) })
  const reorderWorkspacesMutation = useMutation({ mutationFn: (ids: string[]) => gateway.reorderWorkspaces(ids) })
  const initialSection = initialActivity()
  const [theme, setTheme] = useState<'dark' | 'light'>('dark')
  const [activity, setActivity] = useState<ActivityId>(initialSection)
  const [activeWorkspaceId, setActiveWorkspaceId] = useState('')
  const [activeConnectionId, setActiveConnectionId] = useState('')
  const [selectedObject, setSelectedObject] = useState<DatabaseObject>()
  const [activeTable, setActiveTable] = useState('public.users')
  const [querySql, setQuerySql] = useState<string>(() => initialQuerySql(gateway.source))
  const [queryDraftConnectionId, setQueryDraftConnectionId] = useState('')
  const [querySessionKey, setQuerySessionKey] = useState(0)
  const [activeTabId, setActiveTabId] = useState(initialSection === 'query' ? 'query' : initialSection === 'operations' ? 'operations' : 'table')
  const [tabs, setTabs] = useState<WorkbenchTab[]>([tableTab, queryTab, operationsTab])
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [emptyWorkspaceCreateOpen, setEmptyWorkspaceCreateOpen] = useState(false)
  const [inspectorOpen, setInspectorOpen] = useState(false)
  const [tableDirty, setTableDirty] = useState(false)
  const [tableApplying, setTableApplying] = useState(false)
  const [schemaDirty, setSchemaDirty] = useState(false)
  const [queryBlocked, setQueryBlocked] = useState(false)
  const [connectionDirty, setConnectionDirty] = useState(false)
  const [navigationConfirmOpen, setNavigationConfirmOpen] = useState(false)
  const [navigationCleaning, setNavigationCleaning] = useState(false)
  const [navigationError, setNavigationError] = useState('')
  const [shellNotice, setShellNotice] = useState('')
  const [compactInspector, setCompactInspector] = useState(() => window.innerWidth < APP_CONFIG.layout.inspectorInlineMinWidth)
  const inspectorButtonRef = useRef<HTMLButtonElement>(null)
  const activeConnectionSnapshotRef = useRef<Connection | undefined>(undefined)
  const pendingNavigationRef = useRef<(() => void) | null>(null)
  const queryCleanupRef = useRef<(() => Promise<void>) | null>(null)
  const queryDraftOverrideRef = useRef<{ connectionId: string; sql: string } | null>(null)
  const querySqlRef = useRef(querySql)
  const shellNoticeTimerRef = useRef<number | undefined>(undefined)
  const workspaceBlocked = tableDirty || tableApplying || schemaDirty || queryBlocked || connectionDirty

  const workspaces = useQuery({ queryKey: ['workspaces'], queryFn: ({ signal }) => gateway.listWorkspaces(signal) })
  useEffect(() => {
    if (!activeWorkspaceId && workspaces.data?.length) setActiveWorkspaceId(workspaces.data[0].id)
  }, [activeWorkspaceId, workspaces.data])
  const connections = useQuery({ queryKey: ['connections', activeWorkspaceId], enabled: Boolean(activeWorkspaceId), queryFn: ({ signal }) => gateway.listConnections(activeWorkspaceId, signal) })
  const allConnections = useQuery({ queryKey: ['connections'], queryFn: ({ signal }) => gateway.listConnections(undefined, signal) })
  useEffect(() => {
    if (!connections.data?.length) {
      if (connections.isSuccess && activeConnectionId) requestNavigation(() => { setActiveConnectionId(''); setActiveTable(''); setSelectedObject(undefined) })
      return
    }
    if (!connections.data.some((connection) => connection.id === activeConnectionId)) {
      const preferred = connections.data.find((connection) => connection.status === 'connected') ?? connections.data[0]
      requestNavigation(() => { setActiveConnectionId(preferred.id); setActiveTable(''); setSelectedObject(undefined) })
    }
  }, [activeConnectionId, connections.data, connections.isSuccess, workspaceBlocked])
  const currentConnection = connections.data?.find((item) => item.id === activeConnectionId)
  if (currentConnection) activeConnectionSnapshotRef.current = currentConnection
  const activeConnection = currentConnection ?? ((workspaceBlocked || connections.isFetching) && activeConnectionSnapshotRef.current?.id === activeConnectionId ? activeConnectionSnapshotRef.current : undefined)
  const catalog = useQuery({ queryKey: ['catalog', activeConnectionId], enabled: Boolean(activeConnectionId), queryFn: ({ signal }) => gateway.listCatalog(activeConnectionId, signal) })
  const activeWorkspace = workspaces.data?.find((item) => item.id === activeWorkspaceId)
  const browsableObjects = useMemo(() => catalog.data?.connectionId === activeConnectionId ? catalogObjects(catalog.data.databases).filter(isBrowsableObject) : [], [activeConnectionId, catalog.data])
  querySqlRef.current = querySql

  useEffect(() => {
    if (!activeConnectionId || catalog.data?.connectionId !== activeConnectionId) return
    const current = browsableObjects.find((object) => object.qualifiedName === activeTable)
    const next = current ?? browsableObjects[0]
    if (!next) {
      if (activeTable) {
        const clearTable = () => setActiveTable('')
        if (activeTabId === 'table' || activeTabId === 'schema') requestNavigation(clearTable)
        else clearTable()
      }
      return
    }
    if (next.qualifiedName === activeTable) return
    const reconcileTable = () => {
      setActiveTable(next.qualifiedName)
      setTabs((currentTabs) => currentTabs.map((tab) => tab.id === 'table' ? { ...tab, label: next.qualifiedName } : tab))
    }
    if (activeTabId === 'table' || activeTabId === 'schema') requestNavigation(reconcileTable)
    else reconcileTable()
  }, [activeConnectionId, activeTabId, activeTable, browsableObjects, catalog.data?.connectionId, workspaceBlocked])

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    document.documentElement.dataset.density = APP_CONFIG.ui.defaultDensity
  }, [theme])
  useEffect(() => () => {
    if (shellNoticeTimerRef.current) window.clearTimeout(shellNoticeTimerRef.current)
  }, [])
  useEffect(() => {
    if (!activeConnectionId) {
      setQueryDraftConnectionId('')
      return
    }
    const override = queryDraftOverrideRef.current?.connectionId === activeConnectionId ? queryDraftOverrideRef.current : null
    queryDraftOverrideRef.current = null
    setQuerySql(override?.sql ?? loadQueryDraft(activeConnectionId, gateway.source) ?? initialQuerySql(gateway.source))
    setQueryDraftConnectionId(activeConnectionId)
    setQuerySessionKey((value) => value + 1)
  }, [activeConnectionId, gateway.source])
  useEffect(() => {
    const connectionId = activeConnectionId
    if (!connectionId) return
    return () => storeQueryDraft(connectionId, gateway.source, querySqlRef.current)
  }, [activeConnectionId, gateway.source])
  useEffect(() => {
    if (!activeConnectionId || queryDraftConnectionId !== activeConnectionId) return
    const timeout = window.setTimeout(() => {
      storeQueryDraft(activeConnectionId, gateway.source, querySql)
    }, APP_CONFIG.query.draftSaveDebounceMs)
    return () => window.clearTimeout(timeout)
  }, [activeConnectionId, gateway.source, queryDraftConnectionId, querySql])
  useEffect(() => {
    const media = window.matchMedia(`(max-width: ${APP_CONFIG.layout.inspectorInlineMinWidth - 1}px)`)
    const updateInspectorMode = () => setCompactInspector(media.matches)
    updateInspectorMode()
    media.addEventListener('change', updateInspectorMode)
    return () => media.removeEventListener('change', updateInspectorMode)
  }, [])
  useEffect(() => {
    if (compactInspector || !inspectorOpen) return
    const closeInspector = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented) return
      setInspectorOpen(false)
      requestAnimationFrame(() => inspectorButtonRef.current?.focus())
    }
    window.addEventListener('keydown', closeInspector)
    return () => window.removeEventListener('keydown', closeInspector)
  }, [compactInspector, inspectorOpen])

  const inspectorContext = useMemo<InspectorContext | undefined>(() => activeConnection ? ({ connectionId: activeConnection.id, connectionName: activeConnection.name, engine: activeConnection.engine, status: activeConnection.status, readOnly: activeConnection.readOnly, latencyMs: activeConnection.latencyMs, source: gateway.source, transport: connectionTransport(activeConnection), object: selectedObject }) : undefined, [activeConnection, gateway.source, selectedObject])
  useEffect(() => {
    if (!inspectorContext) setInspectorOpen(false)
  }, [inspectorContext])
  useEffect(() => {
    setTabs((current) => current.map((tab) => {
      const dirty = tab.id === 'table' ? tableDirty : tab.id === 'schema' ? schemaDirty : tab.id === 'query' ? queryBlocked : false
      return tab.dirty === dirty ? tab : { ...tab, dirty }
    }))
  }, [queryBlocked, schemaDirty, tableDirty])
  useEffect(() => {
    if (!workspaceBlocked) return
    const preventUnload = (event: BeforeUnloadEvent) => event.preventDefault()
    window.addEventListener('beforeunload', preventUnload)
    return () => window.removeEventListener('beforeunload', preventUnload)
  }, [workspaceBlocked])
  useEffect(() => {
    if (!navigationConfirmOpen || workspaceBlocked || !pendingNavigationRef.current) return
    const action = pendingNavigationRef.current
    pendingNavigationRef.current = null
    setNavigationConfirmOpen(false)
    action()
  }, [navigationConfirmOpen, workspaceBlocked])

  function requestNavigation(action: () => void) {
    if (!workspaceBlocked) {
      action()
      return
    }
    pendingNavigationRef.current = action
    setNavigationError('')
    setNavigationConfirmOpen(true)
  }

  async function confirmNavigation() {
    const action = pendingNavigationRef.current
    pendingNavigationRef.current = null
    if (queryBlocked) {
      const cleanup = queryCleanupRef.current
      if (!cleanup) {
        pendingNavigationRef.current = action
        setNavigationError('The SQL session is still initializing. Try again in a moment.')
        return
      }
      setNavigationCleaning(true)
      setNavigationError('')
      try {
        await cleanup()
      } catch (error) {
        pendingNavigationRef.current = action
        setNavigationCleaning(false)
        setNavigationError(error instanceof Error ? error.message : 'The SQL session could not be cleaned up safely.')
        return
      }
      setNavigationCleaning(false)
    }
    setTableDirty(false)
    setTableApplying(false)
    setSchemaDirty(false)
    setConnectionDirty(false)
    setNavigationConfirmOpen(false)
    action?.()
  }

  const registerQueryCleanup = useCallback((cleanup: (() => Promise<void>) | null) => {
    queryCleanupRef.current = cleanup
  }, [])

  function showWorkspaceError(error: unknown) {
    setShellNotice(workspaceErrorMessage(error))
    if (shellNoticeTimerRef.current) window.clearTimeout(shellNoticeTimerRef.current)
    shellNoticeTimerRef.current = window.setTimeout(() => setShellNotice(''), 5_000)
  }

  function applyWorkspaceSelection(workspaceId: string) {
    activeConnectionSnapshotRef.current = undefined
    setActiveWorkspaceId(workspaceId)
    setActiveConnectionId('')
    setActiveTable('')
    setSelectedObject(undefined)
    setTabs((current) => current.map((tab) => tab.id === 'table' ? { ...tab, label: 'Loading table…' } : tab))
  }

  async function createWorkspace(input: CreateWorkspaceInput) {
    try {
      const created = await createWorkspaceMutation.mutateAsync(input)
      queryClient.setQueryData<Workspace[]>(['workspaces'], (current) => [...(current ?? []).filter((workspace) => workspace.id !== created.id), created].sort((left, right) => left.position - right.position))
      void queryClient.invalidateQueries({ queryKey: ['workspaces'] })
      requestNavigation(() => applyWorkspaceSelection(created.id))
    } catch (error) {
      showWorkspaceError(error)
      throw error
    }
  }

  async function updateWorkspace(id: string, input: UpdateWorkspaceInput) {
    const previous = queryClient.getQueryData<Workspace[]>(['workspaces'])
    queryClient.setQueryData<Workspace[]>(['workspaces'], (current) => current?.map((workspace) => workspace.id === id ? { ...workspace, ...input } : workspace))
    try {
      const updated = await updateWorkspaceMutation.mutateAsync({ id, input })
      queryClient.setQueryData<Workspace[]>(['workspaces'], (current) => current?.map((workspace) => workspace.id === id ? updated : workspace))
    } catch (error) {
      if (previous) queryClient.setQueryData(['workspaces'], previous)
      showWorkspaceError(error)
      throw error
    } finally {
      void queryClient.invalidateQueries({ queryKey: ['workspaces'] })
    }
  }

  async function reorderWorkspaces(ids: string[]) {
    const previous = queryClient.getQueryData<Workspace[]>(['workspaces'])
    const positions = new Map(ids.map((id, position) => [id, position]))
    queryClient.setQueryData<Workspace[]>(['workspaces'], (current) => current ? [...current].sort((left, right) => (positions.get(left.id) ?? left.position) - (positions.get(right.id) ?? right.position)).map((workspace, position) => ({ ...workspace, position })) : current)
    try {
      await reorderWorkspacesMutation.mutateAsync(ids)
    } catch (error) {
      if (previous) queryClient.setQueryData(['workspaces'], previous)
      showWorkspaceError(error)
      throw error
    } finally {
      void queryClient.invalidateQueries({ queryKey: ['workspaces'] })
    }
  }

  async function deleteWorkspaceNow(id: string) {
    const previous = queryClient.getQueryData<Workspace[]>(['workspaces']) ?? []
    const deletedIndex = previous.findIndex((workspace) => workspace.id === id)
    const remaining = previous.filter((workspace) => workspace.id !== id).map((workspace, position) => ({ ...workspace, position }))
    const cachedConnections = queryClient.getQueryData<Connection[]>(['connections']) ?? allConnections.data ?? []
    const deletedConnectionIds = cachedConnections.filter((connection) => connection.workspaceId === id).map((connection) => connection.id)
    try {
      await deleteWorkspaceMutation.mutateAsync(id)
      if (remaining.length) {
        try {
          await gateway.reorderWorkspaces(remaining.map((workspace) => workspace.id))
        } catch {
          void queryClient.invalidateQueries({ queryKey: ['workspaces'] })
        }
      }
      queryClient.setQueryData<Workspace[]>(['workspaces'], remaining)
      queryClient.setQueryData<Connection[]>(['connections'], (current) => current?.filter((connection) => connection.workspaceId !== id))
      queryClient.removeQueries({ queryKey: ['connections', id], exact: true })
      deletedConnectionIds.forEach((connectionId) => queryClient.removeQueries({ queryKey: ['catalog', connectionId] }))
      void queryClient.invalidateQueries({ queryKey: ['workspaces'] })
      void queryClient.invalidateQueries({ queryKey: ['connections'] })
      if (id === activeWorkspaceId) {
        const fallback = remaining[Math.min(Math.max(deletedIndex, 0), remaining.length - 1)]
        applyWorkspaceSelection(fallback?.id ?? '')
      }
    } catch (error) {
      showWorkspaceError(error)
      throw error
    }
  }

  async function deleteWorkspace(id: string) {
    if (id === activeWorkspaceId && workspaceBlocked) {
      requestNavigation(() => { void deleteWorkspaceNow(id).catch(() => undefined) })
      return
    }
    await deleteWorkspaceNow(id)
  }

  function setConnection(connection: Connection, browse = false) {
    const applyConnection = () => {
      const switchingConnection = connection.id !== activeConnectionId
      activeConnectionSnapshotRef.current = connection
      setActiveConnectionId(connection.id)
      if (switchingConnection) {
        setSelectedObject(undefined)
        setActiveTable('')
        setTabs((current) => current.map((tab) => tab.id === 'table' ? { ...tab, label: 'Loading table…' } : tab))
      }
      if (browse) {
        setActivity('explorer')
        setActiveTabId('table')
        window.history.replaceState(null, '', APP_ROUTES.home)
      }
    }
    if (connection.id === activeConnectionId && !browse) applyConnection()
    else requestNavigation(applyConnection)
  }

  function openObject(object: DatabaseObject) {
    const action = () => {
      setSelectedObject(object)
      if (object.kind === 'table' || object.kind === 'view' || object.kind === 'materialized-view') {
        setActiveTable(object.qualifiedName)
        setActiveTabId('table')
        setActivity('explorer')
        window.history.replaceState(null, '', APP_ROUTES.home)
        setTabs((current) => current.map((tab) => tab.id === 'table' ? { ...tab, label: object.qualifiedName } : tab))
      }
    }
    if ((object.kind === 'table' || object.kind === 'view' || object.kind === 'materialized-view') && object.qualifiedName !== activeTable) requestNavigation(action)
    else action()
  }

  function selectActivity(item: ActivityItem) {
    const action = () => {
      if (item.id === 'query') setTabs((current) => current.some((tab) => tab.id === queryTab.id) ? current : [...current, queryTab])
      if (item.id === 'operations') setTabs((current) => current.some((tab) => tab.id === operationsTab.id) ? current : [...current, operationsTab])
      setActivity(item.id)
      window.history.replaceState(null, '', item.route)
      if (item.id === 'explorer') setActiveTabId('table')
      if (item.id === 'query') setActiveTabId('query')
      if (item.id === 'operations') setActiveTabId('operations')
    }
    if (item.id === activity && !(item.id === 'explorer' && activeTabId !== 'table')) action()
    else requestNavigation(action)
  }

  function openConnections() {
    requestNavigation(() => {
      setActivity('connections')
      window.history.replaceState(null, '', APP_ROUTES.connections)
    })
  }

  function openQuery(sql?: string, afterNavigate?: () => void) {
    const queryAlreadyFocused = activity === 'query' && activeTabId === 'query'
    const replacesSql = sql !== undefined && sql !== querySql
    if (queryAlreadyFocused && !replacesSql) {
      afterNavigate?.()
      return
    }
    requestNavigation(() => {
      if (sql !== undefined) {
        setQuerySql(sql)
        setQuerySessionKey((value) => value + 1)
      }
      setTabs((current) => current.some((tab) => tab.id === 'query') ? current : [...current, queryTab])
      setActiveTabId('query')
      setActivity('query')
      window.history.replaceState(null, '', APP_ROUTES.query)
      afterNavigate?.()
    })
  }

  function openLibraryQuery(sql: string, connectionId?: string) {
    if (!connectionId || connectionId === activeConnectionId) {
      openQuery(sql)
      return
    }
    const target = allConnections.data?.find((connection) => connection.id === connectionId)
    if (!target) {
      setShellNotice(allConnections.isLoading ? 'Connection profiles are still loading. Try opening this query again in a moment.' : 'The connection saved with this query is unavailable. The current database was not changed.')
      if (shellNoticeTimerRef.current) window.clearTimeout(shellNoticeTimerRef.current)
      shellNoticeTimerRef.current = window.setTimeout(() => setShellNotice(''), 5_000)
      return
    }
    setShellNotice('')
    requestNavigation(() => {
      queryDraftOverrideRef.current = { connectionId: target.id, sql }
      activeConnectionSnapshotRef.current = target
      setActiveWorkspaceId(target.workspaceId)
      setActiveConnectionId(target.id)
      setActiveTable('')
      setSelectedObject(undefined)
      setTabs((current) => current.some((tab) => tab.id === 'query') ? current : [...current, queryTab])
      setActiveTabId('query')
      setActivity('query')
      window.history.replaceState(null, '', APP_ROUTES.query)
    })
  }

  function openSchema(reference = activeTable, afterNavigate?: () => void) {
    requestNavigation(() => {
      const schemaTab: WorkbenchTab = { id: 'schema', label: `${reference.split('.').at(-1) ?? reference} structure`, kind: 'schema', icon: Rows3, closable: true }
      setActiveTable(reference)
      setTabs((current) => current.some((tab) => tab.id === 'schema') ? current.map((tab) => tab.id === 'schema' ? schemaTab : tab) : [...current, schemaTab])
      setActiveTabId('schema')
      setActivity('explorer')
      window.history.replaceState(null, '', APP_ROUTES.home)
      afterNavigate?.()
    })
  }

  function closeInspector() {
    setInspectorOpen(false)
    if (!compactInspector) requestAnimationFrame(() => inspectorButtonRef.current?.focus())
  }

  function openDataFromInspector() {
    const reference = isBrowsableObject(selectedObject) ? selectedObject.qualifiedName : activeTable
    if (!reference) return
    requestNavigation(() => {
      setActiveTable(reference)
      setTabs((current) => current.map((tab) => tab.id === 'table' ? { ...tab, label: reference } : tab))
      setActiveTabId('table')
      setActivity('explorer')
      window.history.replaceState(null, '', APP_ROUTES.home)
      closeInspector()
    })
  }

  function openQueryFromInspector() {
    openQuery(undefined, closeInspector)
  }

  function openSchemaFromInspector() {
    const reference = isBrowsableObject(selectedObject) ? selectedObject.qualifiedName : activeTable
    if (reference) openSchema(reference, closeInspector)
  }

  function generateSelectFromInspector() {
    const reference = isBrowsableObject(selectedObject) ? selectedObject.qualifiedName : activeTable
    if (reference) openQuery(`SELECT *\nFROM ${reference}\nLIMIT 100;`, closeInspector)
  }

  function selectWorkbenchTab(id: string) {
    const tab = tabs.find((item) => item.id === id)
    if (!tab) return
    if (id === activeTabId) return
    requestNavigation(() => {
      setActiveTabId(id)
      const nextActivity = tab.kind === 'query' ? 'query' : tab.kind === 'operations' ? 'operations' : 'explorer'
      setActivity(nextActivity)
      window.history.replaceState(null, '', nextActivity === 'query' ? APP_ROUTES.query : nextActivity === 'operations' ? APP_ROUTES.operations : APP_ROUTES.home)
    })
  }

  function closeWorkbenchTab(id: string) {
    const close = () => {
      setTabs((current) => current.filter((tab) => tab.id !== id))
      if (activeTabId === id) {
        setActiveTabId('table')
        setActivity('explorer')
        window.history.replaceState(null, '', APP_ROUTES.home)
      }
    }
    if (activeTabId === id) requestNavigation(close)
    else close()
  }

  const showExplorerPane = activity === 'explorer' || activity === 'query'
  const showWorkbenchTabs = activity === 'explorer' || activity === 'query' || activity === 'operations'
  const sharedQueryView = activity === 'saved' && Boolean(new URLSearchParams(window.location.search).get('share'))
  const currentLabel = activity === 'connections'
    ? 'Connections'
    : activity === 'history'
      ? 'Query history'
      : activity === 'saved'
        ? 'Saved queries'
        : activity === 'settings'
          ? 'Settings'
          : activeTabId === 'table'
            ? activeTable || 'Resolving table…'
            : activeTabId === 'schema'
              ? `${activeTable} structure`
              : activeTabId === 'query'
                ? APP_CONFIG.query.fileName
                : 'Operations'

  let content
  if (sharedQueryView) {
    content = <Suspense fallback={<WorkbenchLoader label="Opening shared query…" />}><SavedQueriesPage connection={activeConnection} onOpenQuery={openLibraryQuery} /></Suspense>
  } else if (workspaces.isLoading || (workspaces.isSuccess && workspaces.data.length > 0 && !activeWorkspaceId)) {
    content = <WorkbenchLoader label="Preparing workspace…" />
  } else if (workspaces.isError) {
    content = <EmptyState icon={<AlertTriangle />} title="Workspaces could not be loaded" description={workspaces.error instanceof Error ? workspaces.error.message : 'Try loading DataDock again.'} actions={<Button variant="outline" onClick={() => workspaces.refetch()}>Retry</Button>} />
  } else if (!activeWorkspaceId) {
    content = <EmptyState icon={<Database />} title="No workspace available" description="Create a workspace before adding database connections." actions={<Button onClick={() => setEmptyWorkspaceCreateOpen(true)}>Create workspace</Button>} />
  } else if (activity === 'connections') {
    content = connections.isLoading ? <WorkbenchLoader label="Loading Connection Manager…" /> : connections.isError ? <EmptyState icon={<AlertTriangle />} title="Connection profiles could not be loaded" description={connections.error instanceof Error ? connections.error.message : 'Try loading the workspace again.'} actions={<Button variant="outline" onClick={() => connections.refetch()}>Retry</Button>} /> : <Suspense fallback={<WorkbenchLoader label="Loading Connection Manager…" />}><ConnectionsPage workspaceId={activeWorkspaceId} connections={connections.data ?? []} activeConnectionId={activeConnectionId} onConnectionSelect={(connection) => setConnection(connection)} onConnectionDeleted={(id, next) => { if (activeConnectionId === id) { setConnectionDirty(false); activeConnectionSnapshotRef.current = next; setActiveConnectionId(next?.id ?? ''); setActiveTable('') } }} onDirtyChange={setConnectionDirty} /></Suspense>
  } else if (activity === 'history') {
    content = <Suspense fallback={<WorkbenchLoader label="Loading query history…" />}><QueryHistoryPage connection={activeConnection} onOpenQuery={openLibraryQuery} /></Suspense>
  } else if (activity === 'saved') {
    content = <Suspense fallback={<WorkbenchLoader label="Loading saved queries…" />}><SavedQueriesPage connection={activeConnection} onOpenQuery={openLibraryQuery} /></Suspense>
  } else if (activity === 'settings') {
    content = <EmptyState icon={<Settings />} title="Workspace settings" description="Configure appearance, editor behavior, result limits and local connection defaults." />
  } else if (!activeConnection) {
    content = <ConnectionOnboarding connections={connections.data ?? []} source={gateway.source} onNewConnection={openConnections} onSelectConnection={(connection) => setConnection(connection, true)} />
  } else if (activeTabId === 'query') {
    content = <Suspense fallback={<WorkbenchLoader label="Loading SQL workspace…" />}><QueryPage key={`${activeConnection.id}:${querySessionKey}`} connection={activeConnection} sql={querySql} onSqlChange={setQuerySql} onBlockingStateChange={setQueryBlocked} onCleanupRegistration={registerQueryCleanup} onOpenConnections={openConnections} /></Suspense>
  } else if (activeTabId === 'operations') {
    content = <Suspense fallback={<WorkbenchLoader label="Loading operations…" />}><OperationsPage connection={activeConnection} onOpenQuery={(sql) => openQuery(sql)} /></Suspense>
  } else if (activeTabId === 'schema') {
    content = activeTable ? <Suspense fallback={<WorkbenchLoader label="Loading Schema Studio…" />}><SchemaPage connection={activeConnection} table={activeTable} onDirtyChange={setSchemaDirty} /></Suspense> : <WorkbenchLoader label="Resolving database objects…" />
  } else {
    content = catalog.isError ? <EmptyState icon={<AlertTriangle />} title="Database catalog could not be loaded" description={catalog.error instanceof Error ? catalog.error.message : 'Check the connection and try again.'} actions={<Button variant="outline" onClick={() => catalog.refetch()}>Retry</Button>} /> : !activeTable ? catalog.isLoading ? <WorkbenchLoader label="Resolving database objects…" /> : <EmptyState icon={<Table2 />} title="No browsable tables" description="This connection does not expose a table, view, or materialized view yet." /> : <Suspense fallback={<WorkbenchLoader label="Loading table workspace…" />}><TablePage key={`${activeConnection.id}:${activeTable}`} connection={activeConnection} table={activeTable} onOpenSchema={() => openSchema(activeTable)} onDirtyChange={setTableDirty} onApplyingChange={setTableApplying} /></Suspense>
  }

  return <TooltipProvider>
    <a href="#main-content" className="sr-only fixed top-2 left-2 z-[100] rounded-md bg-primary px-3 py-2 text-primary-foreground focus:not-sr-only">Skip to workbench</a>
    <div className="flex h-dvh min-h-[620px] w-full overflow-hidden bg-background text-foreground">
      <ActivityRail active={activity} onSelect={selectActivity} onOpenCommandPalette={() => setPaletteOpen(true)} />
      {showExplorerPane ? <div className="hidden h-full w-[300px] shrink-0 border-r border-border xl:block"><ExplorerPane workspaces={workspaces.data ?? []} activeWorkspace={activeWorkspace} connections={connections.data ?? []} activeConnectionId={activeConnectionId} catalog={catalog.data} catalogLoading={catalog.isLoading} selectedObjectId={selectedObject?.id} onWorkspaceChange={(id) => { if (id !== activeWorkspaceId) requestNavigation(() => applyWorkspaceSelection(id)) }} onCreateWorkspace={createWorkspace} onUpdateWorkspace={updateWorkspace} onDeleteWorkspace={deleteWorkspace} onReorderWorkspaces={reorderWorkspaces} onNewConnection={openConnections} onOpenCommandPalette={() => setPaletteOpen(true)} onConnectionSelect={(connection) => setConnection(connection, true)} onObjectSelect={setSelectedObject} onOpenObject={openObject} onRefresh={() => { connections.refetch(); catalog.refetch() }} /></div> : null}
      <main id="main-content" className="relative z-0 isolate flex min-w-0 flex-1 flex-col overflow-hidden">
        <header className="flex h-[var(--toolbar-height)] shrink-0 items-center gap-2 border-b border-border bg-surface/88 px-4 backdrop-blur-xl">
          <Database className="size-4 text-primary" /><span className="truncate text-[length:var(--font-size-data)] text-muted-foreground">{activeConnection?.name ?? activeWorkspace?.name ?? 'DataDock Studio'}</span><ChevronRight className="size-3.5 text-muted-foreground/60" /><span className="truncate font-mono text-[length:var(--font-size-data)] text-foreground">{currentLabel}</span>
          <Badge variant={gateway.source === 'mock' ? 'accent' : 'success'} className="ml-2">{gateway.source === 'mock' ? 'Demo data' : 'Live API'}</Badge>
          <div className="ml-auto flex items-center gap-1"><IconButton label="Search everything" onClick={() => setPaletteOpen(true)}><Search /></IconButton><IconButton label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} theme`} onClick={() => setTheme((value) => value === 'dark' ? 'light' : 'dark')}>{theme === 'dark' ? <Sun /> : <Moon />}</IconButton><IconButton ref={inspectorButtonRef} label={inspectorOpen ? 'Close inspector' : 'Open inspector'} aria-pressed={inspectorOpen} disabled={!inspectorContext} onClick={() => setInspectorOpen((value) => !value)}>{inspectorOpen ? <PanelRightClose /> : <PanelRightOpen />}</IconButton></div>
        </header>
        {showWorkbenchTabs ? <WorkbenchTabs tabs={tabs} activeTabId={activeTabId} onSelect={selectWorkbenchTab} onClose={closeWorkbenchTab} onNewQuery={() => openQuery()} /> : null}
        <div className="min-h-0 min-w-0 flex-1 overflow-hidden">{content}</div>
        <StatusBar connectionName={activeConnection?.name} connected={activeConnection?.status === 'connected'} latencyMs={activeConnection?.latencyMs} message={shellNotice || (activeConnection ? `${activeConnection.name} · ${activeConnection.status}` : 'Choose a connection to begin')} />
      </main>
      {!compactInspector ? <div aria-hidden={!inspectorOpen || !inspectorContext} inert={!inspectorOpen || !inspectorContext} className="relative z-10 h-full shrink-0 overflow-hidden opacity-0 transition-[width,opacity] duration-200" style={{ width: inspectorOpen && inspectorContext ? APP_CONFIG.layout.inspectorWidth : 0, opacity: inspectorOpen && inspectorContext ? 1 : 0 }}><div className="h-full border-l border-border" style={{ width: APP_CONFIG.layout.inspectorWidth }}>{inspectorContext ? <InspectorPane context={inspectorContext} onClose={closeInspector} onOpenData={!selectedObject || isBrowsableObject(selectedObject) ? openDataFromInspector : undefined} onNewQuery={openQueryFromInspector} onOpenSchema={isBrowsableObject(selectedObject) ? openSchemaFromInspector : undefined} onGenerateSelect={isBrowsableObject(selectedObject) || !selectedObject ? generateSelectFromInspector : undefined} /> : null}</div></div> : null}
    </div>
    {compactInspector ? <Dialog open={inspectorOpen && Boolean(inspectorContext)} onOpenChange={setInspectorOpen}><DialogContent showCloseButton={false} aria-describedby={undefined} onCloseAutoFocus={(event) => { event.preventDefault(); inspectorButtonRef.current?.focus() }} className="top-0 right-0 bottom-0 left-auto h-dvh max-h-none w-[min(90vw,360px)] max-w-none translate-x-0 translate-y-0 gap-0 overflow-hidden rounded-none border-y-0 border-r-0 p-0 sm:p-0"><DialogTitle className="sr-only">Context inspector</DialogTitle>{inspectorContext ? <InspectorPane context={inspectorContext} onClose={closeInspector} onOpenData={!selectedObject || isBrowsableObject(selectedObject) ? openDataFromInspector : undefined} onNewQuery={openQueryFromInspector} onOpenSchema={isBrowsableObject(selectedObject) ? openSchemaFromInspector : undefined} onGenerateSelect={isBrowsableObject(selectedObject) || !selectedObject ? generateSelectFromInspector : undefined} /> : null}</DialogContent></Dialog> : null}
    <Dialog open={navigationConfirmOpen} onOpenChange={(open) => { if (navigationCleaning) return; setNavigationConfirmOpen(open); setNavigationError(''); if (!open) pendingNavigationRef.current = null }}>
      <DialogContent className="max-w-md">
        <DialogHeader><DialogTitle>{tableApplying ? 'Changes are still being applied' : queryBlocked ? 'Leave the active SQL session?' : connectionDirty ? 'Discard connection edits?' : 'Leave with staged changes?'}</DialogTitle><DialogDescription>{tableApplying ? 'Wait for the database mutation to finish before leaving this table. This prevents a write from completing after the editor is closed.' : queryBlocked ? 'A query is running or a transaction is active. DataDock will cancel the query and confirm rollback before leaving.' : connectionDirty ? 'The unsaved host, security, or pool settings in this connection draft will be discarded.' : schemaDirty ? 'Your unapplied schema draft will be discarded when you leave Schema Studio.' : 'Your unapplied table edits will be discarded when you leave this data workspace.'}</DialogDescription></DialogHeader>
        {navigationError ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-ui)] text-destructive">{navigationError}</div> : null}
        <DialogFooter><Button variant="outline" disabled={navigationCleaning} onClick={() => { pendingNavigationRef.current = null; setNavigationConfirmOpen(false); setNavigationError('') }}>{tableApplying ? 'Close and wait' : 'Stay here'}</Button><Button variant="destructive" disabled={tableApplying || navigationCleaning} onClick={() => void confirmNavigation()}>{tableApplying ? 'Applying…' : navigationCleaning ? 'Cleaning up…' : queryBlocked ? 'Leave and clean up' : 'Discard and continue'}</Button></DialogFooter>
      </DialogContent>
    </Dialog>
    <WorkspaceFormDialog open={emptyWorkspaceCreateOpen} mode="create" defaultColor={WORKSPACE_COLORS[0]} onOpenChange={setEmptyWorkspaceCreateOpen} onSubmit={createWorkspace} />
    <Suspense fallback={null}><CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} connections={connections.data ?? []} catalog={catalog.data} theme={theme} onThemeToggle={() => setTheme((value) => value === 'dark' ? 'light' : 'dark')} onActivitySelect={(id, route) => selectActivity({ id, route, label: id, icon: Database })} onConnectionSelect={(connection) => setConnection(connection, true)} onObjectSelect={openObject} onNewConnection={openConnections} onNewQuery={() => openQuery()} /></Suspense>
  </TooltipProvider>
}

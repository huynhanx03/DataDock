import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, Braces, ChevronRight, Database, FolderHeart, Moon, PanelRightClose, PanelRightOpen, Search, Settings, Sun, Table2 } from 'lucide-react'
import type { Connection, CreateConnectionInput, DatabaseEngine } from '@/entities/connection'
import type { DatabaseObject } from '@/entities/database-object'
import { useDataDockGateway } from '@/app/providers'
import { ActivityRail } from '@/widgets/app-shell/ActivityRail'
import { StatusBar } from '@/widgets/app-shell/StatusBar'
import { WorkbenchTabs } from '@/widgets/app-shell/WorkbenchTabs'
import type { ActivityId, ActivityItem, InspectorContext, WorkbenchTab } from '@/widgets/app-shell/shell-types'
import { ExplorerPane } from '@/widgets/explorer/ExplorerPane'
import { InspectorPane } from '@/widgets/inspector/InspectorPane'
import { ConnectionOnboarding } from '@/widgets/onboarding/ConnectionOnboarding'
import { APP_CONFIG } from '@/shared/config/constants'
import { Badge, Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, IconButton, Input, TooltipProvider } from '@/shared/ui'

const CommandPalette = lazy(() => import('@/features/command-palette/CommandPalette').then((module) => ({ default: module.CommandPalette })))
const TablePage = lazy(() => import('@/pages/TablePage').then((module) => ({ default: module.TablePage })))
const QueryPage = lazy(() => import('@/pages/QueryPage').then((module) => ({ default: module.QueryPage })))
const OperationsPage = lazy(() => import('@/pages/OperationsPage').then((module) => ({ default: module.OperationsPage })))

const tableTab: WorkbenchTab = { id: 'table', label: 'public.users', kind: 'table', icon: Table2, closable: false }
const queryTab: WorkbenchTab = { id: 'query', label: 'customers.sql', kind: 'query', icon: Braces, closable: true }
const operationsTab: WorkbenchTab = { id: 'operations', label: 'Operations', kind: 'operations', icon: Activity, closable: true }

function BlankWorkspace({ icon: Icon, title, description }: { icon: typeof Search; title: string; description: string }) {
  return <div className="grid h-full place-items-center bg-background"><div className="max-w-md text-center"><div className="mx-auto grid size-14 place-items-center rounded-2xl border border-border bg-surface text-primary shadow-panel"><Icon className="size-6" /></div><h1 className="mt-5 text-xl font-semibold tracking-[-0.025em]">{title}</h1><p className="mt-2 text-sm leading-6 text-muted-foreground">{description}</p></div></div>
}

function WorkbenchLoader({ label }: { label: string }) {
  return <div className="grid h-full place-items-center text-[length:var(--font-size-ui)] text-muted-foreground">{label}</div>
}

export function WorkbenchPage() {
  const gateway = useDataDockGateway()
  const queryClient = useQueryClient()
  const [theme, setTheme] = useState<'dark' | 'light'>('dark')
  const [activity, setActivity] = useState<ActivityId>('explorer')
  const [activeWorkspaceId, setActiveWorkspaceId] = useState('')
  const [activeConnectionId, setActiveConnectionId] = useState('')
  const [selectedObject, setSelectedObject] = useState<DatabaseObject>()
  const [activeTable, setActiveTable] = useState('public.users')
  const [activeTabId, setActiveTabId] = useState('table')
  const [tabs, setTabs] = useState<WorkbenchTab[]>([tableTab, queryTab, operationsTab])
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [inspectorOpen, setInspectorOpen] = useState(false)
  const [compactInspector, setCompactInspector] = useState(() => window.innerWidth < APP_CONFIG.layout.inspectorInlineMinWidth)
  const [connectionDialogOpen, setConnectionDialogOpen] = useState(false)
  const [preferredEngine, setPreferredEngine] = useState<DatabaseEngine>('postgresql')
  const [connectionName, setConnectionName] = useState('Local PostgreSQL')
  const [connectionHost, setConnectionHost] = useState<string>(APP_CONFIG.connection.defaultHost)
  const inspectorButtonRef = useRef<HTMLButtonElement>(null)

  const workspaces = useQuery({ queryKey: ['workspaces'], queryFn: ({ signal }) => gateway.listWorkspaces(signal) })
  useEffect(() => { if (!activeWorkspaceId && workspaces.data?.length) setActiveWorkspaceId(workspaces.data[0].id) }, [activeWorkspaceId, workspaces.data])
  const connections = useQuery({ queryKey: ['connections', activeWorkspaceId], enabled: Boolean(activeWorkspaceId), queryFn: ({ signal }) => gateway.listConnections(activeWorkspaceId, signal) })
  useEffect(() => {
    if (!activeConnectionId && connections.data?.length) {
      const preferred = connections.data.find((connection) => connection.status === 'connected') ?? connections.data[0]
      setActiveConnectionId(preferred.id)
    }
  }, [activeConnectionId, connections.data])
  const activeConnection = connections.data?.find((item) => item.id === activeConnectionId)
  const catalog = useQuery({ queryKey: ['catalog', activeConnectionId], enabled: Boolean(activeConnectionId), queryFn: ({ signal }) => gateway.listCatalog(activeConnectionId, signal) })
  const activeWorkspace = workspaces.data?.find((item) => item.id === activeWorkspaceId)

  const createConnection = useMutation({
    mutationFn: (input: CreateConnectionInput) => gateway.createConnection(input),
    onSuccess: async (connection) => {
      await queryClient.invalidateQueries({ queryKey: ['connections'] })
      setActiveConnectionId(connection.id)
      setConnectionDialogOpen(false)
    },
  })

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    document.documentElement.dataset.density = APP_CONFIG.ui.defaultDensity
  }, [theme])
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

  const inspectorContext = useMemo<InspectorContext | undefined>(() => activeConnection ? ({ connectionId: activeConnection.id, connectionName: activeConnection.name, engine: activeConnection.engine, status: activeConnection.status, readOnly: activeConnection.readOnly, latencyMs: activeConnection.latencyMs, object: selectedObject }) : undefined, [activeConnection, selectedObject])
  useEffect(() => { if (!inspectorContext) setInspectorOpen(false) }, [inspectorContext])

  function selectConnection(connection: Connection) {
    setActiveConnectionId(connection.id)
    setSelectedObject(undefined)
    setActivity('explorer')
    setActiveTabId('table')
  }

  function openObject(object: DatabaseObject) {
    setSelectedObject(object)
    if (object.kind === 'table' || object.kind === 'view' || object.kind === 'materialized-view') {
      setActiveTable(object.qualifiedName)
      setActiveTabId('table')
      setTabs((current) => current.map((tab) => tab.id === 'table' ? { ...tab, label: object.qualifiedName } : tab))
    }
  }

  function selectActivity(item: ActivityItem) {
    setActivity(item.id)
    if (item.id === 'query') setActiveTabId('query')
    if (item.id === 'operations') setActiveTabId('operations')
  }

  function openNewConnection(engine: DatabaseEngine = 'postgresql') {
    setPreferredEngine(engine)
    setConnectionDialogOpen(true)
  }

  function submitConnection() {
    if (!activeWorkspaceId) return
    createConnection.mutate({ workspaceId: activeWorkspaceId, name: connectionName, engine: preferredEngine as 'postgresql' | 'mysql' | 'mariadb', host: connectionHost, port: APP_CONFIG.databasePorts[preferredEngine], database: preferredEngine === 'postgresql' ? 'postgres' : 'app', username: preferredEngine === 'postgresql' ? 'postgres' : 'root', password: '', sslMode: 'disable', sslCaPath: '', sslCertPath: '', sslKeyPath: '', proxyUrl: '', sshTunnel: { enabled: false, host: '', port: APP_CONFIG.connection.sshDefaultPort, username: '', password: '', privateKeyPath: '', knownHostsPath: '' }, readOnly: false, autoReconnect: true, maxOpenConns: APP_CONFIG.connection.maxOpenConnections, maxIdleConns: APP_CONFIG.connection.maxIdleConnections, connMaxLifetimeSeconds: APP_CONFIG.connection.maxLifetimeSeconds })
  }

  function closeInspector() {
    setInspectorOpen(false)
    if (!compactInspector) requestAnimationFrame(() => inspectorButtonRef.current?.focus())
  }

  function openDataFromInspector() {
    setActiveTabId('table')
    setActivity('explorer')
    closeInspector()
  }

  function openQueryFromInspector() {
    setActiveTabId('query')
    setActivity('query')
    closeInspector()
  }

  let content = activeConnection ? <Suspense fallback={<WorkbenchLoader label="Loading table workspace…" />}><TablePage key={`${activeConnection.id}:${activeTable}`} connection={activeConnection} table={activeTable} /></Suspense> : <ConnectionOnboarding connections={connections.data ?? []} onNewConnection={openNewConnection} onSelectConnection={selectConnection} />
  if (activeTabId === 'query' && activeConnection) content = <Suspense fallback={<WorkbenchLoader label="Loading SQL workspace…" />}><QueryPage connection={activeConnection} /></Suspense>
  if (activeTabId === 'operations' && activeConnection) content = <Suspense fallback={<WorkbenchLoader label="Loading operations…" />}><OperationsPage connection={activeConnection} /></Suspense>
  if (activity === 'history') content = <BlankWorkspace icon={Search} title="Query history" description="Search, inspect and rerun every statement executed in this workspace." />
  if (activity === 'saved') content = <BlankWorkspace icon={FolderHeart} title="Saved queries" description="Organize important SQL into folders and share reusable workflows." />
  if (activity === 'settings') content = <BlankWorkspace icon={Settings} title="Workspace settings" description="Configure appearance, editors, result limits and connection defaults." />

  return <TooltipProvider>
    <div className="flex h-dvh min-h-[620px] w-full overflow-hidden bg-background text-foreground">
      <ActivityRail active={activity} onSelect={selectActivity} onOpenCommandPalette={() => setPaletteOpen(true)} />
      <div className="hidden h-full w-[300px] shrink-0 border-r border-border xl:block">
        <ExplorerPane workspaces={workspaces.data ?? []} activeWorkspace={activeWorkspace} connections={connections.data ?? []} activeConnectionId={activeConnectionId} catalog={catalog.data} catalogLoading={catalog.isLoading} selectedObjectId={selectedObject?.id} onWorkspaceChange={(id) => { setActiveWorkspaceId(id); setActiveConnectionId('') }} onNewConnection={() => openNewConnection()} onOpenCommandPalette={() => setPaletteOpen(true)} onConnectionSelect={selectConnection} onObjectSelect={setSelectedObject} onOpenObject={openObject} onRefresh={() => { connections.refetch(); catalog.refetch() }} />
      </div>
      <main className="relative z-0 isolate flex min-w-0 flex-1 flex-col overflow-hidden">
        <header className="flex h-[var(--toolbar-height)] shrink-0 items-center gap-2 border-b border-border bg-surface/88 px-4 backdrop-blur-xl">
          <Database className="size-4 text-primary" /><span className="truncate text-[length:var(--font-size-data)] text-muted-foreground">{activeConnection?.name ?? 'DataDock Studio'}</span><ChevronRight className="size-3.5 text-muted-foreground/60" /><span className="truncate font-mono text-xs text-foreground">{activeTabId === 'table' ? activeTable : activeTabId === 'query' ? 'customers.sql' : 'operations'}</span>
          <Badge variant={gateway.source === 'mock' ? 'accent' : 'success'} className="ml-2">{gateway.source === 'mock' ? 'Demo data' : 'Live API'}</Badge>
          <div className="ml-auto flex items-center gap-1"><IconButton label="Search everything" onClick={() => setPaletteOpen(true)}><Search /></IconButton><IconButton label="Toggle theme" onClick={() => setTheme((value) => value === 'dark' ? 'light' : 'dark')}>{theme === 'dark' ? <Sun /> : <Moon />}</IconButton><IconButton ref={inspectorButtonRef} label={inspectorOpen ? 'Close inspector' : 'Open inspector'} aria-pressed={inspectorOpen} disabled={!inspectorContext} onClick={() => setInspectorOpen((value) => !value)}>{inspectorOpen ? <PanelRightClose /> : <PanelRightOpen />}</IconButton></div>
        </header>
        <WorkbenchTabs tabs={tabs} activeTabId={activeTabId} onSelect={(id) => { setActiveTabId(id); setActivity(id === 'operations' ? 'operations' : id === 'query' ? 'query' : 'explorer') }} onClose={(id) => { setTabs((current) => current.filter((tab) => tab.id !== id)); if (activeTabId === id) setActiveTabId('table') }} onNewQuery={() => { if (!tabs.some((tab) => tab.id === 'query')) setTabs((current) => [...current, queryTab]); setActiveTabId('query'); setActivity('query') }} />
        <div className="min-h-0 min-w-0 flex-1 overflow-hidden">{content}</div>
        <StatusBar connectionName={activeConnection?.name} connected={activeConnection?.status === 'connected'} latencyMs={activeConnection?.latencyMs} message={activeConnection ? `Connected to ${activeConnection.database}` : 'Choose a connection to begin'} />
      </main>
      {!compactInspector ? <div aria-hidden={!inspectorOpen || !inspectorContext} inert={!inspectorOpen || !inspectorContext} className="relative z-10 h-full shrink-0 overflow-hidden opacity-0 transition-[width,opacity] duration-200" style={{ width: inspectorOpen && inspectorContext ? APP_CONFIG.layout.inspectorWidth : 0, opacity: inspectorOpen && inspectorContext ? 1 : 0 }}><div className="h-full border-l border-border" style={{ width: APP_CONFIG.layout.inspectorWidth }}>{inspectorContext ? <InspectorPane context={inspectorContext} onClose={closeInspector} onOpenData={openDataFromInspector} onNewQuery={openQueryFromInspector} /> : null}</div></div> : null}
    </div>
    {compactInspector ? <Dialog open={inspectorOpen && Boolean(inspectorContext)} onOpenChange={setInspectorOpen}><DialogContent showCloseButton={false} aria-describedby={undefined} onCloseAutoFocus={(event) => { event.preventDefault(); inspectorButtonRef.current?.focus() }} className="top-0 right-0 bottom-0 left-auto h-dvh max-h-none w-[min(90vw,360px)] max-w-none translate-x-0 translate-y-0 gap-0 overflow-hidden rounded-none border-y-0 border-r-0 p-0 sm:p-0"><DialogTitle className="sr-only">Context inspector</DialogTitle>{inspectorContext ? <InspectorPane context={inspectorContext} onClose={closeInspector} onOpenData={openDataFromInspector} onNewQuery={openQueryFromInspector} /> : null}</DialogContent></Dialog> : null}
    <Suspense fallback={null}><CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} connections={connections.data ?? []} catalog={catalog.data} theme={theme} onThemeToggle={() => setTheme((value) => value === 'dark' ? 'light' : 'dark')} onActivitySelect={(id) => setActivity(id)} onConnectionSelect={selectConnection} onObjectSelect={openObject} onNewConnection={() => openNewConnection()} onNewQuery={() => setActiveTabId('query')} /></Suspense>
    <Dialog open={connectionDialogOpen} onOpenChange={setConnectionDialogOpen}><DialogContent><DialogHeader><DialogTitle>New database connection</DialogTitle><DialogDescription>Create a clean local profile. Advanced SSL, proxy and SSH fields remain available through the live API adapter.</DialogDescription></DialogHeader><div className="grid gap-4"><div className="grid grid-cols-3 gap-2">{(['postgresql','mysql','mariadb'] as const).map((engine) => <button key={engine} type="button" onClick={() => setPreferredEngine(engine)} className={`rounded-lg border p-3 text-left transition-colors ${preferredEngine === engine ? 'border-primary bg-accent text-accent-foreground' : 'border-border bg-surface hover:bg-accent/45'}`}><span className="font-mono text-xs font-bold uppercase">{engine.slice(0,2)}</span><span className="mt-1 block text-xs capitalize">{engine}</span></button>)}</div><label className="grid gap-1.5 text-xs font-medium text-muted-foreground">Connection name<Input value={connectionName} onChange={(event) => setConnectionName(event.target.value)} /></label><label className="grid gap-1.5 text-xs font-medium text-muted-foreground">Host<Input value={connectionHost} onChange={(event) => setConnectionHost(event.target.value)} /></label></div><DialogFooter><Button variant="outline" onClick={() => setConnectionDialogOpen(false)}>Cancel</Button><Button onClick={submitConnection} disabled={createConnection.isPending}>{createConnection.isPending ? 'Creating…' : 'Create connection'}</Button></DialogFooter></DialogContent></Dialog>
  </TooltipProvider>
}

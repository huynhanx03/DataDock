import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Activity, AlertTriangle, CheckCircle2, Database, Plus, Radio, Server, XCircle } from 'lucide-react'
import type { Connection, ConnectionInput } from '@/entities/connection'
import { useDataDockGateway } from '@/app/providers'
import { ConnectionDetails, type ConnectionAction } from '@/features/connections/ConnectionDetails'
import { ConnectionForm, connectionToDraft, createConnectionDraft, firstConnectionErrorSection, validateConnectionInput, type ConnectionFormErrors, type ConnectionFormSection } from '@/features/connections/ConnectionForm'
import { ConnectionList } from '@/features/connections/ConnectionList'
import { Badge, Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, MasterDetailLayout, Tabs, TabsContent, TabsList, TabsTrigger, WorkspaceHeader, WorkspacePage, WorkspaceToolbar } from '@/shared/ui'

type ConnectionsPageProps = {
  workspaceId: string
  connections: Connection[]
  activeConnectionId?: string
  onConnectionSelect: (connection: Connection) => void
  onConnectionDeleted?: (connectionId: string, nextConnection?: Connection) => void
  onDirtyChange?: (dirty: boolean) => void
}

type Notice = { tone: 'success' | 'error' | 'info'; message: string }
type Confirmation = { kind: 'disconnect' | 'delete'; connection: Connection }

const formSections: Array<{ value: ConnectionFormSection; label: string }> = [
  { value: 'general', label: 'General' },
  { value: 'ssl', label: 'SSL' },
  { value: 'ssh', label: 'SSH' },
  { value: 'proxy', label: 'Proxy' },
  { value: 'pool', label: 'Pool' },
]

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'The connection action could not be completed.'
}

function connectionTestProfile(value: ConnectionInput) {
  return {
    engine: value.engine,
    host: value.host,
    port: value.port,
    database: value.database,
    username: value.username,
    password: value.password,
    clearPassword: value.clearPassword ?? false,
    sslMode: value.sslMode,
    sslCaPath: value.sslCaPath,
    sslCertPath: value.sslCertPath,
    sslKeyPath: value.sslKeyPath,
    proxyUrl: value.proxyUrl,
    clearProxyCredentials: value.clearProxyCredentials ?? false,
    sshTunnel: value.sshTunnel,
  }
}

export function ConnectionsPage({ workspaceId, connections, activeConnectionId, onConnectionSelect, onConnectionDeleted, onDirtyChange }: ConnectionsPageProps) {
  const gateway = useDataDockGateway()
  const queryClient = useQueryClient()
  const [selectedId, setSelectedId] = useState(activeConnectionId ?? connections[0]?.id ?? '')
  const [busyAction, setBusyAction] = useState<ConnectionAction>(null)
  const [notice, setNotice] = useState<Notice | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [createTab, setCreateTab] = useState<ConnectionFormSection>('general')
  const [createDraft, setCreateDraft] = useState(() => createConnectionDraft(workspaceId))
  const [createErrors, setCreateErrors] = useState<ConnectionFormErrors>({})
  const [createTestNotice, setCreateTestNotice] = useState<Notice | null>(null)
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null)
  const noticeTimer = useRef<number | undefined>(undefined)
  const createMutation = useMutation({ mutationFn: (input: ConnectionInput) => gateway.createConnection(input) })
  const updateMutation = useMutation({ mutationFn: ({ id, input }: { id: string; input: ConnectionInput }) => gateway.updateConnection(id, input) })
  const duplicateMutation = useMutation({ mutationFn: (id: string) => gateway.duplicateConnection(id) })
  const favoriteMutation = useMutation({ mutationFn: ({ id, favorite }: { id: string; favorite: boolean }) => gateway.setConnectionFavorite(id, favorite) })
  const connectMutation = useMutation({ mutationFn: (id: string) => gateway.connectConnection(id) })
  const disconnectMutation = useMutation({ mutationFn: (id: string) => gateway.disconnectConnection(id) })
  const savedTestMutation = useMutation({ mutationFn: (id: string) => gateway.testConnection(id) })
  const draftTestMutation = useMutation({ mutationFn: (input: ConnectionInput) => gateway.testConnectionDraft(input) })
  const deleteMutation = useMutation({ mutationFn: (id: string) => gateway.deleteConnection(id) })

  useEffect(() => {
    if (activeConnectionId) setSelectedId(activeConnectionId)
  }, [activeConnectionId])

  useEffect(() => {
    if (connections.some((connection) => connection.id === selectedId)) return
    setSelectedId(connections[0]?.id ?? '')
  }, [connections, selectedId])

  useEffect(() => {
    setCreateDraft(createConnectionDraft(workspaceId))
    setCreateErrors({})
    setCreateTestNotice(null)
  }, [workspaceId])

  useEffect(() => () => {
    if (noticeTimer.current) window.clearTimeout(noticeTimer.current)
  }, [])

  const selected = connections.find((connection) => connection.id === selectedId) ?? connections[0]
  const connectedCount = useMemo(() => connections.filter((connection) => connection.status === 'connected').length, [connections])
  const readonlyCount = useMemo(() => connections.filter((connection) => connection.readOnly).length, [connections])

  function showNotice(next: Notice) {
    if (noticeTimer.current) window.clearTimeout(noticeTimer.current)
    setNotice(next)
    noticeTimer.current = window.setTimeout(() => setNotice(null), 3_600)
  }

  function selectConnection(connection: Connection) {
    setSelectedId(connection.id)
    onConnectionSelect(connection)
  }

  function syncConnection(next: Connection, select = false, previousWorkspaceId = next.workspaceId) {
    const upsert = (items: Connection[]) => items.some((item) => item.id === next.id)
      ? items.map((item) => item.id === next.id ? next : item)
      : [...items, next]
    queryClient.setQueryData<Connection[]>(['connections'], (current) => current ? upsert(current) : current)
    queryClient.setQueryData<Connection[]>(['connections', next.workspaceId], (current) => current ? upsert(current) : current)
    if (previousWorkspaceId !== next.workspaceId) {
      queryClient.setQueryData<Connection[]>(['connections', previousWorkspaceId], (current) => current?.filter((item) => item.id !== next.id))
    }
    if (select) {
      setSelectedId(next.id)
      onConnectionSelect(next)
    } else if (selectedId === next.id) {
      onConnectionSelect(next)
    }
    void queryClient.invalidateQueries({ queryKey: ['connections'] })
  }

  function removeConnection(connection: Connection) {
    const connectionId = connection.id
    const remove = (items: Connection[]) => items.filter((item) => item.id !== connectionId)
    queryClient.setQueryData<Connection[]>(['connections'], (current) => current ? remove(current) : current)
    queryClient.setQueryData<Connection[]>(['connections', connection.workspaceId], (current) => current ? remove(current) : current)
    void queryClient.invalidateQueries({ queryKey: ['connections'] })
  }

  function openCreate() {
    const draft = createConnectionDraft(workspaceId)
    draft.name = connections.length ? `New ${draft.engine === 'postgresql' ? 'PostgreSQL' : 'database'} connection` : draft.name
    setCreateDraft(draft)
    setCreateErrors({})
    setCreateTestNotice(null)
    setCreateTab('general')
    setCreateOpen(true)
  }

  async function createConnection() {
    const errors = validateConnectionInput(createDraft)
    setCreateErrors(errors)
    if (Object.keys(errors).length) {
      setCreateTab(firstConnectionErrorSection(errors))
      setCreateTestNotice({ tone: 'error', message: 'Review the highlighted settings before creating this profile.' })
      return
    }
    setBusyAction('create')
    try {
      const connection = await createMutation.mutateAsync(createDraft)
      syncConnection(connection, true)
      setCreateOpen(false)
      showNotice({ tone: 'success', message: `${connection.name} was created.` })
    } catch (error) {
      showNotice({ tone: 'error', message: errorMessage(error) })
    } finally {
      setBusyAction(null)
    }
  }

  async function updateConnection(connection: Connection, value: ConnectionInput) {
    const errors = validateConnectionInput(value, { hasStoredSSHPassword: connection.sshTunnel.hasPassword })
    if (Object.keys(errors).length) {
      showNotice({ tone: 'error', message: 'Complete the required connection fields before saving.' })
      return false
    }
    setBusyAction('update')
    try {
      const updated = await updateMutation.mutateAsync({ id: connection.id, input: value })
      syncConnection(updated, false, connection.workspaceId)
      showNotice({ tone: 'success', message: `Saved changes to ${updated.name}.` })
      return true
    } catch (error) {
      showNotice({ tone: 'error', message: errorMessage(error) })
      return false
    } finally {
      setBusyAction(null)
    }
  }

  async function duplicateConnection(connection: Connection) {
    setBusyAction('duplicate')
    try {
      const duplicate = await duplicateMutation.mutateAsync(connection.id)
      syncConnection(duplicate, true)
      showNotice({ tone: 'success', message: `${connection.name} was duplicated.` })
    } catch (error) {
      showNotice({ tone: 'error', message: errorMessage(error) })
    } finally {
      setBusyAction(null)
    }
  }

  async function toggleFavorite(connection: Connection) {
    const favorite = !connection.favorite
    setBusyAction('favorite')
    try {
      const next = await favoriteMutation.mutateAsync({ id: connection.id, favorite })
      syncConnection(next)
      showNotice({ tone: 'info', message: favorite ? `${connection.name} added to favorites.` : `${connection.name} removed from favorites.` })
    } catch (error) {
      showNotice({ tone: 'error', message: errorMessage(error) })
    } finally {
      setBusyAction(null)
    }
  }

  async function connect(connection: Connection) {
    setBusyAction('connect')
    try {
      const next = await connectMutation.mutateAsync(connection.id)
      syncConnection(next)
      showNotice({ tone: 'success', message: next.latencyMs === undefined ? `${connection.name} connected.` : `${connection.name} connected in ${next.latencyMs} ms.` })
    } catch (error) {
      showNotice({ tone: 'error', message: errorMessage(error) })
      void queryClient.invalidateQueries({ queryKey: ['connections'] })
    } finally {
      setBusyAction(null)
    }
  }

  async function testConnection(connection: Connection, value: ConnectionInput) {
    const errors = validateConnectionInput(value, { hasStoredSSHPassword: connection.sshTunnel.hasPassword })
    if (Object.keys(errors).length) {
      showNotice({ tone: 'error', message: 'Review the highlighted draft settings before testing.' })
      return
    }
    setBusyAction('test')
    try {
      const savedProfile = JSON.stringify(connectionTestProfile(value)) === JSON.stringify(connectionTestProfile(connectionToDraft(connection)))
      const result = savedProfile ? await savedTestMutation.mutateAsync(connection.id) : await draftTestMutation.mutateAsync(value)
      showNotice({ tone: result.ok ? 'success' : 'error', message: result.ok ? `${savedProfile ? 'Saved profile' : 'Current draft'} passed in ${result.latencyMs} ms at ${value.host}:${value.port}.${savedProfile ? '' : ' The saved profile was not changed.'}` : result.message })
    } catch (error) {
      showNotice({ tone: 'error', message: errorMessage(error) })
    } finally {
      setBusyAction(null)
    }
  }

  async function testCreateDraft() {
    const errors = validateConnectionInput(createDraft)
    setCreateErrors(errors)
    if (Object.keys(errors).length) {
      setCreateTab(firstConnectionErrorSection(errors))
      setCreateTestNotice({ tone: 'error', message: 'Review the highlighted settings before testing this draft.' })
      return
    }
    setCreateTestNotice({ tone: 'info', message: `Testing ${createDraft.host}:${createDraft.port} with the current draft settings…` })
    try {
      const result = await draftTestMutation.mutateAsync(createDraft)
      setCreateTestNotice({ tone: result.ok ? 'success' : 'error', message: result.ok ? `Draft passed in ${result.latencyMs} ms. No profile has been saved yet.` : result.message })
    } catch (error) {
      setCreateTestNotice({ tone: 'error', message: errorMessage(error) })
    } finally {
      draftTestMutation.reset()
    }
  }

  function requestDisconnect(connection: Connection) {
    setConfirmation({ kind: 'disconnect', connection })
  }

  function requestDelete(connection: Connection) {
    setConfirmation({ kind: 'delete', connection })
  }

  async function confirmAction() {
    if (!confirmation) return
    const { kind, connection } = confirmation
    setConfirmation(null)
    setBusyAction(kind)
    try {
      if (kind === 'disconnect') {
        const next = await disconnectMutation.mutateAsync(connection.id)
        syncConnection(next)
        showNotice({ tone: 'info', message: `${connection.name} was disconnected.` })
      } else {
        await deleteMutation.mutateAsync(connection.id)
        const remaining = connections.filter((item) => item.id !== connection.id)
        removeConnection(connection)
        const next = remaining[0]
        onConnectionDeleted?.(connection.id, next)
        showNotice({ tone: 'success', message: `${connection.name} was deleted.` })
      }
    } catch (error) {
      showNotice({ tone: 'error', message: errorMessage(error) })
    } finally {
      setBusyAction(null)
    }
  }

  return (
    <WorkspacePage>
      <WorkspaceHeader icon={<Server />} eyebrow="Connection manager" title="Database connections" description="Create and tune connection profiles for this workspace." meta={<Badge variant={gateway.source === 'mock' ? 'accent' : 'success'}>{gateway.source === 'mock' ? 'Demo profiles' : 'Live API'}</Badge>} actions={<Button size="sm" onClick={openCreate}><Plus />New connection</Button>} />
      <WorkspaceToolbar>
        <span className="flex items-center gap-1.5 text-[length:var(--font-size-ui)] text-muted-foreground"><Database className="size-3.5" /><strong className="font-semibold text-foreground">{connections.length}</strong> profiles</span>
        <span className="mx-1 h-4 w-px bg-border" />
        <span className="flex items-center gap-1.5 text-[length:var(--font-size-ui)] text-muted-foreground"><span className="size-2 rounded-full bg-success-foreground" /><strong className="font-semibold text-foreground">{connectedCount}</strong> connected</span>
        <span className="mx-1 h-4 w-px bg-border" />
        <span className="flex items-center gap-1.5 text-[length:var(--font-size-ui)] text-muted-foreground"><Radio className="size-3.5" /><strong className="font-semibold text-foreground">{readonlyCount}</strong> readonly</span>
        <div aria-live="polite" aria-atomic="true" className="ml-auto min-w-0">{notice ? <div className={`flex max-w-md items-center gap-2 truncate rounded-md border px-2.5 py-1 text-[length:var(--font-size-meta)] font-medium ${notice.tone === 'success' ? 'border-success/45 bg-success/50 text-success-foreground' : notice.tone === 'error' ? 'border-destructive/35 bg-destructive/10 text-destructive' : 'border-info/40 bg-info/45 text-info-foreground'}`}>{notice.tone === 'success' ? <CheckCircle2 className="size-3.5 shrink-0" /> : notice.tone === 'error' ? <XCircle className="size-3.5 shrink-0" /> : <Radio className="size-3.5 shrink-0" />}<span className="truncate">{notice.message}</span></div> : null}</div>
      </WorkspaceToolbar>
      <MasterDetailLayout className="grid-cols-[minmax(18rem,21rem)_minmax(0,1fr)] max-lg:grid-cols-[minmax(15rem,18rem)_minmax(0,1fr)]" master={<ConnectionList connections={connections} selectedId={selected?.id} disabled={busyAction !== null || draftTestMutation.isPending} onSelect={selectConnection} onFavorite={toggleFavorite} />} detail={<ConnectionDetails key={selected?.id ?? 'empty'} connection={selected} busyAction={busyAction} onUpdate={updateConnection} onDuplicate={duplicateConnection} onFavorite={toggleFavorite} onConnect={connect} onDisconnect={requestDisconnect} onTest={testConnection} onDelete={requestDelete} onDirtyChange={onDirtyChange} />} />

      <Dialog open={createOpen} onOpenChange={(open) => { if (busyAction !== 'create' && !draftTestMutation.isPending) setCreateOpen(open) }}>
        <DialogContent className="max-w-3xl gap-4 p-0 sm:p-0">
          <DialogHeader className="border-b border-border px-5 pt-5 pb-4 sm:px-6 sm:pt-6"><DialogTitle>New database connection</DialogTitle><DialogDescription>Build a reusable profile now. Every setting remains editable before connecting.</DialogDescription></DialogHeader>
          <Tabs value={createTab} onValueChange={(value) => setCreateTab(value as ConnectionFormSection)} className="min-h-0 gap-0">
            <div className="overflow-x-auto px-5 sm:px-6"><TabsList className="min-w-max">{formSections.map((section) => <TabsTrigger key={section.value} value={section.value}>{section.label}</TabsTrigger>)}</TabsList></div>
            {formSections.map((section) => <TabsContent key={section.value} value={section.value} className="max-h-[min(58vh,560px)] overflow-y-auto px-5 py-4 sm:px-6"><ConnectionForm value={createDraft} onChange={(value) => { setCreateDraft(value); setCreateErrors({}); setCreateTestNotice(null) }} section={section.value} errors={createErrors} idPrefix="new-connection" disabled={busyAction === 'create' || draftTestMutation.isPending} /></TabsContent>)}
          </Tabs>
          <div aria-live="polite" aria-atomic="true" className="min-h-0 px-5 sm:px-6">{createTestNotice ? <div className={`flex items-start gap-2 rounded-lg border px-3 py-2 text-[length:var(--font-size-meta)] leading-5 ${createTestNotice.tone === 'success' ? 'border-success/45 bg-success/35 text-success-foreground' : createTestNotice.tone === 'error' ? 'border-destructive/35 bg-destructive/10 text-destructive' : 'border-info/40 bg-info/40 text-info-foreground'}`}>{createTestNotice.tone === 'success' ? <CheckCircle2 className="mt-0.5 size-3.5 shrink-0" /> : createTestNotice.tone === 'error' ? <XCircle className="mt-0.5 size-3.5 shrink-0" /> : <Activity className="mt-0.5 size-3.5 shrink-0 animate-pulse" />}<span>{createTestNotice.message}</span></div> : null}</div>
          <DialogFooter className="border-t border-border px-5 py-4 sm:px-6"><Button variant="ghost" onClick={testCreateDraft} disabled={busyAction === 'create' || draftTestMutation.isPending} className="sm:mr-auto"><Activity className={draftTestMutation.isPending ? 'animate-pulse' : ''} />{draftTestMutation.isPending ? 'Testing draft…' : 'Test draft'}</Button><Button variant="outline" onClick={() => setCreateOpen(false)} disabled={busyAction === 'create' || draftTestMutation.isPending}>Cancel</Button><Button onClick={createConnection} disabled={busyAction === 'create' || draftTestMutation.isPending}>{busyAction === 'create' ? 'Creating…' : 'Create connection'}</Button></DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(confirmation)} onOpenChange={(open) => { if (!open) setConfirmation(null) }}>
        <DialogContent className="max-w-md">
          <DialogHeader><div className="mb-2 grid size-10 place-items-center rounded-xl bg-warning text-warning-foreground"><AlertTriangle className="size-5" /></div><DialogTitle>{confirmation?.kind === 'delete' ? 'Delete connection profile?' : 'Disconnect active session?'}</DialogTitle><DialogDescription>{confirmation?.kind === 'delete' ? `${confirmation.connection.name} will be removed from this workspace${confirmation.connection.status === 'connected' ? ' and its active session will be disconnected' : ''}. Saved queries and history are not deleted.` : `Open tabs using ${confirmation?.connection.name ?? 'this connection'} may stop loading until it reconnects.`}</DialogDescription></DialogHeader>
          {confirmation ? <div className="rounded-lg border border-border bg-muted/45 p-3"><p className="text-[length:var(--font-size-ui)] font-semibold text-foreground">{confirmation.connection.name}</p><p className="mt-0.5 truncate font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{confirmation.connection.host}:{confirmation.connection.port}/{confirmation.connection.database}</p></div> : null}
          <DialogFooter><Button variant="outline" onClick={() => setConfirmation(null)}>Cancel</Button><Button variant={confirmation?.kind === 'delete' ? 'destructive' : 'default'} onClick={confirmAction}>{confirmation?.kind === 'delete' ? 'Delete profile' : 'Disconnect'}</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </WorkspacePage>
  )
}

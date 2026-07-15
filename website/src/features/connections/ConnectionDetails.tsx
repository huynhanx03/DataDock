import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { Activity, Cable, Check, Clock3, Copy, Database, Edit3, Gauge, KeyRound, MoreHorizontal, Network, Plug, Power, Server, ShieldCheck, SlidersHorizontal, Star, Trash2, UserRound, X } from 'lucide-react'
import type { Connection, ConnectionInput, ConnectionStatus, DatabaseEngine } from '@/entities/connection'
import { Badge, Button, DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger, IconButton, PropertyGrid, Tabs, TabsContent, TabsList, TabsTrigger, WorkspacePanel } from '@/shared/ui'
import { ConnectionForm, connectionToDraft, firstConnectionErrorSection, validateConnectionInput, type ConnectionFormErrors, type ConnectionFormSection } from './ConnectionForm'

export type ConnectionAction = 'create' | 'update' | 'duplicate' | 'favorite' | 'connect' | 'disconnect' | 'test' | 'delete' | null

type ConnectionDetailsProps = {
  connection?: Connection
  busyAction?: ConnectionAction
  onUpdate: (connection: Connection, value: ConnectionInput) => Promise<boolean | void> | boolean | void
  onDuplicate: (connection: Connection) => void
  onFavorite: (connection: Connection) => void
  onConnect: (connection: Connection) => void
  onDisconnect: (connection: Connection) => void
  onTest: (connection: Connection, value: ConnectionInput) => Promise<void> | void
  onDelete: (connection: Connection) => void
  onDirtyChange?: (dirty: boolean) => void
}

const engineLabels: Partial<Record<DatabaseEngine, string>> = {
  postgresql: 'PostgreSQL',
  mysql: 'MySQL',
  mariadb: 'MariaDB',
}

const statusLabels: Record<ConnectionStatus, string> = {
  connected: 'Connected',
  connecting: 'Connecting',
  disconnected: 'Disconnected',
  error: 'Connection error',
}

const statusVariants: Record<ConnectionStatus, 'success' | 'warning' | 'outline' | 'destructive'> = {
  connected: 'success',
  connecting: 'warning',
  disconnected: 'outline',
  error: 'destructive',
}

function dateTimeLabel(value?: string) {
  if (!value) return 'Never'
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}

function OverviewMetric({ icon: Icon, label, value, detail, tone = 'default' }: { icon: typeof Database; label: string; value: string; detail: string; tone?: 'default' | 'success' | 'warning' }) {
  return (
    <div className="rounded-xl border border-border bg-surface/65 p-3.5">
      <div className="flex items-start gap-3"><span className={`grid size-8 shrink-0 place-items-center rounded-lg ${tone === 'success' ? 'bg-success text-success-foreground' : tone === 'warning' ? 'bg-warning text-warning-foreground' : 'bg-accent text-accent-foreground'}`}><Icon className="size-4" /></span><div className="min-w-0"><p className="text-[length:var(--font-size-meta)] font-medium text-muted-foreground">{label}</p><p className="mt-0.5 truncate text-sm font-semibold tracking-[-0.015em] text-foreground">{value}</p><p className="mt-1 truncate text-[length:var(--font-size-meta)] text-muted-foreground">{detail}</p></div></div>
    </div>
  )
}

function DetailRow({ icon: Icon, label, value }: { icon: typeof Database; label: string; value: ReactNode }) {
  return <div className="flex min-w-0 items-center gap-3 border-b border-grid-line px-4 py-2.5 last:border-b-0"><Icon className="size-4 shrink-0 text-muted-foreground" /><span className="min-w-24 text-[length:var(--font-size-meta)] text-muted-foreground">{label}</span><span className="ml-auto min-w-0 truncate text-right text-[length:var(--font-size-ui)] font-medium text-foreground">{value}</span></div>
}

function ToggleCard({ checked, disabled, icon: Icon, title, description, onChange }: { checked: boolean; disabled?: boolean; icon: typeof ShieldCheck; title: string; description: string; onChange: (checked: boolean) => void }) {
  return <button type="button" role="switch" aria-checked={checked} disabled={disabled} onClick={() => onChange(!checked)} className="flex items-center gap-3 rounded-xl border border-border bg-surface/65 p-3.5 text-left transition-colors hover:border-border-strong hover:bg-accent/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/25 disabled:pointer-events-none disabled:opacity-55"><span className="grid size-8 shrink-0 place-items-center rounded-lg bg-accent text-accent-foreground"><Icon className="size-4" /></span><span className="min-w-0 flex-1"><span className="block text-[length:var(--font-size-ui)] font-semibold text-foreground">{title}</span><span className="mt-0.5 block text-[length:var(--font-size-meta)] leading-4 text-muted-foreground">{description}</span></span><span aria-hidden="true" className={`relative h-5 w-9 shrink-0 rounded-full border transition-colors ${checked ? 'border-primary bg-primary' : 'border-border-strong bg-muted'}`}><span className={`absolute top-0.5 size-3.5 rounded-full bg-white shadow-sm transition-transform ${checked ? 'translate-x-[18px]' : 'translate-x-0.5'}`} /></span></button>
}

function EmptyConnection() {
  return <div className="grid h-full min-h-80 place-items-center px-6 text-center"><div><span className="mx-auto grid size-12 place-items-center rounded-2xl border border-border bg-accent text-primary shadow-control"><Database className="size-5" /></span><h2 className="mt-4 text-base font-semibold">Select a connection</h2><p className="mt-1 max-w-sm text-[length:var(--font-size-ui)] leading-5 text-muted-foreground">Choose a profile to inspect its endpoint, security transport and pool configuration.</p></div></div>
}

export function ConnectionDetails({ connection, busyAction = null, onUpdate, onDuplicate, onFavorite, onConnect, onDisconnect, onTest, onDelete, onDirtyChange }: ConnectionDetailsProps) {
  const [activeTab, setActiveTab] = useState<'overview' | ConnectionFormSection>('overview')
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState<ConnectionInput | null>(() => connection ? connectionToDraft(connection) : null)
  const [formErrors, setFormErrors] = useState<ConnectionFormErrors>({})

  useEffect(() => {
    setDraft(connection ? connectionToDraft(connection) : null)
    setEditing(false)
    setActiveTab('overview')
    setFormErrors({})
  }, [connection?.id, connection?.updatedAt])

  const original = useMemo(() => connection ? connectionToDraft(connection) : null, [connection])
  const dirty = Boolean(draft && original && JSON.stringify(draft) !== JSON.stringify(original))
  const hasErrors = Object.keys(formErrors).length > 0

  useEffect(() => {
    onDirtyChange?.(dirty)
  }, [dirty, onDirtyChange])

  useEffect(() => () => onDirtyChange?.(false), [onDirtyChange])

  if (!connection || !draft || !original) return <EmptyConnection />

  const connected = connection.status === 'connected'
  const busy = busyAction !== null

  async function save() {
    const errors = validateConnectionInput(draft!, { hasStoredSSHPassword: connection!.sshTunnel.hasPassword })
    setFormErrors(errors)
    if (Object.keys(errors).length) {
      setEditing(true)
      setActiveTab(firstConnectionErrorSection(errors))
      return
    }
    const saved = await onUpdate(connection!, draft!)
    if (saved !== false) setEditing(false)
  }

  async function testDraft() {
    const errors = validateConnectionInput(draft!, { hasStoredSSHPassword: connection!.sshTunnel.hasPassword })
    setFormErrors(errors)
    if (Object.keys(errors).length) {
      setEditing(true)
      setActiveTab(firstConnectionErrorSection(errors))
      return
    }
    await onTest(connection!, draft!)
  }

  function revert() {
    setDraft(original!)
    setEditing(false)
    setFormErrors({})
  }

  function beginEdit() {
    setEditing(true)
    if (activeTab === 'overview') setActiveTab('general')
  }

  function changeQuickSetting(key: 'readOnly' | 'autoReconnect', checked: boolean) {
    setDraft((current) => current ? { ...current, [key]: checked } : current)
    setEditing(true)
    setFormErrors({})
  }

  function changeDraft(value: ConnectionInput) {
    setDraft(value)
    setFormErrors({})
  }

  return (
    <article className="flex h-full min-h-0 flex-col overflow-hidden">
      <header className="flex shrink-0 flex-wrap items-center gap-3 border-b border-border bg-background/85 px-4 py-3 lg:px-5">
        <span className="grid size-10 shrink-0 place-items-center rounded-xl border border-primary/25 bg-accent font-mono text-[11px] font-bold text-primary">{connection.engine === 'postgresql' ? 'PG' : connection.engine === 'mysql' ? 'MY' : connection.engine === 'mariadb' ? 'MA' : 'DB'}</span>
        <div className="min-w-0"><div className="flex min-w-0 flex-wrap items-center gap-2"><h2 className="truncate text-base font-semibold tracking-[-0.02em] text-foreground">{connection.name}</h2><Badge variant={statusVariants[connection.status]}><span className={`size-1.5 rounded-full ${connected ? 'bg-current' : connection.status === 'connecting' ? 'animate-pulse bg-current' : 'bg-current/60'}`} />{statusLabels[connection.status]}</Badge></div><p className="mt-0.5 truncate font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{connection.username}@{connection.host}:{connection.port}/{connection.database}</p></div>
        <div className="ml-auto flex items-center gap-1.5">
          <IconButton label={connection.favorite ? 'Remove from favorites' : 'Add to favorites'} variant="ghost" onClick={() => onFavorite(connection)} disabled={busy} className={connection.favorite ? 'text-warning-foreground' : ''}><Star className={connection.favorite ? 'fill-current' : ''} /></IconButton>
          <Button size="sm" variant="outline" onClick={testDraft} disabled={busy}><Activity className={busyAction === 'test' ? 'animate-pulse' : ''} />{busyAction === 'test' ? 'Testing draft…' : 'Test draft'}</Button>
          <Button size="sm" variant={connected ? 'outline' : 'default'} onClick={() => connected ? onDisconnect(connection) : onConnect(connection)} disabled={busy}>{connected ? <Power /> : <Plug />}{busyAction === 'connect' ? 'Connecting…' : busyAction === 'disconnect' ? 'Disconnecting…' : connected ? 'Disconnect' : 'Connect'}</Button>
          <DropdownMenu><DropdownMenuTrigger asChild><IconButton label="More connection actions" variant="outline" disabled={busy}><MoreHorizontal /></IconButton></DropdownMenuTrigger><DropdownMenuContent align="end"><DropdownMenuItem onSelect={beginEdit}><Edit3 />Edit settings</DropdownMenuItem><DropdownMenuItem onSelect={() => onDuplicate(connection)}><Copy />Duplicate profile</DropdownMenuItem><DropdownMenuSeparator /><DropdownMenuItem tone="destructive" onSelect={() => onDelete(connection)}><Trash2 />Delete connection</DropdownMenuItem></DropdownMenuContent></DropdownMenu>
        </div>
      </header>

      <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as 'overview' | ConnectionFormSection)} className="min-h-0 flex-1 gap-0 overflow-hidden">
        <div className="shrink-0 overflow-x-auto border-b border-border bg-surface/45 px-4 py-2 lg:px-5"><TabsList className="h-[var(--control-height-sm)] min-w-max"><TabsTrigger value="overview">Overview</TabsTrigger><TabsTrigger value="general">General</TabsTrigger><TabsTrigger value="ssl">SSL</TabsTrigger><TabsTrigger value="ssh">SSH tunnel</TabsTrigger><TabsTrigger value="proxy">Proxy</TabsTrigger><TabsTrigger value="pool">Pool</TabsTrigger></TabsList></div>

        <TabsContent value="overview" className="min-h-0 overflow-y-auto p-4 lg:p-5">
          <div className="mx-auto grid max-w-5xl gap-4">
            <PropertyGrid className="xl:grid-cols-4">
              <OverviewMetric icon={Server} label="Engine" value={engineLabels[connection.engine] ?? connection.engine} detail={`${connection.database} database`} />
              <OverviewMetric icon={Gauge} label="Round trip" value={connection.latencyMs !== undefined ? `${connection.latencyMs} ms` : 'Not measured'} detail={connected ? 'Healthy connection' : 'Last measured latency'} tone={connected ? 'success' : 'default'} />
              <OverviewMetric icon={ShieldCheck} label="Transport" value={connection.sslMode === 'disable' ? 'Unencrypted' : connection.sslMode} detail={connection.sshTunnel.enabled ? 'Via SSH tunnel' : connection.proxyUrl ? 'Via network proxy' : 'Direct transport'} tone={connection.sslMode === 'disable' ? 'warning' : 'success'} />
              <OverviewMetric icon={SlidersHorizontal} label="Pool" value={`${connection.maxOpenConns} open`} detail={`${connection.maxIdleConns} idle · ${connection.connMaxLifetimeSeconds}s lifetime`} />
            </PropertyGrid>
            <div className="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(280px,0.65fr)]">
              <WorkspacePanel title="Connection profile" description="Endpoint and session identity" noPadding>
                <DetailRow icon={Database} label="Database" value={<span className="font-mono">{connection.database}</span>} />
                <DetailRow icon={Server} label="Host" value={<span className="font-mono">{connection.host}:{connection.port}</span>} />
                <DetailRow icon={UserRound} label="Username" value={<span className="font-mono">{connection.username}</span>} />
                <DetailRow icon={Clock3} label="Last connected" value={dateTimeLabel(connection.lastConnectedAt)} />
                <DetailRow icon={Cable} label="Reconnect" value={connection.autoReconnect ? <Badge variant="success"><Check />Enabled</Badge> : <Badge variant="outline"><X />Disabled</Badge>} />
              </WorkspacePanel>
              <WorkspacePanel title="Access controls" description="Changes are staged until saved">
                <div className="grid gap-2"><ToggleCard checked={draft.readOnly} disabled={busy} icon={ShieldCheck} title="Readonly mode" description="Protect this profile from write queries." onChange={(checked) => changeQuickSetting('readOnly', checked)} /><ToggleCard checked={draft.autoReconnect} disabled={busy} icon={Cable} title="Auto reconnect" description="Restore interrupted sessions." onChange={(checked) => changeQuickSetting('autoReconnect', checked)} /></div>
              </WorkspacePanel>
            </div>
            <WorkspacePanel title="Transport path" description="How DataDock reaches the database">
              <div className="grid gap-2 sm:grid-cols-3"><div className="rounded-lg border border-border bg-muted/30 p-3"><Database className="size-4 text-primary" /><p className="mt-2 text-[length:var(--font-size-ui)] font-semibold">Database</p><p className="mt-0.5 truncate font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{connection.host}:{connection.port}</p></div><div className={`rounded-lg border p-3 ${connection.sshTunnel.enabled || connection.proxyUrl ? 'border-primary/30 bg-accent/40' : 'border-border bg-muted/30'}`}>{connection.sshTunnel.enabled ? <KeyRound className="size-4 text-primary" /> : <Network className="size-4 text-muted-foreground" />}<p className="mt-2 text-[length:var(--font-size-ui)] font-semibold">{connection.sshTunnel.enabled ? 'SSH tunnel' : connection.proxyUrl ? 'Network proxy' : 'Direct route'}</p><p className="mt-0.5 truncate font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{connection.sshTunnel.enabled ? `${connection.sshTunnel.host}:${connection.sshTunnel.port}` : connection.proxyUrl || 'No intermediary'}</p></div><div className={`rounded-lg border p-3 ${connection.sslMode !== 'disable' ? 'border-success/40 bg-success/30' : 'border-warning/40 bg-warning/30'}`}><ShieldCheck className={`size-4 ${connection.sslMode !== 'disable' ? 'text-success-foreground' : 'text-warning-foreground'}`} /><p className="mt-2 text-[length:var(--font-size-ui)] font-semibold">{connection.sslMode === 'disable' ? 'SSL disabled' : 'Encrypted transport'}</p><p className="mt-0.5 font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{connection.sslMode}</p></div></div>
            </WorkspacePanel>
          </div>
        </TabsContent>

        {(['general', 'ssl', 'ssh', 'proxy', 'pool'] as const).map((section) => <TabsContent key={section} value={section} className="min-h-0 overflow-y-auto p-4 lg:p-5"><div className="mx-auto max-w-3xl"><div className="mb-4 flex items-center justify-between gap-3"><div><h3 className="text-sm font-semibold text-foreground">{section === 'general' ? 'General settings' : section === 'ssl' ? 'SSL transport' : section === 'ssh' ? 'SSH tunnel' : section === 'proxy' ? 'Proxy transport' : 'Connection pool'}</h3><p className="mt-0.5 text-[length:var(--font-size-meta)] text-muted-foreground">{editing ? 'Editing this profile. Review changes before saving.' : 'Settings are locked until edit mode is enabled.'}</p></div>{!editing ? <Button size="sm" variant="outline" onClick={() => setEditing(true)}><Edit3 />Edit</Button> : <Button size="sm" variant="ghost" onClick={revert} disabled={busy}>{dirty ? 'Discard changes' : 'Done'}</Button>}</div><ConnectionForm value={draft} onChange={changeDraft} section={section} disabled={!editing || busy} errors={formErrors} idPrefix={`detail-${connection.id}`} /></div></TabsContent>)}
      </Tabs>

      {dirty || hasErrors ? <div role={hasErrors ? 'alert' : undefined} className={`flex shrink-0 flex-wrap items-center gap-3 border-t px-4 py-2.5 lg:px-5 ${hasErrors ? 'border-destructive/30 bg-destructive/10' : 'border-primary/25 bg-accent/55'}`}><span className={`size-2 rounded-full ${hasErrors ? 'bg-destructive' : 'bg-primary'}`} /><div className="min-w-0 flex-1"><p className="text-[length:var(--font-size-ui)] font-semibold text-foreground">{hasErrors ? 'Review the highlighted settings' : 'Unsaved connection changes'}</p><p className="text-[length:var(--font-size-meta)] text-muted-foreground">{hasErrors ? 'DataDock moved you to the first setting that needs attention.' : 'Test the current draft after changing endpoint or transport settings.'}</p></div><Button size="sm" variant="ghost" onClick={revert} disabled={busy}>Revert</Button><Button size="sm" variant="outline" onClick={testDraft} disabled={busy}><Activity />Test draft</Button><Button size="sm" onClick={save} disabled={busyAction === 'update'}>{busyAction === 'update' ? 'Saving…' : 'Save changes'}</Button></div> : null}
    </article>
  )
}

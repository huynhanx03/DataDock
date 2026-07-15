import { useId, type ReactNode } from 'react'
import { Cable, Database, KeyRound, Network, ShieldCheck, SlidersHorizontal } from 'lucide-react'
import type { AvailableDatabaseEngine, Connection, ConnectionInput, SSLMode } from '@/entities/connection'
import { APP_CONFIG } from '@/shared/config/constants'
import { Input, Select } from '@/shared/ui'

export type ConnectionFormSection = 'general' | 'ssl' | 'ssh' | 'proxy' | 'pool'

export type ConnectionFormField =
  | 'name'
  | 'host'
  | 'port'
  | 'database'
  | 'username'
  | 'sslCaPath'
  | 'sslCertPath'
  | 'sslKeyPath'
  | 'sshHost'
  | 'sshPort'
  | 'sshUsername'
  | 'sshAuthentication'
  | 'proxyUrl'
  | 'maxOpenConns'
  | 'maxIdleConns'
  | 'connMaxLifetimeSeconds'

export type ConnectionFormErrors = Partial<Record<ConnectionFormField, string>>

export type ConnectionValidationOptions = {
  hasStoredSSHPassword?: boolean
}

type ConnectionFormProps = {
  value: ConnectionInput
  onChange: (value: ConnectionInput) => void
  section: ConnectionFormSection
  disabled?: boolean
  errors?: ConnectionFormErrors
  idPrefix?: string
}

const engines: Array<{ value: AvailableDatabaseEngine; label: string; short: string }> = [
  { value: 'postgresql', label: 'PostgreSQL', short: 'PG' },
  { value: 'mysql', label: 'MySQL', short: 'MY' },
  { value: 'mariadb', label: 'MariaDB', short: 'MA' },
]

const sslModes: Array<{ value: SSLMode; label: string; description: string }> = [
  { value: 'disable', label: 'Disabled', description: 'Use an unencrypted transport.' },
  { value: 'require', label: 'Require', description: 'Encrypt traffic without validating the server certificate.' },
  { value: 'verify-ca', label: 'Verify CA', description: 'Validate the certificate authority.' },
  { value: 'verify-full', label: 'Verify full', description: 'Validate the CA and server hostname.' },
]

const sectionFields: Record<ConnectionFormSection, ConnectionFormField[]> = {
  general: ['name', 'host', 'port', 'database', 'username'],
  ssl: ['sslCaPath', 'sslCertPath', 'sslKeyPath'],
  ssh: ['sshHost', 'sshPort', 'sshUsername', 'sshAuthentication'],
  proxy: ['proxyUrl'],
  pool: ['maxOpenConns', 'maxIdleConns', 'connMaxLifetimeSeconds'],
}

function validPort(value: number) {
  return Number.isInteger(value) && value >= 1 && value <= 65_535
}

function validNonNegativeInteger(value: number) {
  return Number.isInteger(value) && value >= 0
}

export function validateConnectionInput(value: ConnectionInput, options: ConnectionValidationOptions = {}): ConnectionFormErrors {
  const errors: ConnectionFormErrors = {}

  if (!value.name.trim()) errors.name = 'Enter a profile name.'
  if (!value.host.trim()) errors.host = 'Enter the database hostname or IP address.'
  if (!validPort(value.port)) errors.port = 'Use a whole port number from 1 to 65535.'
  if (!value.database.trim()) errors.database = 'Enter the database to open after connecting.'
  if (!value.username.trim()) errors.username = 'Enter the database username.'

  if ((value.sslMode === 'verify-ca' || value.sslMode === 'verify-full') && !value.sslCaPath.trim()) {
    errors.sslCaPath = `${value.sslMode === 'verify-full' ? 'Verify full' : 'Verify CA'} requires a CA certificate path.`
  }
  if (Boolean(value.sslCertPath.trim()) !== Boolean(value.sslKeyPath.trim())) {
    if (!value.sslCertPath.trim()) errors.sslCertPath = 'Add the client certificate that matches this private key.'
    if (!value.sslKeyPath.trim()) errors.sslKeyPath = 'Add the private key that matches this client certificate.'
  }

  if (value.sshTunnel.enabled) {
    if (!value.sshTunnel.host.trim()) errors.sshHost = 'Enter the SSH bastion hostname or IP address.'
    if (!validPort(value.sshTunnel.port)) errors.sshPort = 'Use a whole SSH port number from 1 to 65535.'
    if (!value.sshTunnel.username.trim()) errors.sshUsername = 'Enter the SSH username.'
    if (!value.sshTunnel.password.trim() && !value.sshTunnel.privateKeyPath.trim() && !options.hasStoredSSHPassword) {
      errors.sshAuthentication = 'Enter an SSH password or choose a private key path.'
    }
  }

  if (value.proxyUrl.trim()) {
    try {
      const proxy = new URL(value.proxyUrl)
      if (!['http:', 'https:', 'socks5:'].includes(proxy.protocol) || !proxy.hostname) throw new Error()
    } catch {
      errors.proxyUrl = 'Use a complete HTTP, HTTPS or SOCKS5 URL, for example socks5://proxy.local:1080.'
    }
  }

  if (!validNonNegativeInteger(value.maxOpenConns)) errors.maxOpenConns = 'Use a whole number greater than or equal to 0.'
  if (!validNonNegativeInteger(value.maxIdleConns)) errors.maxIdleConns = 'Use a whole number greater than or equal to 0.'
  if (validNonNegativeInteger(value.maxIdleConns) && validNonNegativeInteger(value.maxOpenConns) && value.maxIdleConns > value.maxOpenConns) {
    errors.maxIdleConns = 'Idle connections cannot exceed the maximum open connections.'
  }
  if (!validNonNegativeInteger(value.connMaxLifetimeSeconds)) errors.connMaxLifetimeSeconds = 'Use a whole number of seconds greater than or equal to 0.'

  return errors
}

export function firstConnectionErrorSection(errors: ConnectionFormErrors): ConnectionFormSection {
  return (Object.keys(sectionFields) as ConnectionFormSection[]).find((section) => sectionFields[section].some((field) => errors[field])) ?? 'general'
}

export function createConnectionDraft(workspaceId: string): ConnectionInput {
  return {
    workspaceId,
    name: 'Local PostgreSQL',
    engine: 'postgresql',
    host: APP_CONFIG.connection.defaultHost,
    port: APP_CONFIG.databasePorts.postgresql,
    database: 'postgres',
    username: 'postgres',
    password: '',
    sslMode: 'disable',
    sslCaPath: '',
    sslCertPath: '',
    sslKeyPath: '',
    proxyUrl: '',
    sshTunnel: {
      enabled: false,
      host: '',
      port: APP_CONFIG.connection.sshDefaultPort,
      username: '',
      password: '',
      privateKeyPath: '',
      knownHostsPath: '',
    },
    readOnly: false,
    autoReconnect: true,
    maxOpenConns: APP_CONFIG.connection.maxOpenConnections,
    maxIdleConns: APP_CONFIG.connection.maxIdleConnections,
    connMaxLifetimeSeconds: APP_CONFIG.connection.maxLifetimeSeconds,
  }
}

export function connectionToDraft(connection: Connection): ConnectionInput {
  return {
    workspaceId: connection.workspaceId,
    name: connection.name,
    engine: connection.engine === 'postgresql' || connection.engine === 'mysql' || connection.engine === 'mariadb' ? connection.engine : 'postgresql',
    host: connection.host,
    port: connection.port,
    database: connection.database,
    username: connection.username,
    password: '',
    sslMode: connection.sslMode,
    sslCaPath: connection.sslCaPath,
    sslCertPath: connection.sslCertPath,
    sslKeyPath: connection.sslKeyPath,
    proxyUrl: connection.proxyUrl,
    sshTunnel: {
      enabled: connection.sshTunnel.enabled,
      host: connection.sshTunnel.host,
      port: connection.sshTunnel.port,
      username: connection.sshTunnel.username,
      password: '',
      privateKeyPath: connection.sshTunnel.privateKeyPath,
      knownHostsPath: connection.sshTunnel.knownHostsPath,
    },
    readOnly: connection.readOnly,
    autoReconnect: connection.autoReconnect,
    maxOpenConns: connection.maxOpenConns,
    maxIdleConns: connection.maxIdleConns,
    connMaxLifetimeSeconds: connection.connMaxLifetimeSeconds,
  }
}

function Field({
  id,
  label,
  hint,
  error,
  children,
  className = '',
}: {
  id: string
  label: string
  hint?: string
  error?: string
  children: ReactNode
  className?: string
}) {
  return (
    <label htmlFor={id} className={`grid min-w-0 gap-1.5 text-[length:var(--font-size-ui)] font-medium text-foreground ${className}`}>
      <span>{label}</span>
      {children}
      {error ? <span id={`${id}-error`} role="alert" className="text-[length:var(--font-size-meta)] font-normal leading-4 text-destructive">{error}</span> : hint ? <span className="text-[length:var(--font-size-meta)] font-normal leading-4 text-muted-foreground">{hint}</span> : null}
    </label>
  )
}

function SwitchRow({ checked, disabled, icon: Icon, label, description, onChange }: { checked: boolean; disabled?: boolean; icon: typeof ShieldCheck; label: string; description: string; onChange: (checked: boolean) => void }) {
  return (
    <button type="button" role="switch" aria-checked={checked} disabled={disabled} onClick={() => onChange(!checked)} className="flex w-full items-center gap-3 rounded-lg border border-border bg-surface/70 p-3 text-left transition-colors hover:border-border-strong hover:bg-accent/35 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/25 disabled:pointer-events-none disabled:opacity-55">
      <span className="grid size-8 shrink-0 place-items-center rounded-lg bg-accent text-accent-foreground"><Icon className="size-4" /></span>
      <span className="min-w-0 flex-1"><span className="block text-[length:var(--font-size-ui)] font-semibold text-foreground">{label}</span><span className="mt-0.5 block text-[length:var(--font-size-meta)] leading-4 text-muted-foreground">{description}</span></span>
      <span aria-hidden="true" className={`relative h-5 w-9 shrink-0 rounded-full border transition-colors ${checked ? 'border-primary bg-primary' : 'border-border-strong bg-muted'}`}><span className={`absolute top-0.5 size-3.5 rounded-full bg-white shadow-sm transition-transform ${checked ? 'translate-x-[18px]' : 'translate-x-0.5'}`} /></span>
    </button>
  )
}

function SectionIntro({ icon: Icon, title, description }: { icon: typeof Database; title: string; description: string }) {
  return <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/35 p-3"><span className="grid size-8 shrink-0 place-items-center rounded-lg bg-accent text-accent-foreground"><Icon className="size-4" /></span><div><h3 className="text-[length:var(--font-size-ui)] font-semibold text-foreground">{title}</h3><p className="mt-0.5 text-[length:var(--font-size-meta)] leading-4 text-muted-foreground">{description}</p></div></div>
}

export function ConnectionForm({ value, onChange, section, disabled = false, errors = {}, idPrefix }: ConnectionFormProps) {
  const generatedId = useId()
  const prefix = idPrefix ?? generatedId.replaceAll(':', '')
  const setValue = <K extends keyof ConnectionInput>(key: K, next: ConnectionInput[K]) => onChange({ ...value, [key]: next })
  const setSSH = <K extends keyof ConnectionInput['sshTunnel']>(key: K, next: ConnectionInput['sshTunnel'][K]) => onChange({ ...value, sshTunnel: { ...value.sshTunnel, [key]: next } })

  if (section === 'general') return (
    <div className="grid gap-4">
      <SectionIntro icon={Database} title="Database endpoint" description="Basic profile and credentials used when DataDock opens a session." />
      <fieldset disabled={disabled} className="grid gap-4 transition-opacity disabled:opacity-60">
        <legend className="sr-only">Database engine</legend>
        <div className="grid grid-cols-3 gap-2">
          {engines.map((engine) => <button key={engine.value} type="button" aria-pressed={value.engine === engine.value} onClick={() => onChange({ ...value, engine: engine.value, port: APP_CONFIG.databasePorts[engine.value] })} className={`flex min-w-0 items-center gap-2 rounded-lg border p-2.5 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/25 ${value.engine === engine.value ? 'border-primary/65 bg-accent text-accent-foreground' : 'border-border bg-surface hover:border-border-strong hover:bg-accent/35'}`}><span className={`grid size-8 shrink-0 place-items-center rounded-md font-mono text-[11px] font-bold ${value.engine === engine.value ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'}`}>{engine.short}</span><span className="truncate text-[length:var(--font-size-ui)] font-semibold">{engine.label}</span></button>)}
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field id={`${prefix}-name`} label="Connection name" error={errors.name} className="sm:col-span-2"><Input id={`${prefix}-name`} value={value.name} invalid={Boolean(errors.name)} aria-describedby={errors.name ? `${prefix}-name-error` : undefined} onChange={(event) => setValue('name', event.target.value)} placeholder="Production PostgreSQL" /></Field>
          <Field id={`${prefix}-host`} label="Host" error={errors.host}><Input id={`${prefix}-host`} value={value.host} invalid={Boolean(errors.host)} aria-describedby={errors.host ? `${prefix}-host-error` : undefined} onChange={(event) => setValue('host', event.target.value)} placeholder="localhost" /></Field>
          <Field id={`${prefix}-port`} label="Port" error={errors.port}><Input id={`${prefix}-port`} type="number" min={1} max={65535} value={value.port} invalid={Boolean(errors.port)} aria-describedby={errors.port ? `${prefix}-port-error` : undefined} onChange={(event) => setValue('port', Number(event.target.value))} /></Field>
          <Field id={`${prefix}-database`} label="Database" error={errors.database}><Input id={`${prefix}-database`} value={value.database} invalid={Boolean(errors.database)} aria-describedby={errors.database ? `${prefix}-database-error` : undefined} onChange={(event) => setValue('database', event.target.value)} placeholder="postgres" /></Field>
          <Field id={`${prefix}-username`} label="Username" error={errors.username}><Input id={`${prefix}-username`} value={value.username} invalid={Boolean(errors.username)} aria-describedby={errors.username ? `${prefix}-username-error` : undefined} onChange={(event) => setValue('username', event.target.value)} placeholder="postgres" autoComplete="username" /></Field>
          <Field id={`${prefix}-password`} label="Password" hint="Leave blank to keep the saved secret."><Input id={`${prefix}-password`} type="password" value={value.password} onChange={(event) => setValue('password', event.target.value)} placeholder="••••••••••••" autoComplete="new-password" /></Field>
        </div>
        <div className="grid gap-2 sm:grid-cols-2">
          <SwitchRow checked={value.readOnly} disabled={disabled} icon={ShieldCheck} label="Readonly mode" description="Block write statements in this profile." onChange={(checked) => setValue('readOnly', checked)} />
          <SwitchRow checked={value.autoReconnect} disabled={disabled} icon={Cable} label="Auto reconnect" description="Restore interrupted sessions automatically." onChange={(checked) => setValue('autoReconnect', checked)} />
        </div>
      </fieldset>
    </div>
  )

  if (section === 'ssl') return (
    <div className="grid gap-4">
      <SectionIntro icon={ShieldCheck} title="SSL transport" description="Choose certificate verification and optional client credentials." />
      <fieldset disabled={disabled} className="grid gap-4 transition-opacity disabled:opacity-60">
        <Field id={`${prefix}-ssl-mode`} label="SSL mode" hint={sslModes.find((mode) => mode.value === value.sslMode)?.description}>
          <Select id={`${prefix}-ssl-mode`} value={value.sslMode} onChange={(event) => setValue('sslMode', event.target.value as SSLMode)}>{sslModes.map((mode) => <option key={mode.value} value={mode.value}>{mode.label}</option>)}</Select>
        </Field>
        {value.sslMode !== 'disable' ? <div className="grid gap-4 sm:grid-cols-2">
          <Field id={`${prefix}-ssl-ca`} label="CA certificate" error={errors.sslCaPath} className="sm:col-span-2"><Input id={`${prefix}-ssl-ca`} value={value.sslCaPath} invalid={Boolean(errors.sslCaPath)} aria-describedby={errors.sslCaPath ? `${prefix}-ssl-ca-error` : undefined} onChange={(event) => setValue('sslCaPath', event.target.value)} placeholder="/certs/ca.pem" /></Field>
          <Field id={`${prefix}-ssl-cert`} label="Client certificate" error={errors.sslCertPath}><Input id={`${prefix}-ssl-cert`} value={value.sslCertPath} invalid={Boolean(errors.sslCertPath)} aria-describedby={errors.sslCertPath ? `${prefix}-ssl-cert-error` : undefined} onChange={(event) => setValue('sslCertPath', event.target.value)} placeholder="/certs/client.pem" /></Field>
          <Field id={`${prefix}-ssl-key`} label="Client key" error={errors.sslKeyPath}><Input id={`${prefix}-ssl-key`} value={value.sslKeyPath} invalid={Boolean(errors.sslKeyPath)} aria-describedby={errors.sslKeyPath ? `${prefix}-ssl-key-error` : undefined} onChange={(event) => setValue('sslKeyPath', event.target.value)} placeholder="/certs/client-key.pem" /></Field>
        </div> : <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center"><ShieldCheck className="mx-auto size-5 text-muted-foreground" /><p className="mt-2 text-[length:var(--font-size-ui)] font-medium">SSL is disabled</p><p className="mt-1 text-[length:var(--font-size-meta)] text-muted-foreground">Select a verification mode to configure certificates.</p></div>}
      </fieldset>
    </div>
  )

  if (section === 'ssh') return (
    <div className="grid gap-4">
      <SectionIntro icon={KeyRound} title="SSH tunnel" description="Reach private databases through a bastion host without changing the database endpoint." />
      <fieldset disabled={disabled} className="grid gap-4 transition-opacity disabled:opacity-60">
        <SwitchRow checked={value.sshTunnel.enabled} disabled={disabled} icon={KeyRound} label="Use SSH tunnel" description="Route this connection through the configured SSH host." onChange={(checked) => setSSH('enabled', checked)} />
        {value.sshTunnel.enabled ? <div className="grid gap-4 sm:grid-cols-2">
          <Field id={`${prefix}-ssh-host`} label="SSH host" error={errors.sshHost}><Input id={`${prefix}-ssh-host`} value={value.sshTunnel.host} invalid={Boolean(errors.sshHost)} aria-describedby={errors.sshHost ? `${prefix}-ssh-host-error` : undefined} onChange={(event) => setSSH('host', event.target.value)} placeholder="bastion.example.com" /></Field>
          <Field id={`${prefix}-ssh-port`} label="SSH port" error={errors.sshPort}><Input id={`${prefix}-ssh-port`} type="number" min={1} max={65535} value={value.sshTunnel.port} invalid={Boolean(errors.sshPort)} aria-describedby={errors.sshPort ? `${prefix}-ssh-port-error` : undefined} onChange={(event) => setSSH('port', Number(event.target.value))} /></Field>
          <Field id={`${prefix}-ssh-user`} label="SSH username" error={errors.sshUsername}><Input id={`${prefix}-ssh-user`} value={value.sshTunnel.username} invalid={Boolean(errors.sshUsername)} aria-describedby={errors.sshUsername ? `${prefix}-ssh-user-error` : undefined} onChange={(event) => setSSH('username', event.target.value)} placeholder="deploy" autoComplete="username" /></Field>
          <Field id={`${prefix}-ssh-password`} label="SSH password" hint="Optional when a private key is configured."><Input id={`${prefix}-ssh-password`} type="password" value={value.sshTunnel.password} onChange={(event) => setSSH('password', event.target.value)} placeholder="••••••••••••" autoComplete="new-password" /></Field>
          <Field id={`${prefix}-ssh-key`} label="Private key path" error={errors.sshAuthentication} className="sm:col-span-2"><Input id={`${prefix}-ssh-key`} value={value.sshTunnel.privateKeyPath} invalid={Boolean(errors.sshAuthentication)} aria-describedby={errors.sshAuthentication ? `${prefix}-ssh-key-error` : undefined} onChange={(event) => setSSH('privateKeyPath', event.target.value)} placeholder="~/.ssh/id_ed25519" /></Field>
          <Field id={`${prefix}-known-hosts`} label="Known hosts path" className="sm:col-span-2"><Input id={`${prefix}-known-hosts`} value={value.sshTunnel.knownHostsPath} onChange={(event) => setSSH('knownHostsPath', event.target.value)} placeholder="~/.ssh/known_hosts" /></Field>
        </div> : <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center"><KeyRound className="mx-auto size-5 text-muted-foreground" /><p className="mt-2 text-[length:var(--font-size-ui)] font-medium">Direct database transport</p><p className="mt-1 text-[length:var(--font-size-meta)] text-muted-foreground">Enable the tunnel when the host is only reachable through SSH.</p></div>}
      </fieldset>
    </div>
  )

  if (section === 'proxy') return (
    <div className="grid gap-4">
      <SectionIntro icon={Network} title="Network proxy" description="Route outbound database traffic through an HTTP, HTTPS or SOCKS5 proxy." />
      <fieldset disabled={disabled} className="grid gap-4 transition-opacity disabled:opacity-60">
        <Field id={`${prefix}-proxy-url`} label="Proxy URL" error={errors.proxyUrl} hint="Leave blank to connect directly. Credentials may be included in the URL."><Input id={`${prefix}-proxy-url`} value={value.proxyUrl} invalid={Boolean(errors.proxyUrl)} aria-describedby={errors.proxyUrl ? `${prefix}-proxy-url-error` : undefined} onChange={(event) => setValue('proxyUrl', event.target.value)} placeholder="socks5://proxy.local:1080" /></Field>
        <div className="grid grid-cols-3 gap-2">
          {['HTTP', 'HTTPS', 'SOCKS5'].map((protocol) => <div key={protocol} className="rounded-lg border border-border bg-surface/70 px-3 py-2 text-center font-mono text-[length:var(--font-size-meta)] font-semibold text-muted-foreground">{protocol}</div>)}
        </div>
        <div className="rounded-lg border border-info/35 bg-info/40 p-3 text-[length:var(--font-size-meta)] leading-5 text-info-foreground">Proxy settings are isolated per connection profile and never change the browser network configuration.</div>
      </fieldset>
    </div>
  )

  return (
    <div className="grid gap-4">
      <SectionIntro icon={SlidersHorizontal} title="Connection pool" description="Keep resource usage predictable for browsing, editing and concurrent query tabs." />
      <fieldset disabled={disabled} className="grid gap-4 transition-opacity disabled:opacity-60 sm:grid-cols-2">
        <Field id={`${prefix}-pool-open`} label="Maximum open connections" error={errors.maxOpenConns} hint="Total active and idle connections. Use 0 only with 0 idle connections."><Input id={`${prefix}-pool-open`} type="number" min={0} step={1} value={value.maxOpenConns} invalid={Boolean(errors.maxOpenConns)} aria-describedby={errors.maxOpenConns ? `${prefix}-pool-open-error` : undefined} onChange={(event) => setValue('maxOpenConns', Number(event.target.value))} /></Field>
        <Field id={`${prefix}-pool-idle`} label="Maximum idle connections" error={errors.maxIdleConns} hint="Warm connections retained for reuse."><Input id={`${prefix}-pool-idle`} type="number" min={0} step={1} value={value.maxIdleConns} invalid={Boolean(errors.maxIdleConns)} aria-describedby={errors.maxIdleConns ? `${prefix}-pool-idle-error` : undefined} onChange={(event) => setValue('maxIdleConns', Number(event.target.value))} /></Field>
        <Field id={`${prefix}-pool-lifetime`} label="Maximum lifetime" error={errors.connMaxLifetimeSeconds} hint="Seconds before a connection is recycled. Use 0 for no limit." className="sm:col-span-2"><div className="relative"><Input id={`${prefix}-pool-lifetime`} type="number" min={0} step={1} value={value.connMaxLifetimeSeconds} invalid={Boolean(errors.connMaxLifetimeSeconds)} aria-describedby={errors.connMaxLifetimeSeconds ? `${prefix}-pool-lifetime-error` : undefined} onChange={(event) => setValue('connMaxLifetimeSeconds', Number(event.target.value))} className="pr-16" /><span className="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 text-[length:var(--font-size-meta)] text-muted-foreground">seconds</span></div></Field>
        <div className="sm:col-span-2 grid gap-2 rounded-lg border border-border bg-muted/35 p-3 text-[length:var(--font-size-meta)] text-muted-foreground sm:grid-cols-3"><span><strong className="block text-[length:var(--font-size-ui)] text-foreground">{value.maxOpenConns}</strong>open limit</span><span><strong className="block text-[length:var(--font-size-ui)] text-foreground">{value.maxIdleConns}</strong>idle limit</span><span><strong className="block text-[length:var(--font-size-ui)] text-foreground">{Math.max(0, value.maxOpenConns - value.maxIdleConns)}</strong>burst capacity</span></div>
      </fieldset>
    </div>
  )
}

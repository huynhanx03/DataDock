import { Cloud, GitBranch, Keyboard, Wifi } from 'lucide-react'
import { APP_CONFIG } from '@/shared/config/constants'
import { cn } from '@/shared/lib/cn'

type StatusBarProps = {
  connectionName?: string
  connected?: boolean
  latencyMs?: number
  message?: string
}

export function StatusBar({ connectionName, connected, latencyMs, message }: StatusBarProps) {
  return (
    <footer className="flex h-7 shrink-0 items-center gap-4 border-t border-border bg-surface/85 px-3 font-mono text-[length:var(--font-size-meta)] text-muted-foreground" aria-label="Application status" aria-live="polite">
      <span className="flex min-w-0 items-center gap-1.5">
        <span className={cn('size-1.5 shrink-0 rounded-full bg-muted-foreground', connected && 'bg-emerald-400 shadow-[0_0_8px_color-mix(in_oklab,var(--primitive-emerald-400)_55%,transparent)]')} />
        <span className="truncate">{message || (connectionName ? connectionName : 'Ready')}</span>
      </span>
      {latencyMs !== undefined ? (
        <span className="hidden items-center gap-1.5 sm:flex">
          <Wifi className="size-3" />
          {latencyMs} ms
        </span>
      ) : null}
      <span className="ml-auto hidden items-center gap-1.5 md:flex">
        <Cloud className="size-3" />
        {APP_CONFIG.dataSource === 'mock' ? 'Demo data' : 'Self-hosted'}
      </span>
      <span className="hidden items-center gap-1.5 lg:flex">
        <GitBranch className="size-3" />
        local
      </span>
      <span className="flex items-center gap-1.5">
        <Keyboard className="size-3" />
        UTF-8
      </span>
    </footer>
  )
}

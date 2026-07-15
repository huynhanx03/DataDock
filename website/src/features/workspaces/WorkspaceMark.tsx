import { Boxes, Braces, Cloud, Database, FlaskConical, Layers3, Server, Sparkles, type LucideIcon } from 'lucide-react'
import type { Workspace } from '@/entities/workspace'
import { cn } from '@/shared/lib/cn'

export const WORKSPACE_COLORS = ['#7168f5', '#2563eb', '#0891b2', '#0d9488', '#16a34a', '#ca8a04', '#ea580c', '#e11d48'] as const

export const WORKSPACE_ICONS: Array<{ value: string; label: string; icon: LucideIcon }> = [
  { value: 'database', label: 'Database', icon: Database },
  { value: 'server', label: 'Server', icon: Server },
  { value: 'layers', label: 'Layers', icon: Layers3 },
  { value: 'boxes', label: 'Boxes', icon: Boxes },
  { value: 'code', label: 'Code', icon: Braces },
  { value: 'cloud', label: 'Cloud', icon: Cloud },
  { value: 'lab', label: 'Lab', icon: FlaskConical },
  { value: 'sparkles', label: 'Sparkles', icon: Sparkles },
]

const workspaceIconMap = new Map(WORKSPACE_ICONS.map((option) => [option.value, option.icon]))

type WorkspaceMarkProps = {
  workspace: Pick<Workspace, 'name' | 'icon' | 'color'>
  className?: string
  iconClassName?: string
}

export function WorkspaceMark({ workspace, className, iconClassName }: WorkspaceMarkProps) {
  const Icon = workspaceIconMap.get(workspace.icon)
  const fallback = Array.from(workspace.icon.trim() || workspace.name.trim() || 'D').slice(0, 2).join('').toUpperCase()

  return (
    <span
      aria-hidden="true"
      className={cn('grid size-7 shrink-0 place-items-center rounded-lg text-[length:var(--font-size-meta)] font-bold text-white shadow-control', className)}
      style={{ backgroundColor: workspace.color || 'var(--primary)' }}
    >
      {Icon ? <Icon className={cn('size-3.5', iconClassName)} /> : fallback}
    </span>
  )
}

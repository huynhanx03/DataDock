import type { ReactNode } from 'react'

export type WorkspaceViewMode = 'list' | 'grid' | 'table' | 'json' | 'tree' | 'chart' | 'plan'

export type WorkspaceStatus = 'idle' | 'loading' | 'ready' | 'success' | 'warning' | 'error' | 'unavailable'

export type WorkspaceFilterOption = {
  value: string
  label: string
  count?: number
  icon?: ReactNode
}

export type WorkspaceNotice = {
  id: string
  status: Exclude<WorkspaceStatus, 'idle' | 'loading' | 'ready'>
  title: string
  description?: string
}

export type WorkspaceSelection<T> = {
  selected?: T
  selectedIds: Set<string>
}

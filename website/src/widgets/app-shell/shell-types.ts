import type { LucideIcon } from 'lucide-react'
import type { DatabaseObject } from '@/entities/database-object'

export type ActivityId =
  | 'explorer'
  | 'query'
  | 'history'
  | 'saved'
  | 'operations'
  | 'settings'

export type ActivityItem = {
  id: ActivityId
  label: string
  icon: LucideIcon
  route: string
  shortcut?: string
}

export type WorkbenchTabKind = 'welcome' | 'table' | 'query' | 'operations' | 'saved'

export type WorkbenchTab = {
  id: string
  label: string
  kind: WorkbenchTabKind
  icon: LucideIcon
  closable?: boolean
  dirty?: boolean
}

export type InspectorContext = {
  connectionId: string
  connectionName: string
  engine: string
  status: string
  readOnly: boolean
  latencyMs?: number
  object?: DatabaseObject
}

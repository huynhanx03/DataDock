import type { ApiWorkspace } from '../contracts/workspace'
import type { Workspace } from '../../entities/workspace'

export function mapWorkspace(value: ApiWorkspace): Workspace {
  return {
    id: value.id,
    name: value.name,
    icon: value.icon,
    color: value.color,
    position: value.position,
    collapsed: value.collapsed,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
  }
}

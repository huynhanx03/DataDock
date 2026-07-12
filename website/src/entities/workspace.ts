export type Workspace = {
  id: string
  name: string
  icon: string
  color: string
  position: number
  collapsed: boolean
  createdAt: string
  updatedAt: string
}

export type CreateWorkspaceInput = Pick<Workspace, 'name' | 'icon' | 'color'>
export type UpdateWorkspaceInput = Pick<Workspace, 'name' | 'icon' | 'color' | 'collapsed'>

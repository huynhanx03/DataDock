export type ApiSavedQuery = {
  id: string
  connectionId?: string
  folder: string
  title: string
  sql: string
  tags: string[]
  favorite: boolean
  createdAt: string
  updatedAt: string
}

export type ApiSavedQueryFolder = {
  id: string
  name: string
  position: number
  queryCount: number
  createdAt: string
  updatedAt: string
}

export type ApiSavedQueryShare = {
  savedQueryId: string
  shareCode: string
  createdAt: string
  expiresAt?: string
}

export type ApiPublicSavedQuery = {
  title: string
  sql: string
  tags: string[]
  createdAt: string
  sharedAt: string
  expiresAt?: string
}

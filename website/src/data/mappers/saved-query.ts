import type { ApiPublicSavedQuery, ApiSavedQuery, ApiSavedQueryFolder, ApiSavedQueryShare } from '../contracts/saved-query'
import type { PublicSavedQuery, SavedQuery, SavedQueryFolder, SavedQueryShare } from '../../entities/query'

export function mapSavedQuery(value: ApiSavedQuery): SavedQuery {
  return {
    id: value.id,
    connectionId: value.connectionId,
    folder: value.folder,
    title: value.title,
    sql: value.sql,
    tags: [...value.tags],
    favorite: value.favorite,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
  }
}

export function mapSavedQueryFolder(value: ApiSavedQueryFolder): SavedQueryFolder {
  return { ...value }
}

export function mapSavedQueryShare(value: ApiSavedQueryShare): SavedQueryShare {
  return { ...value }
}

export function mapPublicSavedQuery(value: ApiPublicSavedQuery): PublicSavedQuery {
  return { ...value, tags: [...value.tags] }
}

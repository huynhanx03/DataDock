import type { ApiCatalogObject, ApiCatalogTree } from '../contracts/catalog'
import type { CatalogTree, DatabaseObject } from '../../entities/database-object'

export function mapCatalogObject(value: ApiCatalogObject): DatabaseObject {
  return {
    id: value.id,
    reference: value.reference,
    connectionId: value.connectionId,
    parentId: value.parentId,
    name: value.name,
    qualifiedName: value.qualifiedName,
    kind: value.kind,
    database: value.database,
    schema: value.schema,
    dataType: value.dataType,
    count: value.count,
    childrenState: value.childrenState,
    capabilities: [...(value.capabilities ?? [])],
    children: value.children?.map(mapCatalogObject),
  }
}

export function mapCatalog(value: ApiCatalogTree): CatalogTree {
  return {
    connectionId: value.connectionId,
    engine: value.engine,
    capabilities: [...(value.capabilities ?? [])],
    databases: value.databases.map(mapCatalogObject),
    nextCursor: value.nextCursor,
    loadedAt: value.loadedAt,
  }
}

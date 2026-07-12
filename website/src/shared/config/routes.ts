export const APP_ROUTES = Object.freeze({
  home: '/',
  connections: '/connections',
  query: '/query',
  savedQueries: '/saved',
  history: '/history',
  operations: '/operations',
  settings: '/settings',
  table: '/table/:connectionId/:schema/:table',
  schema: '/schema/:connectionId/:schema/:table',
  tablePath: (connectionId: string, schema: string, table: string) =>
    `/table/${encodeURIComponent(connectionId)}/${encodeURIComponent(schema)}/${encodeURIComponent(table)}`,
  schemaPath: (connectionId: string, schema: string, table: string) =>
    `/schema/${encodeURIComponent(connectionId)}/${encodeURIComponent(schema)}/${encodeURIComponent(table)}`,
})

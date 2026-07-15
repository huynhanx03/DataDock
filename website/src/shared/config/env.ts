export type DataSource = 'mock' | 'api'

function readString(name: keyof ImportMetaEnv, fallback: string) {
  const value = import.meta.env[name]
  return typeof value === 'string' && value.trim() ? value.trim() : fallback
}

function readDataSource(): DataSource {
  const value = readString('VITE_DATA_SOURCE', 'api')
  return value === 'mock' ? 'mock' : 'api'
}

export const ENV = Object.freeze({
  apiBaseUrl: readString('VITE_API_BASE_URL', ''),
  dataSource: readDataSource(),
  development: import.meta.env.DEV,
  production: import.meta.env.PROD,
})

declare module '*.css'

interface ImportMetaEnv {
  readonly DEV: boolean
  readonly PROD: boolean
  readonly VITE_API_BASE_URL?: string
  readonly VITE_DATA_SOURCE?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

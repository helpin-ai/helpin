/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly DEV: boolean
  readonly MODE: string
  readonly PROD: boolean
  readonly VITE_API_URL?: string
  readonly VITE_WEB_BASE_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

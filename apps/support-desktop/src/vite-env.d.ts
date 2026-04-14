interface ImportMetaEnv {
  readonly VITE_API_URL?: string
  readonly VITE_WEB_BASE_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

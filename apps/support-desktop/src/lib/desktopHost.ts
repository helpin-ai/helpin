export interface DesktopShellInfo {
  runtime: 'tauri'
  platform: string
  app_version: string
}

declare global {
  interface Window {
    __TAURI_INTERNALS__?: unknown
  }
}

export function isTauriDesktop() {
  return typeof window !== 'undefined' && typeof window.__TAURI_INTERNALS__ !== 'undefined'
}

export async function getDesktopShellInfo() {
  if (!isTauriDesktop()) {
    return null
  }

  const { invoke } = await import('@tauri-apps/api/core')
  return invoke<DesktopShellInfo>('desktop_shell_info')
}

import { openUrl } from '@tauri-apps/plugin-opener'
import { isTauri } from '@mobile/lib/host'

function configuredWebBase() {
  const configured = String(import.meta.env.VITE_WEB_APP_URL ?? '').trim()
  if (configured) return configured
  return typeof window === 'undefined' ? 'http://localhost:5173' : window.location.origin
}

export function buildTaskWebUrl(workspaceSlug: string, taskId: string) {
  const base = configuredWebBase().replace(/\/$/, '')
  return `${base}/w/${encodeURIComponent(workspaceSlug)}/pm/tasks/${encodeURIComponent(taskId)}`
}

export async function openTaskInWeb(workspaceSlug: string, taskId: string) {
  const url = buildTaskWebUrl(workspaceSlug, taskId)
  if (isTauri()) {
    await openUrl(url)
    return
  }
  window.open(url, '_blank', 'noopener,noreferrer')
}

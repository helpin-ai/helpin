import { isTauriDesktop } from './desktopHost'

/**
 * Bring the main desktop window to front: unminimize → show → focus.
 * No-ops gracefully in browser dev mode.
 */
export async function revealDesktopWindow() {
  if (!isTauriDesktop()) return

  try {
    const { getCurrentWindow } = await import('@tauri-apps/api/window')
    const appWindow = getCurrentWindow()
    const isMinimized = await appWindow.isMinimized()
    if (isMinimized) {
      await appWindow.unminimize()
    }
    await appWindow.show()
    await appWindow.setFocus()
  } catch {
    // Ignore focus/show failures.
  }
}

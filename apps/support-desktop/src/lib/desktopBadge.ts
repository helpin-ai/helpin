import { isTauriDesktop } from './desktopHost'

/**
 * Update the app's dock/taskbar badge with the unread count.
 * - macOS: dock badge number
 * - Linux: some DEs support it
 * - Windows: not supported (silently ignored)
 *
 * Pass 0 to clear the badge.
 */
export async function updateBadgeCount(count: number) {
  if (!isTauriDesktop()) return

  try {
    const { getCurrentWindow } = await import('@tauri-apps/api/window')
    const appWindow = getCurrentWindow()
    await appWindow.setBadgeCount(count > 0 ? count : null)
  } catch {
    // Badge not supported on this platform — ignore.
  }
}

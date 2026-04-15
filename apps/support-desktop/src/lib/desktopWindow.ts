import { isTauriDesktop } from './desktopHost'

async function getDesktopWindow() {
  const { getCurrentWindow } = await import('@tauri-apps/api/window')
  return getCurrentWindow()
}

/**
 * Bring the main desktop window to front: unminimize → show → focus.
 * No-ops gracefully in browser dev mode.
 */
export async function revealDesktopWindow() {
  if (!isTauriDesktop()) return

  try {
    const appWindow = await getDesktopWindow()
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

export async function minimizeDesktopWindow() {
  if (!isTauriDesktop()) return

  try {
    const appWindow = await getDesktopWindow()
    await appWindow.minimize()
  } catch {
    // Ignore minimize failures.
  }
}

export async function toggleDesktopWindowMaximize() {
  if (!isTauriDesktop()) return

  try {
    const appWindow = await getDesktopWindow()
    await appWindow.toggleMaximize()
  } catch {
    // Ignore maximize failures.
  }
}

export async function closeDesktopWindow() {
  if (!isTauriDesktop()) return

  try {
    const appWindow = await getDesktopWindow()
    await appWindow.close()
  } catch {
    // Ignore close failures.
  }
}

export async function isDesktopWindowMaximized() {
  if (!isTauriDesktop()) return false

  try {
    const appWindow = await getDesktopWindow()
    return await appWindow.isMaximized()
  } catch {
    return false
  }
}

export async function onDesktopWindowResized(listener: () => void) {
  if (!isTauriDesktop()) return () => {}

  try {
    const appWindow = await getDesktopWindow()
    return await appWindow.onResized(listener)
  } catch {
    return () => {}
  }
}

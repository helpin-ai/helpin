import { useState, useSyncExternalStore } from 'react'
import { useAuthStore } from '@/stores/authStore'
import { DESKTOP_SETTINGS_EVENT, getDesktopChoice, getDesktopPermission, setDesktopEnabled } from '@/lib/desktopNotifications'

function subscribe(callback: () => void) {
  window.addEventListener('storage', callback)
  window.addEventListener('focus', callback)
  window.addEventListener(DESKTOP_SETTINGS_EVENT, callback)
  return () => {
    window.removeEventListener('storage', callback)
    window.removeEventListener('focus', callback)
    window.removeEventListener(DESKTOP_SETTINGS_EVENT, callback)
  }
}

export function useDesktopNotificationSettings() {
  const userId = useAuthStore(state => state.user?.id) ?? ''
  const permission = useSyncExternalStore(subscribe, getDesktopPermission, () => 'unsupported' as const)
  const choice = useSyncExternalStore(subscribe, () => getDesktopChoice(userId), () => false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')

  const toggle = async (value: boolean) => {
    if (!userId || pending) return
    setError('')
    setPending(true)
    try {
      const currentPermission = getDesktopPermission()
      if (value && (currentPermission === 'unsupported' || currentPermission === 'denied')) return
      if (value && currentPermission !== 'granted') {
        const result = await Notification.requestPermission()
        window.dispatchEvent(new Event(DESKTOP_SETTINGS_EVENT))
        // A dismissed browser prompt is a choice too: do not auto-invite again.
        if (useAuthStore.getState().user?.id !== userId) return
        setDesktopEnabled(userId, result === 'granted')
        if (result === 'default') setError('Permission wasn’t granted. Try again when you’re ready.')
        return
      }
      if (useAuthStore.getState().user?.id === userId) setDesktopEnabled(userId, value)
    } catch {
      setError('Couldn’t save desktop notifications. Check your browser permissions and try again.')
    } finally { setPending(false) }
  }

  const dismissPrompt = () => {
    // Reuse the existing persisted off choice; it already survives logout.
    try { setDesktopEnabled(userId, false) }
    catch { setError('Couldn’t save your preference. Check your browser storage settings.') }
  }

  return {
    userId, permission, enabled: choice === true, pending, error, setError, toggle, dismissPrompt,
    shouldPrompt: !!userId && choice === null && (permission === 'default' || permission === 'granted'),
  }
}

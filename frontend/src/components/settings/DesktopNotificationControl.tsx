import { useState, useSyncExternalStore } from 'react'
import { useAuthStore } from '@/stores/authStore'
import { DESKTOP_SETTINGS_EVENT, getDesktopPermission, isDesktopEnabled, setDesktopEnabled } from '@/lib/desktopNotifications'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'

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

export function DesktopNotificationControl() {
  const userId = useAuthStore(state => state.user?.id) ?? ''
  const permission = useSyncExternalStore(subscribe, getDesktopPermission, () => 'unsupported' as const)
  const enabled = useSyncExternalStore(subscribe, () => isDesktopEnabled(userId), () => false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')

  const toggle = async (value: boolean) => {
    setError('')
    setPending(true)
    try {
      if (value && getDesktopPermission() !== 'granted') {
        const result = await Notification.requestPermission()
        window.dispatchEvent(new Event(DESKTOP_SETTINGS_EVENT))
        if (result !== 'granted') {
          if (result === 'default') setError('Permission wasn’t granted. Turn this on again when you’re ready.')
          return
        }
      }
      // Permission prompts can outlive a session change.
      if (useAuthStore.getState().user?.id === userId) setDesktopEnabled(userId, value)
    } catch {
      setError('Couldn’t save desktop notifications. Check your browser permissions and try again.')
    } finally { setPending(false) }
  }

  const test = () => {
    setError('')
    try {
      const notification = new Notification('Helpin desktop notifications', { body: 'You’ll receive alerts here when you switch tabs or applications.', tag: 'helpin:desktop:test' })
      notification.onclick = () => { window.focus(); notification.close() }
    } catch { setError('Couldn’t display the test. Check your browser and system notification settings.') }
  }

  const description = permission === 'unsupported'
    ? 'Desktop notifications need a supported browser and a secure connection.'
    : permission === 'denied'
      ? 'Blocked in this browser. Allow notifications in site settings, then return here.'
      : 'In this browser, while Helpin is open and you’re working elsewhere.'

  return (
    <div className="py-3">
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <p className="text-sm font-medium">Desktop notifications</p>
          <p id="desktop-notifications-description" className="mt-1 text-xs text-quiet-text-secondary">{description}</p>
        </div>
        <Switch aria-label="Desktop notifications" aria-describedby="desktop-notifications-description" checked={enabled && permission === 'granted'} disabled={!userId || pending || permission === 'unsupported' || permission === 'denied'} onCheckedChange={toggle} />
      </div>
      {enabled && permission === 'granted' ? <Button type="button" variant="ghost" size="sm" className="mt-1" onClick={test}>Send test notification</Button> : null}
      {error ? <p role="alert" className="mt-2 text-xs text-destructive">{error}</p> : null}
    </div>
  )
}

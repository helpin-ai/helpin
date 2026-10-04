import { useDesktopNotificationSettings } from '@/hooks/useDesktopNotificationSettings'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'

export function DesktopNotificationControl() {
  const { userId, permission, enabled, pending, error, setError, toggle } = useDesktopNotificationSettings()

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

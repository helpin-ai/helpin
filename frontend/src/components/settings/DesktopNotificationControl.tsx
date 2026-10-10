import { useCallback, useEffect, useRef, useState } from 'react'
import { useDesktopNotificationSettings } from '@/hooks/useDesktopNotificationSettings'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { createDesktopNotification, DESKTOP_SETTINGS_EVENT, getDesktopPermission } from '@/lib/desktopNotifications'

export function DesktopNotificationControl() {
  const { userId, permission, enabled, pending, error, setError, toggle } = useDesktopNotificationSettings()
  const [testing, setTesting] = useState(false)
  const [feedback, setFeedback] = useState('')
  const testNotification = useRef<Notification | null>(null)
  const testTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const clearTest = useCallback(() => {
    if (testTimer.current) clearTimeout(testTimer.current)
    testTimer.current = null
    if (testNotification.current) {
      testNotification.current.onshow = null
      testNotification.current.onerror = null
      testNotification.current.onclick = null
      testNotification.current.close()
      testNotification.current = null
    }
  }, [])

  useEffect(() => {
    setTesting(false)
    setFeedback('')
    return clearTest
  }, [clearTest, userId, enabled, permission])

  const test = () => {
    if (testing) return
    clearTest()
    setError('')
    setFeedback('')
    if (getDesktopPermission() !== 'granted') {
      window.dispatchEvent(new Event(DESKTOP_SETTINGS_EVENT))
      setError('Allow notifications in your browser’s site settings, then try again.')
      return
    }
    setTesting(true)
    const finish = () => {
      if (testTimer.current) clearTimeout(testTimer.current)
      testTimer.current = null
      setTesting(false)
    }
    try {
      const notification = createDesktopNotification('Helpin desktop notifications', {
        body: 'You’ll receive alerts here when you switch tabs or applications.',
        tag: `helpin:desktop:test:${crypto.randomUUID()}`,
      })
      testNotification.current = notification
      notification.onshow = () => { finish(); setFeedback('Test sent to your browser.') }
      notification.onerror = () => {
        finish()
        setFeedback('')
        setError('Couldn’t display the test. Check your browser and system notification settings.')
      }
      notification.onclick = () => { window.focus(); notification.close() }
      testTimer.current = setTimeout(() => {
        finish()
        setFeedback('The browser couldn’t confirm delivery. Check system notification settings.')
      }, 8_000)
    } catch {
      finish()
      setError('Couldn’t display the test. Check your browser and system notification settings.')
    }
  }

  const description = permission === 'unsupported'
    ? 'Desktop notifications need a supported browser and a secure connection.'
    : permission === 'denied'
      ? 'Blocked in this browser. Allow notifications in site settings, then return here.'
      : 'In this browser, while Helpin is open and you’re working elsewhere.'

  return (
    <div className="py-3">
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <div className="min-w-0">
          <p className="text-sm font-medium">Desktop notifications</p>
          <p id="desktop-notifications-description" className="mt-1 text-xs text-quiet-text-secondary">{description}</p>
        </div>
        <div className="ml-auto flex shrink-0 items-center gap-3">
          {enabled && permission === 'granted' ? (
            <Button type="button" variant="ghost" size="sm" aria-label="Send test notification" disabled={testing || pending} onClick={test}>
              {testing ? 'Sending…' : 'Send test notification'}
            </Button>
          ) : null}
          <Switch aria-label="Desktop notifications" aria-describedby="desktop-notifications-description" checked={enabled && permission === 'granted'} disabled={!userId || pending || permission === 'unsupported' || permission === 'denied'} onCheckedChange={toggle} />
        </div>
      </div>
      {error || feedback ? (
        <div className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
          {error ? <p role="alert" className="text-destructive">{error}</p> : <p role="status" className="text-quiet-text-secondary">{feedback}</p>}
          <Tooltip>
            <TooltipTrigger asChild><button type="button" className="text-quiet-text-secondary underline underline-offset-2">No banner?</button></TooltipTrigger>
            <TooltipContent className="max-w-xs">
              {/Mac/.test(navigator.userAgent)
                ? 'In macOS System Settings → Notifications, allow notifications for your browser and enable banners. Also check Focus and screen-sharing notification settings.'
                : 'Allow notifications for your browser in system notification settings. Also check Do Not Disturb and screen-sharing settings.'}
            </TooltipContent>
          </Tooltip>
        </div>
      ) : null}
    </div>
  )
}

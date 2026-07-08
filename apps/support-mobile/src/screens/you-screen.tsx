import { useEffect, useState } from 'react'
import { useNavigate, useParams } from '@tanstack/react-router'
import { useTheme } from 'next-themes'
import { Bell, ChevronRight } from 'lucide-react'
import { toast } from 'sonner'
import { TopBar } from '@mobile/ui/top-bar'
import { TabShell } from '@mobile/navigation/tab-bar'
import { Avatar } from '@mobile/ui/avatar'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { SegmentedControl } from '@mobile/ui/segmented-control'
import { useAuthStore, signOut } from '@mobile/stores/auth-store'
import { useConfirmPress } from '@mobile/lib/use-confirm-press'
import { isTauri } from '@mobile/lib/host'
import { getPushPrimingPref, setPushPrimingPref, type PushPrimingPref } from '@mobile/lib/prefs'
import { registerForPush } from '@mobile/push/push-registration'

/** App version footer caption: real version via the Tauri shell, 'dev' in a plain browser tab. */
function useAppVersion(): string {
  const [version, setVersion] = useState('dev')

  useEffect(() => {
    if (!isTauri()) return
    let cancelled = false
    void import('@tauri-apps/api/core')
      .then(({ invoke }) => invoke<{ app_version: string }>('mobile_shell_info'))
      .then((info) => {
        if (!cancelled) setVersion(info.app_version)
      })
      .catch(() => {
        // Stay on the 'dev' fallback if the native call is unavailable.
      })
    return () => {
      cancelled = true
    }
  }, [])

  return version
}

/**
 * SPIKE / V1.1 note: detecting an OS-level *denied* permission (to show
 * "Off — enable in Settings") requires plugin support we don't have yet —
 * `@helpin/plugin-push` has no "current OS permission state" query, only
 * `getPushToken()` (which re-prompts / returns null) and the token-changed
 * and tap events. Accepted simplification for V1: this row reflects our
 * OWN persisted decision (`enabled` / `later` / never-asked) rather than
 * the true OS permission state. If the user denied the OS prompt, the row
 * still offers "Enable notifications" and re-attempts `registerForPush()`,
 * which returns 'unavailable' again (surfaced as a "Couldn't enable
 * notifications" toast) — not incorrect, just not as informative as a real
 * "denied, go to Settings" deep link would be.
 */
function useNotificationsRowState() {
  const [pref, setPref] = useState<PushPrimingPref | null>(null)
  const [loaded, setLoaded] = useState(false)
  const [enabling, setEnabling] = useState(false)

  useEffect(() => {
    if (!isTauri()) {
      setLoaded(true)
      return
    }
    let cancelled = false
    void getPushPrimingPref().then((value) => {
      if (!cancelled) {
        setPref(value)
        setLoaded(true)
      }
    })
    return () => {
      cancelled = true
    }
  }, [])

  const handleEnable = async () => {
    setEnabling(true)
    try {
      const result = await registerForPush()
      if (result !== 'registered') {
        // Denied OS prompt / simulator / failed POST — don't record
        // 'enabled' (the row would claim Enabled forever with no retry
        // path); leave the pref as-is so the row keeps offering the action.
        toast.error("Couldn't enable notifications")
        return
      }
      const next: PushPrimingPref = { decision: 'enabled', at: new Date().toISOString() }
      await setPushPrimingPref(next)
      setPref(next)
    } catch (error) {
      // `registerForPush()` can reject (e.g. the native plugin's
      // `getPushToken()` throwing) instead of resolving to 'unavailable' —
      // without this catch the rejection escaped unhandled (this handler is
      // invoked via `void notifications.handleEnable()`) and the toast below
      // never showed.
      console.debug('[push] registerForPush rejected', error)
      toast.error("Couldn't enable notifications")
    } finally {
      setEnabling(false)
    }
  }

  return { pref, loaded, enabling, handleEnable }
}

export function YouScreen() {
  const { slug } = useParams({ strict: false })
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.user)
  const { theme, setTheme } = useTheme()
  const appVersion = useAppVersion()
  const { armed, trigger } = useConfirmPress(3000)
  const notifications = useNotificationsRowState()
  const notificationsEnabled = notifications.pref?.decision === 'enabled'

  // workspaceId is unresolved until Task 11 wires the real workspace lookup;
  // TabShell/useUnreadStats stay inert (enabled: !!workspaceId) until then.
  return (
    <TabShell workspaceSlug={slug ?? ''} workspaceId="">
      <div className="flex h-full flex-col overflow-y-auto">
        <TopBar title="You" large />

        <div className="px-4 pb-[var(--safe-bottom)]">
          <section className="flex items-center gap-3 border-b border-border/70 py-3">
            <Avatar name={user?.full_name ?? ''} src={user?.avatar_url} size={44} />
            <span className="flex flex-1 flex-col">
              <span className="text-headline">{user?.full_name}</span>
              <span className="text-footnote text-muted-foreground">{user?.email}</span>
            </span>
          </section>

          <section className="border-b border-border/70 py-1">
            <Pressable
              onPress={() => navigate({ to: '/workspaces' })}
              className="flex w-full items-center justify-between py-2 text-left"
            >
              <span className="text-body">Workspace</span>
              <span className="flex items-center gap-1 text-footnote text-muted-foreground">
                {slug}
                <ChevronRight className="h-4 w-4" />
              </span>
            </Pressable>
          </section>

          {isTauri() && notifications.loaded && (
            <section className="border-b border-border/70 py-1">
              {notificationsEnabled ? (
                <div className="flex w-full items-center justify-between py-2">
                  <span className="flex items-center gap-2 text-body">
                    <Bell className="h-4 w-4 text-muted-foreground" />
                    Notifications
                  </span>
                  <span className="text-footnote text-muted-foreground">Enabled</span>
                </div>
              ) : (
                <Pressable
                  haptic="selection"
                  disabled={notifications.enabling}
                  onPress={() => void notifications.handleEnable()}
                  className="flex w-full items-center justify-between py-2 text-left"
                >
                  <span className="flex items-center gap-2 text-body">
                    <Bell className="h-4 w-4 text-muted-foreground" />
                    Notifications
                  </span>
                  {notifications.enabling ? (
                    <Spinner size={16} />
                  ) : (
                    <span className="text-footnote text-primary">Enable notifications</span>
                  )}
                </Pressable>
              )}
            </section>
          )}

          <section className="border-b border-border/70 py-3">
            <span className="mb-2 block text-footnote text-muted-foreground">Appearance</span>
            <SegmentedControl
              segments={[
                { value: 'light', label: 'Light' },
                { value: 'dark', label: 'Dark' },
                { value: 'system', label: 'System' },
              ]}
              value={(theme as 'light' | 'dark' | 'system' | undefined) ?? 'system'}
              onChange={setTheme}
            />
          </section>

          <section className="py-4">
            <Pressable
              haptic="notificationError"
              onPress={() => trigger(() => void signOut())}
              className="flex w-full items-center justify-center rounded-xl bg-destructive/10 py-3 text-body font-medium text-destructive"
            >
              {armed ? 'Tap again to confirm' : 'Sign out'}
            </Pressable>
          </section>

          <p className="pb-6 pt-2 text-center text-caption text-muted-foreground">Helpin Support v{appVersion}</p>
        </div>
      </div>
    </TabShell>
  )
}

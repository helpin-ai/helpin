import { useEffect, useState } from 'react'
import { useNavigate, useParams } from '@tanstack/react-router'
import { useTheme } from 'next-themes'
import { ChevronRight } from 'lucide-react'
import { TopBar } from '@mobile/ui/top-bar'
import { TabShell } from '@mobile/navigation/tab-bar'
import { Avatar } from '@mobile/ui/avatar'
import { Pressable } from '@mobile/ui/pressable'
import { SegmentedControl } from '@mobile/ui/segmented-control'
import { useAuthStore, signOut } from '@mobile/stores/auth-store'
import { useConfirmPress } from '@mobile/lib/use-confirm-press'
import { isTauri } from '@mobile/lib/host'

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

export function YouScreen() {
  const { slug } = useParams({ strict: false })
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.user)
  const { theme, setTheme } = useTheme()
  const appVersion = useAppVersion()
  const { armed, trigger } = useConfirmPress(3000)

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

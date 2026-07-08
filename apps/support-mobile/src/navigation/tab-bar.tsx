import type { ComponentType, ReactNode } from 'react'
import { motion } from 'motion/react'
import { useRouter, useRouterState } from '@tanstack/react-router'
import { Inbox, CircleUser } from 'lucide-react'
import { useUnreadStats } from '@helpin-ai/support-core'
import { cn } from '@mobile/lib/cn'
import { pressTransition } from '@mobile/lib/motion'
import { Pressable } from '@mobile/ui/pressable'

export type TabKey = 'inbox' | 'you'

/**
 * `0` (or negative, defensively) hides the badge entirely, otherwise the raw
 * count is shown up to 99 and anything above that caps at "99+" so the pill
 * never has to grow past 2 characters.
 */
export function formatBadgeCount(count: number): string | null {
  if (count <= 0) return null
  return count > 99 ? '99+' : String(count)
}

interface TabDef {
  key: TabKey
  label: string
  icon: ComponentType<{ className?: string }>
}

const TABS: TabDef[] = [
  { key: 'inbox', label: 'Inbox', icon: Inbox },
  { key: 'you', label: 'You', icon: CircleUser },
]

export interface TabBarProps {
  activeTab: TabKey
  onNavigate: (tab: TabKey) => void
  /** Unread count shown as a badge on the Inbox tab. Omit/0 hides it. */
  unreadCount?: number
  className?: string
}

/**
 * Pure presentational bottom tab bar — no router or data-fetching
 * dependencies, so it can be exercised directly in tests. `TabShell` below
 * is the wiring layer that supplies `activeTab`/`onNavigate`/`unreadCount`
 * from the router and `useUnreadStats`.
 */
export function TabBar({ activeTab, onNavigate, unreadCount = 0, className }: TabBarProps) {
  const badgeLabel = formatBadgeCount(unreadCount)

  return (
    <nav
      aria-label="Primary"
      className={cn(
        'flex h-[49px] items-stretch border-t border-border bg-background pb-[var(--safe-bottom)]',
        className,
      )}
    >
      {TABS.map(({ key, label, icon: Icon }) => {
        const selected = key === activeTab
        return (
          <Pressable
            key={key}
            haptic="selection"
            aria-pressed={selected}
            onPress={() => onNavigate(key)}
            className={cn(
              'flex flex-1 flex-col items-center justify-center gap-0.5',
              selected ? 'text-primary' : 'text-muted-foreground',
            )}
          >
            <span className="relative inline-flex">
              <motion.span
                className="inline-flex"
                animate={{ scale: selected ? 1.08 : 1 }}
                transition={pressTransition}
              >
                <Icon className="h-6 w-6" />
              </motion.span>
              {key === 'inbox' && badgeLabel && (
                <span className="absolute -right-2 -top-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-caption tnum text-white">
                  {badgeLabel}
                </span>
              )}
            </span>
            <span className="text-caption">{label}</span>
          </Pressable>
        )
      })}
    </nav>
  )
}

const TAB_PATHS: Record<TabKey, string> = {
  inbox: '/w/$slug/support',
  you: '/w/$slug/you',
}

export interface TabShellProps {
  workspaceSlug: string
  workspaceId: string
  children: ReactNode
}

/**
 * Layout wrapper for the two tab-root screens: content fills the remaining
 * height, TabBar is pinned to the bottom. Active tab is derived from the
 * current pathname rather than passed in, so callers don't have to track it.
 */
export function TabShell({ workspaceSlug, workspaceId, children }: TabShellProps) {
  const router = useRouter()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const activeTab: TabKey = pathname.endsWith('/you') ? 'you' : 'inbox'

  // `useUnreadStats` is internally `enabled: !!workspaceId`, so this is inert
  // (no request fires) until a real workspaceId is wired in (Task 11).
  const { data } = useUnreadStats(workspaceId)
  // `my_inbox` (not `total`) is the count of unread conversations assigned to
  // the current user — the badge represents "things waiting on you," not the
  // whole workspace's unread volume (which also includes unassigned/AI-active).
  const unreadCount = data?.my_inbox ?? 0

  const handleNavigate = (tab: TabKey) => {
    if (tab === activeTab) return
    router.navigate({ to: TAB_PATHS[tab], params: { slug: workspaceSlug } })
  }

  return (
    <div className="flex h-dvh flex-col">
      <div className="min-h-0 flex-1 overflow-hidden">{children}</div>
      <TabBar activeTab={activeTab} onNavigate={handleNavigate} unreadCount={unreadCount} />
    </div>
  )
}

import type { ComponentType } from 'react'
import { motion } from 'motion/react'
import { Inbox, Search, Settings, UserRoundCheck } from 'lucide-react'
import { cn } from '@mobile/lib/cn'
import { pressTransition } from '@mobile/lib/motion'
import { Pressable } from '@mobile/ui/pressable'

export type TabKey = 'inbox' | 'mine' | 'search' | 'settings'

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
  { key: 'mine', label: 'Mine', icon: UserRoundCheck },
  { key: 'search', label: 'Search', icon: Search },
  { key: 'settings', label: 'Settings', icon: Settings },
]

export interface TabBarProps {
  activeTab: TabKey
  onNavigate: (tab: TabKey) => void
  /** Per-destination unread counts. Omitted/zero values do not render badges. */
  badges?: Partial<Record<TabKey, number>>
  className?: string
}

/**
 * Pure presentational bottom tab bar — no router or data-fetching
 * dependencies, so it can be exercised directly in tests. Route and inbox
 * filter wiring lives in `PrimaryNavigation`.
 */
export function TabBar({ activeTab, onNavigate, badges = {}, className }: TabBarProps) {
  // Two-layer split (same precedent as TopBar's safe-top handling): the outer
  // nav absorbs the safe-area inset as padding, the inner row keeps the full
  // 49px content height. Putting both on one border-box element would subtract
  // the ~34pt inset from the 49px and crush the 44pt tab items.
  return (
    <nav
      aria-label="Primary"
      className={cn('border-t border-border bg-background pb-[var(--safe-bottom)]', className)}
    >
      <div className="flex h-[49px] items-stretch">
        {TABS.map(({ key, label, icon: Icon }) => {
          const selected = key === activeTab
          const badgeLabel = formatBadgeCount(badges[key] ?? 0)
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
                {badgeLabel && (
                  <span
                    data-testid={`tab-badge-${key}`}
                    className="absolute -right-2 -top-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-caption tnum text-white"
                  >
                    {badgeLabel}
                  </span>
                )}
              </span>
              <span className="text-caption">{label}</span>
            </Pressable>
          )
        })}
      </div>
    </nav>
  )
}

import {
  BadgeCheck,
  Ban,
  Bookmark,
  CircleCheck,
  CircleUser,
  Clock3,
  Inbox as InboxIcon,
  Sparkles,
  Users,
  type LucideIcon,
} from 'lucide-react'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import {
  useInboxScopes,
  useSupportInboxViewCounts,
  useSupportInboxViews,
  useUnreadStats,
} from '@helpin-ai/support-core'
import type { NavFilter } from '@/stores/supportInboxStore'
import { cn } from '@mobile/lib/cn'
import { stackSpring } from '@mobile/lib/motion'
import { formatBadgeCount } from '@mobile/navigation/tab-bar'
import { Pressable } from '@mobile/ui/pressable'
import { Skeleton } from '@mobile/ui/skeleton'
import {
  buildDrawerGroups,
  isItemActive,
  type DrawerItem,
} from './view-list-config'
import type { ViewSelection } from './use-inbox-filters'

const BUILTIN_ICONS: Record<NavFilter, LucideIcon> = {
  inbox: InboxIcon,
  mine: CircleUser,
  waiting: Clock3,
  resolved: CircleCheck,
  spam: Ban,
  ai_active: Sparkles,
  resolved_by_ai: BadgeCheck,
}

function iconForSelection(selection: ViewSelection): LucideIcon {
  switch (selection.kind) {
    case 'builtin':
      return BUILTIN_ICONS[selection.navFilter]
    case 'custom':
      return Bookmark
    case 'mailbox':
      return Users
  }
}

function DrawerRow({
  item,
  active,
  onPress,
}: {
  item: DrawerItem
  active: boolean
  onPress: () => void
}) {
  const Icon = iconForSelection(item.selection)
  // Mirror the web sidebar: the number is the TOTAL (workload) count; a red dot
  // signals unread.
  const badge = formatBadgeCount(item.count.total)
  const hasUnread = item.count.unread > 0
  return (
    <Pressable
      haptic="selection"
      aria-pressed={active}
      onPress={onPress}
      className={cn(
        'mx-2 flex min-h-[44px] items-center gap-3 rounded-xl px-3 text-left transition-colors',
        active ? 'bg-primary/[0.06]' : 'active:bg-muted',
      )}
    >
      <Icon
        className={cn('h-[18px] w-[18px] shrink-0', active ? 'text-primary' : 'text-muted-foreground')}
        strokeWidth={active ? 2.4 : 2}
      />
      {/* Label + unread dot together on the left; the count number sits on the
          far right — mirrors the web sidebar. */}
      <span className="flex min-w-0 flex-1 items-center gap-1.5">
        <span className={cn('truncate text-body', active ? 'font-semibold text-primary' : 'text-foreground')}>
          {item.label}
        </span>
        {hasUnread && (
          <span
            aria-label={`${item.count.unread > 99 ? '99+' : item.count.unread} unread`}
            className="h-1.5 w-1.5 shrink-0 rounded-full bg-red-500"
          />
        )}
      </span>
      {badge && (
        <span
          className={cn(
            'shrink-0 rounded-full px-2 py-0.5 text-footnote tnum',
            active ? 'bg-primary/15 text-primary' : 'bg-muted text-muted-foreground',
          )}
        >
          {badge}
        </span>
      )}
    </Pressable>
  )
}

export interface ViewsDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  workspaceId: string
  workspaceName?: string
  activeSelection: ViewSelection
  onSelect: (selection: ViewSelection) => void
}

/**
 * Left slide-out drawer that mirrors the web support sidebar: Views · AI ·
 * Custom views · Team inboxes, each with a live unread count. Selecting a row
 * updates the active view and closes the drawer.
 */
export function ViewsDrawer({
  open,
  onOpenChange,
  workspaceId,
  workspaceName,
  activeSelection,
  onSelect,
}: ViewsDrawerProps) {
  const reduced = useReducedMotion()
  // Builtin view counts come from unread-stats (like web); custom views from the
  // views/counts endpoint; team inboxes from the inbox scopes.
  const { data: unreadStats } = useUnreadStats(workspaceId, undefined, open)
  const { data: counts = [] } = useSupportInboxViewCounts(workspaceId, open)
  const { data: customViews = [] } = useSupportInboxViews(workspaceId, open)
  const scopes = useInboxScopes(workspaceId, open)
  const loadingSections = scopes.isLoading

  const groups = buildDrawerGroups({
    unreadStats,
    counts,
    customViews,
    scopes: scopes.data,
  })

  const handleSelect = (selection: ViewSelection) => {
    onSelect(selection)
    onOpenChange(false)
  }

  return (
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-50">
          <motion.button
            type="button"
            aria-label="Close menu"
            className="absolute inset-0 bg-[var(--overlay-scrim)]"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.18 }}
            onClick={() => onOpenChange(false)}
          />
          <motion.aside
            role="navigation"
            aria-label="Inbox views"
            className="absolute inset-y-0 left-0 flex w-[82%] max-w-[360px] flex-col border-r border-border bg-background shadow-[0_8px_24px_rgba(0,0,0,0.12)]"
            initial={reduced ? { opacity: 0 } : { x: '-100%' }}
            animate={reduced ? { opacity: 1 } : { x: 0 }}
            exit={reduced ? { opacity: 0 } : { x: '-100%' }}
            transition={reduced ? { duration: 0.15 } : stackSpring}
            drag={reduced ? false : 'x'}
            dragDirectionLock
            dragConstraints={{ left: 0, right: 0 }}
            dragElastic={{ left: 0.6, right: 0 }}
            onDragEnd={(_, info) => {
              if (info.offset.x < -60 || info.velocity.x < -300) onOpenChange(false)
            }}
          >
            {workspaceName && (
              <div className="shrink-0 px-4 pb-2 pt-[calc(var(--safe-top)+12px)]">
                <p className="truncate text-title">{workspaceName}</p>
              </div>
            )}
            <div
              className={cn(
                'flex-1 overflow-y-auto pb-[max(var(--safe-bottom),12px)]',
                workspaceName ? '' : 'pt-[calc(var(--safe-top)+8px)]',
              )}
            >
              {groups.map((group, groupIndex) => (
                <div key={group.title} className={groupIndex === 0 ? 'pt-1' : 'pt-4'}>
                  <p className="px-4 pb-1 text-caption uppercase text-muted-foreground">{group.title}</p>
                  {group.items.map((item) => (
                    <DrawerRow
                      key={item.key}
                      item={item}
                      active={isItemActive(item, activeSelection)}
                      onPress={() => handleSelect(item.selection)}
                    />
                  ))}
                </div>
              ))}
              {loadingSections && (
                <div className="space-y-2 px-4 pt-4">
                  {Array.from({ length: 3 }).map((_, i) => (
                    <Skeleton key={i} className="h-8 w-full rounded-xl" />
                  ))}
                </div>
              )}
            </div>
          </motion.aside>
        </div>
      )}
    </AnimatePresence>
  )
}

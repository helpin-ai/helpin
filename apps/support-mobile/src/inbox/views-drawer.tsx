import {
  BadgeCheck,
  Ban,
  Bookmark,
  Check,
  ChevronDown,
  CircleCheck,
  CircleUser,
  Clock3,
  Inbox as InboxIcon,
  Settings,
  Sparkles,
  Users,
  type LucideIcon,
} from 'lucide-react'
import { useState } from 'react'
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
import { Avatar } from '@mobile/ui/avatar'
import { Skeleton } from '@mobile/ui/skeleton'
import type { Workspace } from '@mobile/lib/types'
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
        'mx-2 flex min-h-[44px] w-[calc(100%-1rem)] items-center gap-3 rounded-xl px-3 text-left transition-colors',
        active ? 'bg-primary/[0.06]' : 'active:bg-muted',
      )}
    >
      <Icon
        className={cn('h-[18px] w-[18px] shrink-0', active ? 'text-primary' : 'text-muted-foreground')}
        strokeWidth={active ? 2.4 : 2}
      />
      <span className="min-w-0 flex-1">
        <span className={cn('truncate text-body', active ? 'font-semibold text-primary' : 'text-foreground')}>
          {item.label}
        </span>
      </span>
      {(hasUnread || badge) && (
        <span className="flex min-w-8 shrink-0 items-center justify-end gap-1.5">
          {hasUnread && (
            <span
              aria-label={`${item.count.unread > 99 ? '99+' : item.count.unread} unread`}
              className="h-1.5 w-1.5 shrink-0 rounded-full bg-red-500"
            />
          )}
          {badge && (
            <span
              className={`shrink-0 text-footnote tnum ${active ? 'text-primary/75' : 'text-muted-foreground'}`}
            >
              {badge}
            </span>
          )}
        </span>
      )}
    </Pressable>
  )
}

export interface ViewsDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  workspaceId: string
  workspace?: Workspace
  workspaces?: Workspace[]
  workspacesLoading?: boolean
  workspacesError?: boolean
  onRetryWorkspaces?: () => void
  onSelectWorkspace?: (workspace: Workspace) => void
  onOpenSettings?: () => void
  activeSelection: ViewSelection
  onSelect: (selection: ViewSelection) => void
}

function WorkspaceSwitcher({
  workspace,
  workspaces = [],
  loading,
  error,
  onRetry,
  onSelect,
  onClose,
}: {
  workspace: Workspace
  workspaces?: Workspace[]
  loading?: boolean
  error?: boolean
  onRetry?: () => void
  onSelect?: (workspace: Workspace) => void
  onClose: () => void
}) {
  const [expanded, setExpanded] = useState(false)
  const reduced = useReducedMotion()
  const options = workspaces.some((item) => item.id === workspace.id)
    ? workspaces
    : [workspace, ...workspaces]

  const handleSelect = (nextWorkspace: Workspace) => {
    if (nextWorkspace.id === workspace.id) {
      setExpanded(false)
      return
    }
    onSelect?.(nextWorkspace)
    onClose()
  }

  return (
    <div className="shrink-0 border-b border-border/70 px-2 pb-2 pt-[calc(var(--safe-top)+8px)]">
      <Pressable
        haptic="selection"
        aria-label={`Switch workspace, currently ${workspace.name}`}
        aria-expanded={expanded}
        onPress={() => setExpanded((value) => !value)}
        className="flex min-h-[52px] w-full items-center gap-3 rounded-xl px-2 text-left active:bg-muted"
      >
        <Avatar name={workspace.name} src={workspace.logo_url} size={36} />
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="text-caption text-muted-foreground">Workspace</span>
          <span className="truncate text-headline">{workspace.name}</span>
        </span>
        <motion.span
          aria-hidden
          animate={{ rotate: expanded ? 180 : 0 }}
          transition={reduced ? { duration: 0 } : { duration: 0.18, ease: 'easeOut' }}
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted/70 text-muted-foreground"
        >
          <ChevronDown className="h-4 w-4" />
        </motion.span>
      </Pressable>

      <AnimatePresence initial={false}>
        {expanded && (
          <motion.div
            initial={reduced ? false : { height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={reduced ? { opacity: 0 } : { height: 0, opacity: 0 }}
            transition={reduced ? { duration: 0 } : { duration: 0.2, ease: 'easeOut' }}
            className="overflow-hidden"
          >
            <div
              data-testid="workspace-options"
              className="max-h-[min(42dvh,320px)] space-y-0.5 overflow-y-auto overscroll-contain pb-1 pt-1"
            >
              {options.map((item) => {
                const current = item.id === workspace.id
                const content = (
                  <>
                    <Avatar name={item.name} src={item.logo_url} size={28} />
                    <span className={cn('min-w-0 flex-1 truncate text-body', current && 'font-medium text-primary')}>
                      {item.name}
                    </span>
                    {current && <Check className="h-4 w-4 shrink-0 text-primary" strokeWidth={2.5} />}
                  </>
                )

                return current ? (
                  <div
                    key={item.id}
                    aria-label={`${item.name}, current workspace`}
                    aria-current="true"
                    className="flex min-h-[44px] w-full items-center gap-3 rounded-xl bg-primary/[0.06] px-2 text-left"
                  >
                    {content}
                  </div>
                ) : (
                  <Pressable
                    key={item.id}
                    haptic="selection"
                    aria-label={`Switch to ${item.name}`}
                    onPress={() => handleSelect(item)}
                    className="flex min-h-[44px] w-full items-center gap-3 rounded-xl px-2 text-left active:bg-muted"
                  >
                    {content}
                  </Pressable>
                )
              })}
              {loading && workspaces.length === 0 && (
                <div className="flex min-h-[44px] items-center gap-3 px-2" aria-label="Loading workspaces">
                  <Skeleton className="h-7 w-7 rounded-lg" />
                  <Skeleton className="h-3.5 w-32" />
                </div>
              )}
              {error && (
                <div className="flex min-h-[44px] items-center justify-between gap-3 px-2 text-footnote">
                  <span className="text-muted-foreground">Couldn't load workspaces</span>
                  <Pressable
                    haptic="selection"
                    aria-label="Retry loading workspaces"
                    onPress={onRetry}
                    className="shrink-0 rounded-full px-3 py-1.5 font-medium text-primary active:bg-primary/10"
                  >
                    Retry
                  </Pressable>
                </div>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
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
  workspace,
  workspaces,
  workspacesLoading,
  workspacesError,
  onRetryWorkspaces,
  onSelectWorkspace,
  onOpenSettings,
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
            {workspace && (
              <WorkspaceSwitcher
                workspace={workspace}
                workspaces={workspaces}
                loading={workspacesLoading}
                error={workspacesError}
                onRetry={onRetryWorkspaces}
                onSelect={onSelectWorkspace}
                onClose={() => onOpenChange(false)}
              />
            )}
            <div
              className={cn(
                'flex-1 overflow-y-auto pb-3',
                workspace ? '' : 'pt-[calc(var(--safe-top)+8px)]',
              )}
            >
              {groups.map((group, groupIndex) => {
                const showLabel = group.title !== 'Views' && group.title !== 'AI'
                return (
                  <div key={group.title} className={groupIndex === 0 ? 'pt-2' : showLabel ? 'pt-4' : ''}>
                    {showLabel && (
                      <p className="px-4 pb-1 text-caption uppercase text-muted-foreground">{group.title}</p>
                    )}
                    {group.items.map((item) => (
                      <DrawerRow
                        key={item.key}
                        item={item}
                        active={isItemActive(item, activeSelection)}
                        onPress={() => handleSelect(item.selection)}
                      />
                    ))}
                  </div>
                )
              })}
              {loadingSections && (
                <div className="space-y-2 px-4 pt-4">
                  {Array.from({ length: 3 }).map((_, i) => (
                    <Skeleton key={i} className="h-8 w-full rounded-xl" />
                  ))}
                </div>
              )}
            </div>
            <div
              data-testid="drawer-footer"
              className="shrink-0 border-t border-border bg-background px-2 pb-[max(var(--safe-bottom),8px)] pt-2"
            >
              <Pressable
                haptic="selection"
                onPress={() => {
                  onOpenChange(false)
                  onOpenSettings?.()
                }}
                className="flex min-h-[48px] w-full items-center gap-3 rounded-xl px-3 text-left active:bg-muted"
              >
                <Settings className="h-[18px] w-[18px] text-muted-foreground" />
                <span className="text-body font-medium">Settings</span>
              </Pressable>
            </div>
          </motion.aside>
        </div>
      )}
    </AnimatePresence>
  )
}

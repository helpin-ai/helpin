import { useState, useMemo } from 'react'
import { useTitle } from '@/hooks/useTitle'
import { useLocation, useNavigate } from '@tanstack/react-router'
import {
  Notification02Icon,
  TickDouble01Icon,
  ArchiveIcon,
  Clock01Icon,
  Delete01Icon,
  ViewIcon,
  ViewOffIcon,
  MoreVerticalIcon,
  InboxIcon,
  UserAdd01Icon,
  AtIcon,
} from '@/lib/icons'
import { formatDistanceToNow } from 'date-fns'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation'
import { getExternalMCPNotificationTarget, getNotificationTaskRunTarget } from '@/lib/notificationNavigation'
import {
  useNotifications,
  useUnreadCount,
  useMarkAsRead,
  useMarkAsUnread,
  useArchiveNotification,
  useMarkAllAsRead,
  useArchiveAllRead,
  useDeleteNotification,
  useSnoozeNotification,
} from '@/hooks/queries'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn, getInitials } from '@/lib/utils'
import type { Notification, NotificationFilter } from '@/lib/notificationTypes'

const FILTERS: { key: NotificationFilter; label: string; icon: typeof InboxIcon }[] = [
  { key: 'all', label: 'All Notifications', icon: InboxIcon },
  { key: 'mentions', label: 'Mentions', icon: AtIcon },
  { key: 'assigned', label: 'Assigned to me', icon: UserAdd01Icon },
]

function getTimeAgo(dateStr: string): string {
  try {
    return formatDistanceToNow(new Date(dateStr), { addSuffix: true })
  } catch {
    return ''
  }
}

function getPriorityDot(priority: string): string {
  switch (priority) {
    case 'urgent': return 'bg-red-500'
    case 'high': return 'bg-orange-500'
    case 'normal': return 'bg-blue-500'
    default: return 'bg-muted-foreground/30'
  }
}

function getCategoryLabel(category: string): string {
  switch (category) {
    case 'assignment': return 'Assignment'
    case 'comment': return 'Comment'
    case 'mention': return 'Mention'
    case 'status_change': return 'Status'
    case 'completion': return 'Completed'
    case 'blocked': return 'Blocked'
    case 'sprint': return 'Sprint'
    case 'assignments': return 'Assignment'
    case 'comments': return 'Comment'
    case 'mentions': return 'Mention'
    case 'status_changes': return 'Status'
    case 'subscriptions': return 'Update'
    case 'agent_attention': return 'Agent attention'
    case 'support_replies': return 'Customer reply'
    case 'support_mentions': return 'Support mention'
    default: return category
  }
}

/** Split text on @mentions and wrap each mention in a highlighted span. */
function highlightMentions(text: string): React.ReactNode {
  const parts = text.split(/(@[a-zA-Z0-9][a-zA-Z0-9._-]*)/g)
  if (parts.length === 1) return text
  return parts.map((part, i) =>
    part.startsWith('@') ? (
      <span key={i} className="font-medium text-primary">{part}</span>
    ) : (
      part
    ),
  )
}

function NotificationListItem({
  notification,
  isSelected,
  onSelect,
  onRead,
  onUnread,
  onArchive,
  onDelete,
  onSnooze,
}: {
  notification: Notification
  isSelected: boolean
  onSelect: () => void
  onRead: (id: string) => void
  onUnread: (id: string) => void
  onArchive: (id: string) => void
  onDelete: (id: string) => void
  onSnooze: (id: string, until: string) => void
}) {
  const isUnread = notification.status === 'unread'
  const identifier = notification.entity_snapshot?.identifier
  const actorName = notification.actor_snapshot?.name
  const actionTitle = actorName && notification.title.startsWith(`${actorName} `)
    ? notification.title.slice(actorName.length + 1)
    : notification.title

  return (
    <div
      className={cn(
        'group relative flex cursor-pointer gap-3 px-4 py-3 transition-colors border-b border-border/50',
        isSelected
          ? 'bg-accent/50'
          : isUnread
            ? 'bg-primary/[0.02] hover:bg-accent/30'
            : 'hover:bg-accent/30',
      )}
      onClick={() => {
        onSelect()
        if (isUnread) onRead(notification.id)
      }}
    >
      {/* Unread dot */}
      <div className="mt-2 shrink-0">
        {isUnread ? (
          <div className={cn('h-2 w-2 rounded-full', getPriorityDot(notification.priority))} />
        ) : (
          <div className="h-2 w-2" />
        )}
      </div>

      {/* Content */}
      <div className="min-w-0 flex-1">
        <div className="flex items-start justify-between gap-2">
          <p className={cn('text-sm leading-snug line-clamp-2', isUnread && 'font-medium')}>
            {notification.actor_snapshot?.name && (
              <span className="text-primary">{notification.actor_snapshot.name}</span>
            )}{' '}
            {actionTitle}
          </p>
          <span className="shrink-0 text-[11px] text-muted-foreground whitespace-nowrap mt-0.5">
            {getTimeAgo(notification.last_event_at)}
          </span>
        </div>
        {notification.body && (
          <p className="mt-0.5 text-xs text-muted-foreground line-clamp-1">
            {highlightMentions(notification.body)}
          </p>
        )}
        <div className="mt-1.5 flex items-center gap-2">
          {identifier && (
            <span className="inline-flex items-center rounded bg-muted px-1.5 py-0.5 text-[10px] font-mono text-muted-foreground">
              {identifier}
            </span>
          )}
          <span className="text-[10px] text-muted-foreground">
            {getCategoryLabel(notification.latest_event_category)}
          </span>
          {notification.event_count > 1 && (
            <span className="text-[10px] text-muted-foreground">
              · {notification.event_count} updates
            </span>
          )}
        </div>
      </div>

      {/* Row actions */}
      <div className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity self-center">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={(e) => e.stopPropagation()}>
              <MoreVerticalIcon className="h-3.5 w-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48">
            {isUnread ? (
              <DropdownMenuItem onClick={() => onRead(notification.id)}>
                <ViewIcon className="h-3.5 w-3.5 mr-2" /> Mark as read
              </DropdownMenuItem>
            ) : (
              <DropdownMenuItem onClick={() => onUnread(notification.id)}>
                <ViewOffIcon className="h-3.5 w-3.5 mr-2" /> Mark as unread
              </DropdownMenuItem>
            )}
            <DropdownMenuItem onClick={() => onArchive(notification.id)}>
              <ArchiveIcon className="h-3.5 w-3.5 mr-2" /> Archive
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => {
              const inOneHour = new Date(Date.now() + 60 * 60 * 1000)
              onSnooze(notification.id, inOneHour.toISOString())
            }}>
              <Clock01Icon className="h-3.5 w-3.5 mr-2" /> Snooze 1 hour
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => {
              const tomorrow = new Date()
              tomorrow.setDate(tomorrow.getDate() + 1)
              tomorrow.setHours(9, 0, 0, 0)
              onSnooze(notification.id, tomorrow.toISOString())
            }}>
              <Clock01Icon className="h-3.5 w-3.5 mr-2" /> Snooze until tomorrow
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => onDelete(notification.id)} className="text-destructive">
              <Delete01Icon className="h-3.5 w-3.5 mr-2" /> Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  )
}

function NotificationDetail({ notification }: { notification: Notification }) {
  const navigate = useNavigate()
  const location = useLocation()
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsSlug = workspace?.slug ?? ''

  const actorName = notification.actor_snapshot?.name || 'Someone'
  const entityTitle = notification.entity_snapshot?.title || ''
  const identifier = notification.entity_snapshot?.identifier || ''
  const entityState = notification.entity_snapshot?.state || ''
  const parentTitle = notification.parent_entity_snapshot?.title
  const parentIdentifier = notification.parent_entity_snapshot?.identifier
  const openLabel = notification.entity_type === 'external_mcp_server'
    ? 'Open connection'
    : notification.entity_type === 'support_conversation'
    ? 'Open conversation'
    : notification.entity_type === 'agent_run'
      ? 'Open task run'
      : `Open ${notification.entity_type}`

  const handleNavigateToEntity = () => {
    const type = notification.entity_type
    const id = notification.entity_id
    const externalMCPRoute = getExternalMCPNotificationTarget(notification, wsSlug)
    if (externalMCPRoute) {
      navigate({ to: externalMCPRoute })
      return
    }
    const taskRunTarget = getNotificationTaskRunTarget(notification)
    if (taskRunTarget) {
      if (!wsSlug) return
      openTaskRoute(navigate as never, location as never, wsSlug, taskRunTarget.taskId, { run: taskRunTarget.runId })
    } else if (type === 'epic') {
      navigate({ to: '/w/$slug/pm/epics/$epicId' as string, params: { slug: wsSlug, epicId: id } })
    } else if (type === 'objective') {
      navigate({ to: '/w/$slug/pm/objectives/$objectiveId' as string, params: { slug: wsSlug, objectiveId: id } })
    } else if (type === 'support_conversation') {
      navigate({ to: '/w/$slug/support/$conversationId' as string, params: { slug: wsSlug, conversationId: id } })
    } else if (type === 'doc' || type === 'document') {
      navigate({ to: '/w/$slug/docs/documents/$docId' as string, params: { slug: wsSlug, docId: id } })
    }
  }

  return (
    <div className="flex h-full flex-col">
      {/* Detail header */}
      <div className="border-b px-6 py-4">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 flex-1">
            {parentTitle && (
              <p className="text-xs text-muted-foreground mb-1">
                {parentIdentifier && <span className="font-mono mr-1">{parentIdentifier}</span>}
                {parentTitle}
              </p>
            )}
            <div className="flex items-center gap-2">
              {identifier && (
                <span className="shrink-0 text-xs font-mono text-muted-foreground">{identifier}</span>
              )}
              <h2 className="text-lg font-semibold leading-tight">{entityTitle || notification.title}</h2>
            </div>
            {entityState && (
              <div className="mt-2 flex items-center gap-2">
                <span className="inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs">
                  {entityState}
                </span>
              </div>
            )}
          </div>
          <Button variant="outline" size="sm" className="shrink-0" onClick={handleNavigateToEntity}>
            {openLabel}
          </Button>
        </div>
      </div>

      {/* Notification content */}
      <ScrollArea className="flex-1">
        <div className="px-6 py-4 space-y-4">
          {/* Latest event */}
          <div className="rounded-lg border p-4">
            <div className="flex items-center gap-2 mb-2">
              <div className="flex h-7 w-7 items-center justify-center rounded-full bg-primary/10 text-xs font-medium text-primary">
                {getInitials(actorName)}
              </div>
              <div>
                <p className="text-sm font-medium text-primary">{actorName}</p>
                <p className="text-[11px] text-muted-foreground">{getTimeAgo(notification.last_event_at)}</p>
              </div>
            </div>
            <p className="text-sm">{notification.title}</p>
            {notification.body && (
              <p className="mt-2 text-sm text-muted-foreground">{highlightMentions(notification.body)}</p>
            )}
            {notification.metadata && Object.keys(notification.metadata).length > 0 && (
              <div className="mt-3 space-y-1">
                {!!notification.metadata.comment_preview && (
                  <blockquote className="border-l-2 border-muted-foreground/30 pl-3 text-sm italic text-muted-foreground">
                    {String(notification.metadata.comment_preview)}
                  </blockquote>
                )}
                {!!notification.metadata.old_value && !!notification.metadata.new_value && (
                  <p className="text-xs text-muted-foreground">
                    Changed from <span className="font-medium">{String(notification.metadata.old_value)}</span> to{' '}
                    <span className="font-medium">{String(notification.metadata.new_value)}</span>
                  </p>
                )}
              </div>
            )}
          </div>

          {/* Event count info */}
          {notification.event_count > 1 && (
            <p className="text-xs text-muted-foreground text-center">
              {notification.event_count} total updates on this {notification.entity_type}
            </p>
          )}

          {/* Activity section */}
          <div>
            <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3">Activity</h3>
            <div className="flex items-start gap-3 rounded-lg border p-3">
              <div className="flex h-6 w-6 items-center justify-center rounded-full bg-muted text-[10px] font-medium">
                {getInitials(actorName)}
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm">
                  <span className="font-medium text-primary">{actorName}</span>{' '}
                  <span className="text-muted-foreground">
                    {getCategoryLabel(notification.latest_event_category).toLowerCase()} · {getTimeAgo(notification.last_event_at)}
                  </span>
                </p>
              </div>
            </div>
          </div>
        </div>
      </ScrollArea>
    </div>
  )
}

export function NotificationsPage() {
  useTitle('Notifications')
  const [filter, setFilter] = useState<NotificationFilter>('all')
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id || ''

  const { data: unreadData } = useUnreadCount(wsId)
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage } = useNotifications(wsId, filter)
  const markAsRead = useMarkAsRead(wsId)
  const markAsUnread = useMarkAsUnread(wsId)
  const archive = useArchiveNotification(wsId)
  const markAllRead = useMarkAllAsRead(wsId)
  const archiveAllRead = useArchiveAllRead(wsId)
  const deleteNotif = useDeleteNotification(wsId)
  const snooze = useSnoozeNotification(wsId)

  const unreadCount = unreadData?.count ?? 0
  const notifications = useMemo(
    () => data?.pages.flatMap((p) => p.data) ?? [],
    [data],
  )

  const selected = useMemo(
    () => notifications.find((n) => n.id === selectedId) ?? null,
    [notifications, selectedId],
  )

  return (
    <div className="flex h-full">
      {/* Left panel - notification list */}
      <div className="flex w-[380px] shrink-0 flex-col border-r">
        {/* List header */}
        <div className="flex items-center justify-between border-b px-4 py-3">
          <div className="flex items-center gap-2">
            <Notification02Icon className="h-4 w-4" />
            <h1 className="text-sm font-semibold">Notifications</h1>
            {unreadCount > 0 && (
              <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-primary px-1.5 text-[10px] font-medium text-primary-foreground">
                {unreadCount}
              </span>
            )}
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-7 w-7">
                <MoreVerticalIcon className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-48">
              <DropdownMenuItem onClick={() => markAllRead.mutate()}>
                <TickDouble01Icon className="h-3.5 w-3.5 mr-2" /> Mark all as read
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => archiveAllRead.mutate()}>
                <ArchiveIcon className="h-3.5 w-3.5 mr-2" /> Archive all read
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        {/* Filter tabs */}
        <div className="border-b">
          {FILTERS.map((f) => (
            <button
              key={f.key}
              type="button"
              className={cn(
                'inline-flex items-center gap-1.5 px-4 py-2.5 text-xs font-medium transition-colors border-b-2',
                filter === f.key
                  ? 'border-primary text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground'
              )}
              onClick={() => setFilter(f.key)}
            >
              <f.icon className="h-3 w-3" />
              {f.label}
            </button>
          ))}
        </div>

        {/* Notification list */}
        <ScrollArea className="flex-1">
          {notifications.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
              <InboxIcon className="h-10 w-10 mb-3 opacity-30" />
              <p className="text-sm font-medium">No notifications</p>
              <p className="text-xs mt-1">You're all caught up</p>
            </div>
          ) : (
            <>
              {notifications.map((notif) => (
                <NotificationListItem
                  key={notif.id}
                  notification={notif}
                  isSelected={selectedId === notif.id}
                  onSelect={() => setSelectedId(notif.id)}
                  onRead={(id) => markAsRead.mutate(id)}
                  onUnread={(id) => markAsUnread.mutate(id)}
                  onArchive={(id) => {
                    archive.mutate(id)
                    if (selectedId === id) setSelectedId(null)
                  }}
                  onDelete={(id) => {
                    deleteNotif.mutate(id)
                    if (selectedId === id) setSelectedId(null)
                  }}
                  onSnooze={(id, until) => {
                    snooze.mutate({ notifId: id, until })
                    if (selectedId === id) setSelectedId(null)
                  }}
                />
              ))}
              {hasNextPage && (
                <div className="flex justify-center py-4">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-xs"
                    onClick={() => fetchNextPage()}
                    disabled={isFetchingNextPage}
                  >
                    {isFetchingNextPage ? 'Loading...' : 'Load more'}
                  </Button>
                </div>
              )}
            </>
          )}
        </ScrollArea>
      </div>

      {/* Right panel - detail view */}
      <div className="flex-1 min-w-0">
        {selected ? (
          <NotificationDetail notification={selected} />
        ) : (
          <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
            <Notification02Icon className="h-12 w-12 mb-3 opacity-20" />
            <p className="text-sm font-medium">Select a notification</p>
            <p className="text-xs mt-1">Click on a notification to view details</p>
          </div>
        )}
      </div>
    </div>
  )
}

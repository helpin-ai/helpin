import { useState } from 'react'
import { Bell, CheckCheck, Archive, Clock, Trash2, Eye, EyeOff } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { useNavigate } from '@tanstack/react-router'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import {
  useNotifications,
  useUnreadCount,
  useMarkAsRead,
  useMarkAsUnread,
  useArchiveNotification,
  useMarkAllAsRead,
  useDeleteNotification,
  useSnoozeNotification,
} from '@/hooks/queries'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'
import type { Notification, NotificationFilter } from '@/lib/notificationTypes'

const TABS: { key: NotificationFilter; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'mentions', label: 'Mentions' },
  { key: 'assigned', label: 'Assigned' },
]

function getTimeAgo(dateStr: string): string {
  try {
    return formatDistanceToNow(new Date(dateStr), { addSuffix: false })
  } catch {
    return ''
  }
}

function getPriorityColor(priority: string): string {
  switch (priority) {
    case 'urgent': return 'bg-red-500'
    case 'high': return 'bg-orange-500'
    case 'normal': return 'bg-blue-500'
    default: return 'bg-muted-foreground/40'
  }
}

function NotificationRow({
  notification,
  onRead,
  onUnread,
  onArchive,
  onDelete,
  onSnooze,
  onNavigate,
}: {
  notification: Notification
  onRead: (id: string) => void
  onUnread: (id: string) => void
  onArchive: (id: string) => void
  onDelete: (id: string) => void
  onSnooze: (id: string, until: string) => void
  onNavigate: (notification: Notification) => void
}) {
  const isUnread = notification.status === 'unread'
  const identifier = notification.entity_snapshot?.identifier || ''

  return (
    <div
      className={cn(
        'group relative flex gap-3 px-4 py-3 transition-colors hover:bg-muted/50 cursor-pointer',
        isUnread && 'bg-primary/[0.03]'
      )}
      onClick={() => {
        if (isUnread) onRead(notification.id)
        onNavigate(notification)
      }}
    >
      {/* Unread indicator */}
      <div className="mt-1.5 shrink-0">
        {isUnread ? (
          <div className={cn('h-2 w-2 rounded-full', getPriorityColor(notification.priority))} />
        ) : (
          <div className="h-2 w-2" />
        )}
      </div>

      {/* Content */}
      <div className="min-w-0 flex-1">
        <p className={cn('text-sm leading-snug', isUnread ? 'font-medium' : 'text-muted-foreground')}>
          {notification.title}
        </p>
        {notification.body && (
          <p className="mt-0.5 text-xs text-muted-foreground line-clamp-1">
            {notification.body}
          </p>
        )}
        <div className="mt-1 flex items-center gap-2 text-[11px] text-muted-foreground">
          {identifier && <span className="font-mono">{identifier}</span>}
          {notification.event_count > 1 && (
            <span>{notification.event_count} updates</span>
          )}
        </div>
      </div>

      {/* Time + actions */}
      <div className="shrink-0 flex flex-col items-end gap-1">
        <span className="text-[11px] text-muted-foreground whitespace-nowrap">
          {getTimeAgo(notification.last_event_at)}
        </span>
        <div className="opacity-0 group-hover:opacity-100 transition-opacity">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-6 w-6" onClick={(e) => e.stopPropagation()}>
                <svg className="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 16 16">
                  <circle cx="8" cy="3" r="1.5" />
                  <circle cx="8" cy="8" r="1.5" />
                  <circle cx="8" cy="13" r="1.5" />
                </svg>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-44">
              {isUnread ? (
                <DropdownMenuItem onClick={() => onRead(notification.id)}>
                  <Eye className="h-3.5 w-3.5 mr-2" /> Mark as read
                </DropdownMenuItem>
              ) : (
                <DropdownMenuItem onClick={() => onUnread(notification.id)}>
                  <EyeOff className="h-3.5 w-3.5 mr-2" /> Mark as unread
                </DropdownMenuItem>
              )}
              <DropdownMenuItem onClick={() => onArchive(notification.id)}>
                <Archive className="h-3.5 w-3.5 mr-2" /> Archive
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => {
                const tomorrow = new Date()
                tomorrow.setDate(tomorrow.getDate() + 1)
                tomorrow.setHours(9, 0, 0, 0)
                onSnooze(notification.id, tomorrow.toISOString())
              }}>
                <Clock className="h-3.5 w-3.5 mr-2" /> Snooze until tomorrow
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => onDelete(notification.id)} className="text-destructive">
                <Trash2 className="h-3.5 w-3.5 mr-2" /> Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </div>
  )
}

function getEntityRoute(slug: string, entityType: string, entityId: string): string | null {
  switch (entityType) {
    case 'story': return `/w/${slug}/pm/stories/${entityId}`
    case 'epic': return `/w/${slug}/pm/epics/${entityId}`
    case 'objective': return `/w/${slug}/pm/objectives/${entityId}`
    case 'sprint': return `/w/${slug}/pm/sprints`
    case 'support_conversation': return `/w/${slug}/support/${entityId}`
    default: return null
  }
}

export function NotificationCenter() {
  const [open, setOpen] = useState(false)
  const [filter, setFilter] = useState<NotificationFilter>('all')
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id || ''
  const slug = workspace?.slug || ''
  const navigate = useNavigate()

  const { data: unreadData } = useUnreadCount(wsId)
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage } = useNotifications(wsId, filter)
  const markAsRead = useMarkAsRead(wsId)
  const markAsUnread = useMarkAsUnread(wsId)
  const archive = useArchiveNotification(wsId)
  const markAllRead = useMarkAllAsRead(wsId)
  const deleteNotif = useDeleteNotification(wsId)
  const snooze = useSnoozeNotification(wsId)

  const unreadCount = unreadData?.count ?? 0
  const notifications = data?.pages.flatMap((p) => p.data) ?? []

  const handleNavigate = (notification: Notification) => {
    const route = getEntityRoute(slug, notification.entity_type, notification.entity_id)
    if (route) {
      setOpen(false)
      navigate({ to: route })
    }
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="relative h-8 w-8 text-muted-foreground hover:text-foreground"
          aria-label="Notifications"
        >
          <Bell className="h-4 w-4" />
          {unreadCount > 0 && (
            <span className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-medium text-primary-foreground">
              {unreadCount > 99 ? '99+' : unreadCount}
            </span>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className="w-[400px] p-0"
        align="end"
        sideOffset={8}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b px-4 py-3">
          <h3 className="text-sm font-semibold">Notifications</h3>
          {unreadCount > 0 && (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 gap-1.5 text-xs text-muted-foreground"
              onClick={() => markAllRead.mutate()}
            >
              <CheckCheck className="h-3.5 w-3.5" />
              Mark all read
            </Button>
          )}
        </div>

        {/* Tabs */}
        <div className="flex border-b px-4">
          {TABS.map((tab) => (
            <button
              key={tab.key}
              type="button"
              className={cn(
                'px-3 py-2 text-xs font-medium transition-colors border-b-2',
                filter === tab.key
                  ? 'border-primary text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground'
              )}
              onClick={() => setFilter(tab.key)}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Notification list */}
        <ScrollArea className="max-h-[420px]">
          {notifications.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <Bell className="h-8 w-8 mb-2 opacity-40" />
              <p className="text-sm">No notifications</p>
              <p className="text-xs mt-1">You're all caught up</p>
            </div>
          ) : (
            <div className="divide-y">
              {notifications.map((notif) => (
                <NotificationRow
                  key={notif.id}
                  notification={notif}
                  onRead={(id) => markAsRead.mutate(id)}
                  onUnread={(id) => markAsUnread.mutate(id)}
                  onArchive={(id) => archive.mutate(id)}
                  onDelete={(id) => deleteNotif.mutate(id)}
                  onSnooze={(id, until) => snooze.mutate({ notifId: id, until })}
                  onNavigate={handleNavigate}
                />
              ))}
              {hasNextPage && (
                <div className="flex justify-center py-3">
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
            </div>
          )}
        </ScrollArea>
      </PopoverContent>
    </Popover>
  )
}

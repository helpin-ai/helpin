import { useCallback, useMemo, useState } from 'react'
import { formatDistanceToNowStrict, parseISO } from 'date-fns'
import { toast } from 'sonner'
import { UserAvatar } from '@/components/pm/UserAvatar'
import { CommentBody } from '@/components/pm/CommentBody'
import { CommentEditor } from '@/components/pm/CommentEditor'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import {
  ArrowReloadHorizontalIcon,
  ArrowTurnBackwardIcon,
  CheckmarkCircle02Icon,
  Delete01Icon,
  MoreHorizontalIcon,
  PencilEdit01Icon,
  SmilePlusIcon,
} from '@/lib/icons'
import { docsCommentService } from '@/lib/services/docsCommentService'
import type { CommentWithAuthor } from '@/lib/pmTypes'
import type { AssignableMember, WorkspaceTeam } from '@/lib/types'

const REACTION_EMOJIS = [
  { emoji: '👍', label: 'Thumbs up' },
  { emoji: '❤️', label: 'Heart' },
  { emoji: '🎉', label: 'Celebrate' },
  { emoji: '😄', label: 'Smile' },
  { emoji: '👀', label: 'Eyes' },
  { emoji: '🚀', label: 'Rocket' },
]

function relTime(dateStr: string): string {
  try {
    const raw = formatDistanceToNowStrict(parseISO(dateStr), { addSuffix: false })
    if (raw.startsWith('0 ')) return 'Just now'
    return raw
      .replace(/ seconds?/, 's')
      .replace(/ minutes?/, 'm')
      .replace(/ hours?/, 'h')
      .replace(/ days?/, 'd')
      .replace(/ weeks?/, 'w')
      .replace(/ months?/, 'mo')
      .replace(/ years?/, 'y')
  } catch {
    return dateStr
  }
}

interface InlineCommentCardProps {
  workspaceId: string
  docId: string
  thread: CommentWithAuthor
  replies: CommentWithAuthor[]
  currentUserId?: string
  members: AssignableMember[]
  teams: WorkspaceTeam[]
  isActive?: boolean
  onChange: (next: { thread: CommentWithAuthor; replies: CommentWithAuthor[] }) => void
  onDelete: () => void
}

export function InlineCommentCard({
  workspaceId,
  docId,
  thread,
  replies,
  currentUserId,
  members,
  teams,
  isActive,
  onChange,
  onDelete,
}: InlineCommentCardProps) {
  const isResolved = Boolean(thread.comment.resolved_at)
  return (
    <div
      data-comment-card-id={thread.comment.id}
      className={`mb-2 rounded-md border bg-popover shadow-sm transition-shadow ${
        isActive ? 'border-amber-500/70 ring-2 ring-amber-500/20' : 'border-border/60'
      } ${isResolved ? 'opacity-70' : ''}`}
    >
      <CommentRow
        workspaceId={workspaceId}
        docId={docId}
        entry={thread}
        currentUserId={currentUserId}
        members={members}
        teams={teams}
        isThreadRoot
        threadResolved={isResolved}
        replyCount={replies.length}
        onUpdated={(updated) => onChange({ thread: updated, replies })}
        onDelete={onDelete}
        onResolveToggle={async () => {
          const fn = isResolved ? docsCommentService.reopen : docsCommentService.resolve
          const { data, error } = await fn(workspaceId, thread.comment.id)
          if (error || !data) {
            toast.error(error || 'Failed to update')
            return
          }
          onChange({ thread: { ...thread, comment: data }, replies })
        }}
        onReplied={(reply) => onChange({ thread, replies: [...replies, reply] })}
      />
      {replies.map((reply) => (
        <div key={reply.comment.id} className="relative border-t border-border/40 pl-3">
          <span className="absolute bottom-2 left-3 top-2 w-px rounded-full bg-border" aria-hidden="true" />
          <CommentRow
            workspaceId={workspaceId}
            docId={docId}
            entry={reply}
            currentUserId={currentUserId}
            members={members}
            teams={teams}
            onUpdated={(updated) => {
              const nextReplies = replies.map((r) => (r.comment.id === reply.comment.id ? updated : r))
              onChange({ thread, replies: nextReplies })
            }}
            onDelete={async () => {
              const { error } = await docsCommentService.remove(workspaceId, reply.comment.id)
              if (error) {
                toast.error(error)
                return
              }
              onChange({ thread, replies: replies.filter((r) => r.comment.id !== reply.comment.id) })
            }}
          />
        </div>
      ))}
    </div>
  )
}

interface CommentRowProps {
  workspaceId: string
  docId: string
  entry: CommentWithAuthor
  currentUserId?: string
  members: AssignableMember[]
  teams: WorkspaceTeam[]
  isThreadRoot?: boolean
  threadResolved?: boolean
  replyCount?: number
  onUpdated: (updated: CommentWithAuthor) => void
  onDelete: () => void | Promise<void>
  onResolveToggle?: () => void | Promise<void>
  onReplied?: (reply: CommentWithAuthor) => void
}

function CommentRow({
  workspaceId,
  docId,
  entry,
  currentUserId,
  members,
  teams,
  isThreadRoot,
  threadResolved,
  replyCount = 0,
  onUpdated,
  onDelete,
  onResolveToggle,
  onReplied,
}: CommentRowProps) {
  const isOwn = currentUserId === entry.comment.author_id
  const [editing, setEditing] = useState(false)
  const [replying, setReplying] = useState(false)
  const [busy, setBusy] = useState(false)
  const authorName = entry.author.full_name || entry.author.email

  const teamPicks = useMemo(
    () => teams.map((t) => ({ id: t.id, name: t.name, handle: t.handle })),
    [teams],
  )

  const handleEditSave = useCallback(
    async (body: string) => {
      if (!body.trim()) return
      setBusy(true)
      const { data, error } = await docsCommentService.update(workspaceId, entry.comment.id, { body })
      setBusy(false)
      setEditing(false)
      if (error || !data) {
        toast.error(error || 'Failed to update')
        return
      }
      onUpdated({ ...entry, comment: { ...entry.comment, body } })
    },
    [workspaceId, entry, onUpdated],
  )

  const handleReplySave = useCallback(
    async (body: string) => {
      if (!body.trim() || !onReplied) return
      setBusy(true)
      const { data, error } = await docsCommentService.create(workspaceId, {
        entity_type: 'doc',
        entity_id: docId,
        body: body.trim(),
        parent_id: entry.comment.id,
      })
      setBusy(false)
      setReplying(false)
      if (error || !data) {
        toast.error(error || 'Failed to reply')
        return
      }
      onReplied(data)
    },
    [workspaceId, docId, entry.comment.id, onReplied],
  )

  const toggleReaction = useCallback(
    async (emoji: string) => {
      const { data, error } = await docsCommentService.toggleReaction(workspaceId, entry.comment.id, emoji)
      if (error || !data) {
        toast.error(error || 'Failed to react')
        return
      }
      onUpdated({ ...entry, reactions: data })
    },
    [workspaceId, entry, onUpdated],
  )

  return (
    <div className="group/comment px-3 py-2.5">
      <div className="flex items-start gap-2">
        <UserAvatar
          name={authorName}
          avatarUrl={entry.author.avatar_url}
          avatarStyle={entry.author.avatar_style}
          avatarSeed={entry.author.avatar_seed}
          avatarBackgroundMode={entry.author.avatar_background_mode}
          avatarBackgroundColor={entry.author.avatar_background_color}
          className="h-6 w-6 shrink-0 mt-0.5 text-[9px]"
        />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
            <span className="truncate text-xs font-semibold">{authorName}</span>
            <span className="shrink-0 text-[11px] text-muted-foreground">{relTime(entry.comment.created_at)}</span>
            {/* Hover-revealed action row */}
            <div className="ml-auto flex items-center gap-0.5 opacity-0 transition-opacity group-hover/comment:opacity-100">
              <Popover>
                <PopoverTrigger asChild>
                  <button
                    type="button"
                    className="flex h-6 w-6 items-center justify-center rounded text-foreground/60 hover:bg-accent hover:text-foreground"
                    aria-label="React"
                  >
                    <SmilePlusIcon className="h-3.5 w-3.5" />
                  </button>
                </PopoverTrigger>
                <PopoverContent side="top" align="end" className="w-auto p-1">
                  <div className="flex items-center gap-0.5">
                    {REACTION_EMOJIS.map((r) => (
                      <button
                        key={r.emoji}
                        type="button"
                        className="flex h-7 w-7 items-center justify-center rounded text-base hover:bg-accent"
                        onClick={() => void toggleReaction(r.emoji)}
                        aria-label={r.label}
                      >
                        {r.emoji}
                      </button>
                    ))}
                  </div>
                </PopoverContent>
              </Popover>
              {isThreadRoot && (
                <QuickTooltip label="Reply">
                  <button
                    type="button"
                    className="flex h-6 w-6 items-center justify-center rounded text-foreground/60 hover:bg-accent hover:text-foreground"
                    onClick={() => setReplying(true)}
                    aria-label="Reply"
                  >
                    <ArrowTurnBackwardIcon className="h-3.5 w-3.5 -scale-y-100" />
                  </button>
                </QuickTooltip>
              )}
              {isThreadRoot && onResolveToggle && (
                <QuickTooltip label={threadResolved ? 'Reopen' : 'Resolve'}>
                  <button
                    type="button"
                    className="flex h-6 w-6 items-center justify-center rounded text-foreground/60 hover:bg-accent hover:text-foreground"
                    onClick={() => void onResolveToggle()}
                    aria-label={threadResolved ? 'Reopen' : 'Resolve'}
                  >
                    {threadResolved ? (
                      <ArrowReloadHorizontalIcon className="h-3.5 w-3.5" />
                    ) : (
                      <CheckmarkCircle02Icon className="h-3.5 w-3.5" />
                    )}
                  </button>
                </QuickTooltip>
              )}
              {isOwn && (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      className="flex h-6 w-6 items-center justify-center rounded text-foreground/60 hover:bg-accent hover:text-foreground"
                      aria-label="More"
                    >
                      <MoreHorizontalIcon className="h-3.5 w-3.5" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-36">
                    <DropdownMenuItem onSelect={() => setEditing(true)}>
                      <PencilEdit01Icon className="h-3.5 w-3.5" />
                      Edit
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      onSelect={() => void onDelete()}
                      className="text-destructive focus:text-destructive"
                    >
                      <Delete01Icon className="h-3.5 w-3.5" />
                      Delete
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              )}
            </div>
          </div>

          {/* Body */}
          {editing ? (
            <div className="mt-1.5">
              <CommentEditor
                onSubmit={handleEditSave}
                onCancel={() => setEditing(false)}
                loading={busy}
                placeholder="Edit comment…"
                variant="primary"
                teams={teamPicks}
                members={members}
                initialContent={entry.comment.body}
                autoFocus
              />
            </div>
          ) : (
            <CommentBody body={entry.comment.body} members={members} teams={teams} className="mt-1" />
          )}

          {/* Reactions strip */}
          {(entry.reactions?.length ?? 0) > 0 && (
            <div className="mt-1.5 flex flex-wrap gap-1">
              {entry.reactions!.map((r) => {
                const mine = currentUserId ? r.user_ids.includes(currentUserId) : false
                return (
                  <button
                    key={r.emoji}
                    type="button"
                    onClick={() => void toggleReaction(r.emoji)}
                    className={`inline-flex items-center gap-1 rounded-full border px-1.5 py-0.5 text-[11px] ${
                      mine
                        ? 'border-primary/40 bg-primary/10 text-foreground'
                        : 'border-border/60 bg-muted/40 text-muted-foreground hover:bg-muted'
                    }`}
                  >
                    <span>{r.emoji}</span>
                    <span>{r.count}</span>
                  </button>
                )
              })}
            </div>
          )}

          {/* Reply count helper for collapsed threads */}
          {isThreadRoot && replyCount > 0 && !replying && (
            <div className="mt-1.5 text-[11px] text-muted-foreground">
              {replyCount} {replyCount === 1 ? 'reply' : 'replies'}
            </div>
          )}

          {/* Inline reply composer */}
          {replying && (
            <div className="mt-2">
              <CommentEditor
                onSubmit={handleReplySave}
                onCancel={() => setReplying(false)}
                loading={busy}
                placeholder="Reply…"
                variant="reply"
                teams={teamPicks}
                members={members}
                autoFocus
              />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

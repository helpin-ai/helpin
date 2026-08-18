import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { Editor } from '@tiptap/core'
import { CommentEditor } from '@/components/pm/CommentEditor'
import { docsCommentService } from '@/lib/services/docsCommentService'
import { InlineCommentCard } from '@/components/docs/InlineCommentCard'
import { Cancel01Icon } from '@/lib/icons'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'
import type { CommentWithAuthor } from '@/lib/pmTypes'
import type { AssignableMember, WorkspaceTeam } from '@/lib/types'
import type { DocsCommentAnchor } from '@/components/docs/DocsEditor'

interface CommentSideGutterProps {
  editor: Editor | null
  workspaceId: string
  docId: string
  comments: CommentWithAuthor[]
  currentUserId?: string
  members: AssignableMember[]
  teams: WorkspaceTeam[]
  composingAnchor: DocsCommentAnchor | null
  onCommentsChange: (comments: CommentWithAuthor[]) => void
  onComposingAnchorConsumed: () => void
  activeCommentId?: string | null
}

interface PositionedThread {
  threadKey: string
  blockId: string | null
  entry: CommentWithAuthor
  top: number
}

const CARD_GAP = 12
const CARD_DEFAULT_HEIGHT = 96

function normalizeThreads(comments: CommentWithAuthor[]): CommentWithAuthor[] {
  const roots: CommentWithAuthor[] = []
  const repliesByParent = new Map<string, CommentWithAuthor[]>()

  comments.forEach((entry) => {
    const parentID = entry.comment.parent_id
    if (!parentID) {
      roots.push(entry)
      return
    }
    repliesByParent.set(parentID, [...(repliesByParent.get(parentID) ?? []), entry])
  })

  return roots.map((root) => {
    const replies = [...(root.replies ?? [])]
    const seen = new Set(replies.map((reply) => reply.comment.id))
    ;(repliesByParent.get(root.comment.id) ?? []).forEach((reply) => {
      if (seen.has(reply.comment.id)) return
      seen.add(reply.comment.id)
      replies.push(reply)
    })
    if (replies.length === 0) return root
    return { ...root, replies, reply_count: replies.length }
  })
}

function withThreadReplies(thread: CommentWithAuthor, replies: CommentWithAuthor[]): CommentWithAuthor {
  return {
    ...thread,
    replies,
    reply_count: replies.length,
  }
}

export function CommentSideGutter({
  editor,
  workspaceId,
  docId,
  comments,
  currentUserId,
  members,
  teams,
  composingAnchor,
  onCommentsChange,
  onComposingAnchorConsumed,
  activeCommentId,
}: CommentSideGutterProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const [positioned, setPositioned] = useState<PositionedThread[]>([])
  const [orphans, setOrphans] = useState<CommentWithAuthor[]>([])
  const [composeTop, setComposeTop] = useState<number | null>(null)
  const cardHeightsRef = useRef<Map<string, number>>(new Map())

  const threads = useMemo(() => normalizeThreads(comments), [comments])

  const findBlockEl = useCallback(
    (entry: CommentWithAuthor): HTMLElement | null => {
      if (!editor || editor.isDestroyed) return null
      const blockId = entry.comment.block_id
      if (!blockId) return null
      return editor.view.dom.querySelector<HTMLElement>(`[data-block-id="${CSS.escape(blockId)}"]`)
    },
    [editor],
  )

  const recompute = useCallback(() => {
    if (!editor || editor.isDestroyed) return
    const wrapper = containerRef.current?.parentElement as HTMLElement | null
    if (!wrapper) return
    const wrapperRect = wrapper.getBoundingClientRect()
    const next: PositionedThread[] = []
    const newOrphans: CommentWithAuthor[] = []
    threads.forEach((entry) => {
      const block = findBlockEl(entry)
      if (!block) {
        newOrphans.push(entry)
        return
      }
      const rect = block.getBoundingClientRect()
      const top = rect.top - wrapperRect.top + (wrapper.scrollTop ?? 0)
      next.push({
        threadKey: entry.comment.id,
        blockId: entry.comment.block_id ?? null,
        entry,
        top,
      })
    })
    next.sort((a, b) => a.top - b.top)
    let lastBottom = -Infinity
    next.forEach((p) => {
      if (p.top < lastBottom + CARD_GAP) p.top = lastBottom + CARD_GAP
      const h = cardHeightsRef.current.get(p.threadKey) ?? CARD_DEFAULT_HEIGHT
      lastBottom = p.top + h
    })
    setPositioned(next)
    setOrphans(newOrphans)

    if (composingAnchor?.block_id) {
      const block = editor.view.dom.querySelector<HTMLElement>(
        `[data-block-id="${CSS.escape(composingAnchor.block_id)}"]`,
      )
      if (block) {
        const rect = block.getBoundingClientRect()
        let top = rect.top - wrapperRect.top + (wrapper.scrollTop ?? 0)
        if (top < lastBottom + CARD_GAP) top = lastBottom + CARD_GAP
        setComposeTop(top)
      } else {
        setComposeTop(0)
      }
    } else {
      setComposeTop(null)
    }
  }, [editor, threads, findBlockEl, composingAnchor])

  useEffect(() => {
    if (!editor || editor.isDestroyed) return
    let pending = false
    const schedule = () => {
      if (pending) return
      pending = true
      requestAnimationFrame(() => {
        pending = false
        recompute()
      })
    }
    schedule()
    const wrapper = editor.view.dom.closest('.docs-editor-wrapper') as HTMLElement | null
    const targets: (HTMLElement | Window)[] = [window]
    if (wrapper) targets.push(wrapper)
    targets.forEach((t) => t.addEventListener('scroll', schedule, { passive: true }))
    window.addEventListener('resize', schedule)
    const onUpdate = () => schedule()
    editor.on('update', onUpdate)
    return () => {
      targets.forEach((t) => t.removeEventListener('scroll', schedule))
      window.removeEventListener('resize', schedule)
      editor.off('update', onUpdate)
    }
  }, [editor, recompute])

  const measureCard = useCallback((key: string, el: HTMLDivElement | null) => {
    if (!el) {
      cardHeightsRef.current.delete(key)
      return
    }
    cardHeightsRef.current.set(key, el.getBoundingClientRect().height)
  }, [])

  const submitComposer = useCallback(
    async (body: string) => {
      if (!body.trim() || !composingAnchor) return
      const { data, error } = await docsCommentService.create(workspaceId, {
        entity_type: 'doc',
        entity_id: docId,
        body: body.trim(),
        block_id: composingAnchor.block_id,
        range: composingAnchor.range as Record<string, unknown> | undefined,
        anchor_text: composingAnchor.anchor_text,
      })
      if (error || !data) {
        toast.error(error || 'Failed to add comment')
        return
      }
      onCommentsChange([...comments, data])
      onComposingAnchorConsumed()
    },
    [workspaceId, docId, composingAnchor, comments, onCommentsChange, onComposingAnchorConsumed],
  )

  const teamPicks = useMemo(
    () => teams.map((t) => ({ id: t.id, name: t.name, handle: t.handle })),
    [teams],
  )

  const handleCardChange = useCallback(
    (next: { thread: CommentWithAuthor; replies: CommentWithAuthor[] }) => {
      const nextThread = withThreadReplies(next.thread, next.replies)
      let replaced = false
      const nextComments: CommentWithAuthor[] = []

      comments.forEach((entry) => {
        if (entry.comment.id === next.thread.comment.id) {
          nextComments.push(nextThread)
          replaced = true
          return
        }
        if (entry.comment.parent_id === next.thread.comment.id) return
        nextComments.push(entry)
      })

      if (!replaced) nextComments.push(nextThread)
      onCommentsChange(nextComments)
    },
    [comments, onCommentsChange],
  )

  const handleCardDelete = useCallback(
    async (entry: CommentWithAuthor) => {
      const { error } = await docsCommentService.remove(workspaceId, entry.comment.id)
      if (error) {
        toast.error(error)
        return
      }
      onCommentsChange(
        comments.filter(
          (c) => c.comment.id !== entry.comment.id && c.comment.parent_id !== entry.comment.id,
        ),
      )
    },
    [workspaceId, comments, onCommentsChange],
  )

  if (!editor) return null

  return (
    <div
      ref={containerRef}
      className="docs-comment-side-gutter pointer-events-none absolute top-0 z-10 hidden w-72 xl:block"
      aria-label="Comments"
    >
      {orphans.length > 0 && (
        <div className="pointer-events-auto mb-3 rounded-md border border-amber-300/40 bg-amber-50/40 p-1 dark:border-amber-700/30 dark:bg-amber-950/20">
          <div className="px-2 py-1 text-[11px] font-medium uppercase tracking-wide text-amber-700 dark:text-amber-300">
            Unanchored ({orphans.length})
          </div>
          {orphans.map((entry) => {
            const replies = entry.replies ?? []
            return (
              <InlineCommentCard
                key={`orphan-${entry.comment.id}`}
                workspaceId={workspaceId}
                docId={docId}
                thread={entry}
                replies={replies}
                currentUserId={currentUserId}
                members={members}
                teams={teams}
                isActive={activeCommentId === entry.comment.id}
                onChange={handleCardChange}
                onDelete={() => void handleCardDelete(entry)}
              />
            )
          })}
        </div>
      )}

      {positioned.map((p) => {
        const replies = p.entry.replies ?? []
        return (
          <div
            key={p.threadKey}
            data-comment-thread-card={p.threadKey}
            ref={(el) => measureCard(p.threadKey, el)}
            className="pointer-events-auto absolute left-0 right-0"
            style={{ top: p.top }}
          >
            <InlineCommentCard
              workspaceId={workspaceId}
              docId={docId}
              thread={p.entry}
              replies={replies}
              currentUserId={currentUserId}
              members={members}
              teams={teams}
              isActive={activeCommentId === p.entry.comment.id}
              onChange={handleCardChange}
              onDelete={() => void handleCardDelete(p.entry)}
            />
          </div>
        )
      })}

      {composingAnchor && composeTop != null && (
        <div
          className="pointer-events-auto absolute left-0 right-0 rounded-md border border-border/60 bg-popover p-2 shadow-sm"
          style={{ top: composeTop }}
        >
          {composingAnchor.anchor_text && (
            <div className="mb-1.5 flex items-start gap-2 rounded-sm border-l-2 border-amber-500/70 bg-amber-50/40 px-2 py-1 text-[11px] italic text-muted-foreground dark:bg-amber-950/20">
              <span className="flex-1 truncate">“{composingAnchor.anchor_text}”</span>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-5 w-5 shrink-0 p-0"
                onClick={onComposingAnchorConsumed}
                aria-label="Cancel"
              >
                <Cancel01Icon className="h-3 w-3" />
              </Button>
            </div>
          )}
          <CommentEditor
            onSubmit={submitComposer}
            onCancel={onComposingAnchorConsumed}
            placeholder="Add a comment…"
            variant="primary"
            teams={teamPicks}
            members={members}
            autoFocus
          />
        </div>
      )}
    </div>
  )
}

import { useState } from 'react'
import Markdown from 'react-markdown'
import { ArrowUpRight01Icon } from '@/lib/icons'
import type { SupportGapEvidence } from '@/lib/supportCoverageTypes'
import { timeAgo } from '@/lib/utils'
import { cn } from '@/lib/utils'
import { isOurSenderRole } from './evidenceGrouping'

interface EvidenceConversationCardProps {
  conversationId: string | null
  wsSlug: string
  evidenceTypeLabel: string
  evidenceItems: SupportGapEvidence[]
}

const MAX_VISIBLE = 3

export function EvidenceConversationCard({
  conversationId,
  wsSlug,
  evidenceTypeLabel,
  evidenceItems,
}: EvidenceConversationCardProps) {
  const [expanded, setExpanded] = useState(false)

  // Sort oldest → newest for natural thread reading. Backend sends DESC.
  const ordered = [...evidenceItems].sort((a, b) =>
    a.created_at.localeCompare(b.created_at),
  )

  const latest = ordered[ordered.length - 1]
  const messageCount = ordered.length
  const visible = expanded ? ordered : ordered.slice(-MAX_VISIBLE)
  const hiddenCount = ordered.length - visible.length

  return (
    <div className="rounded-md border border-border/40 bg-muted/30 p-2.5 text-xs">
      <div className="mb-2 flex items-center justify-between gap-2">
        <p className="font-medium text-muted-foreground">
          {evidenceTypeLabel}
          {messageCount > 1 && (
            <span className="ml-2 font-normal">
              {messageCount} messages
            </span>
          )}
          {latest && (
            <span className="ml-2 font-normal">{timeAgo(latest.created_at)}</span>
          )}
        </p>
        {conversationId && (
          <a
            href={`/w/${wsSlug}/support/${conversationId}`}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-0.5 text-primary hover:underline"
          >
            Conversation
            <ArrowUpRight01Icon className="h-2.5 w-2.5" />
          </a>
        )}
      </div>

      {hiddenCount > 0 && (
        <button
          type="button"
          onClick={() => setExpanded(true)}
          className="mb-2 text-[11px] text-muted-foreground hover:text-foreground hover:underline"
        >
          Show {hiddenCount} earlier message{hiddenCount === 1 ? '' : 's'}
        </button>
      )}

      <div className="flex flex-col gap-1.5">
        {visible.map((ev) => {
          const ours = isOurSenderRole(ev.sender_role)
          return (
            <div
              key={ev.id}
              className={cn('flex', ours ? 'justify-end' : 'justify-start')}
            >
              <div
                className={cn(
                  'max-w-[80%] rounded-md px-2.5 py-1.5',
                  ours
                    ? 'bg-primary/10 text-foreground'
                    : 'bg-background border border-border/40',
                )}
              >
                {ev.excerpt && (
                  <div className="prose-chat text-sm leading-relaxed">
                    <Markdown>{ev.excerpt}</Markdown>
                  </div>
                )}
                <p
                  className={cn(
                    'mt-0.5 text-[10px] text-muted-foreground/80',
                    ours ? 'text-right' : 'text-left',
                  )}
                >
                  {timeAgo(ev.created_at)}
                </p>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

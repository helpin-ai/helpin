import { Check, UserMinus } from 'lucide-react'
import { useConversationAssignees, useSupportTeammatePresence } from '@helpin-ai/support-core'
import { TeamMemberAvatar } from '@mobile/ui/team-member-avatar'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import {
  sortTeammatesByPresence,
  teammatePresenceByUserId,
  teammatePresenceDotClass,
  teammatePresenceLabel,
} from './teammate-presence'

export interface AssignListProps {
  workspaceId: string
  conversationId: string
  /** `conversation.assigned_user_id` — `null`/`undefined` means unassigned. */
  currentUserId?: string | null
  onSelect: (userId: string | null) => void
}

/**
 * Inline (non-sheet) teammate picker rendered inside ContextSheet when the
 * Assign action is expanded. Only members with a linked `user_id` are
 * assignable — mirrors the desktop web client's filter
 * (frontend/src/components/support/ConversationDetailSidebar.tsx:293),
 * since the assign-user endpoint takes a user id, not a member id.
 */
export function AssignList({ workspaceId, conversationId, currentUserId, onSelect }: AssignListProps) {
  const { data: assignees, isLoading, isError, refetch } = useConversationAssignees(workspaceId, conversationId)
  const presenceQuery = useSupportTeammatePresence(workspaceId)
  const presence = presenceQuery.data ?? []
  const presenceByUserId = teammatePresenceByUserId(presence)
  const assignableUsers = sortTeammatesByPresence(
    (assignees ?? []).filter((member) => !!member.user_id),
    presence,
  )

  return (
    <div className="flex flex-col py-1">
      <Pressable
        haptic="selection"
        aria-pressed={!currentUserId}
        onPress={() => onSelect(null)}
        className="flex h-auto min-h-0 w-full items-center gap-3 px-4 py-2 text-left"
      >
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
          <UserMinus className="h-4 w-4" />
        </span>
        <span className="flex-1 text-body">Unassign</span>
        {!currentUserId && <Check className="h-4 w-4 shrink-0 text-primary" />}
      </Pressable>

      {isLoading && (
        <div className="flex items-center justify-center px-4 py-3">
          <Spinner size={16} />
        </div>
      )}

      {isError && (
        <div className="flex items-center justify-between gap-3 px-4 py-3">
          <span className="text-footnote text-muted-foreground">Couldn't load teammates</span>
          <Pressable
            haptic="selection"
            onPress={() => void refetch()}
            className="h-auto min-h-0 w-auto min-w-0 text-footnote font-medium text-primary"
          >
            Retry
          </Pressable>
        </div>
      )}

      {assignableUsers.map((member) => {
        const memberId = member.user_id ?? member.id
        const selected = memberId === currentUserId
        const memberPresence = member.user_id ? presenceByUserId.get(member.user_id) : undefined
        const presenceLabel = teammatePresenceLabel(memberPresence?.status)
        return (
          <Pressable
            key={member.id}
            haptic="selection"
            aria-label={`${member.display_name}, ${presenceLabel}`}
            aria-pressed={selected}
            onPress={() => onSelect(memberId)}
            className="flex h-auto min-h-0 w-full items-center gap-3 px-4 py-2 text-left"
          >
            <span className="relative shrink-0">
              <TeamMemberAvatar name={member.display_name} member={member} size={36} />
              <span aria-hidden className={`absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full border-2 border-background ${teammatePresenceDotClass(memberPresence?.status)}`} />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-body">{member.display_name}</span>
              <span className="block text-caption text-muted-foreground">{presenceLabel}</span>
            </span>
            {selected && <Check className="h-4 w-4 shrink-0 text-primary" />}
          </Pressable>
        )
      })}
    </div>
  )
}

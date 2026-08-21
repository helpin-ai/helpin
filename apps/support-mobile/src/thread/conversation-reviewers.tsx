import type { AssignableMember } from '@helpin-ai/support-core'
import { Avatar } from '@mobile/ui/avatar'

interface ConversationReviewersProps {
  viewerIds: string[]
  members: AssignableMember[]
  currentUserId?: string
}

export function conversationReviewerLabel(names: string[]): string {
  if (names.length === 1) return `${names[0]} is also viewing`
  if (names.length === 2) return `${names[0]} and ${names[1]} are also viewing`
  return `${names[0]} and ${names.length - 1} others are also viewing`
}

/** Teammates currently reviewing this conversation, excluding the local user. */
export function ConversationReviewers({ viewerIds, members, currentUserId }: ConversationReviewersProps) {
  const memberByUserId = new Map(
    members.filter((member) => member.user_id).map((member) => [member.user_id!, member]),
  )
  const reviewers = [...new Set(viewerIds)]
    .filter((id) => id !== currentUserId)
    .map((id) => memberByUserId.get(id) ?? {
      id,
      user_id: id,
      display_name: 'Teammate',
      email: '',
      role: '',
      avatar_url: undefined,
    })

  if (reviewers.length === 0) return null
  const names = reviewers.map((reviewer) => reviewer.display_name || reviewer.email || 'Teammate')

  return (
    <span
      aria-label={conversationReviewerLabel(names)}
      className="mt-2 flex max-w-full items-center gap-2 text-[11px] text-muted-foreground"
    >
      <span className="flex shrink-0 flex-row-reverse justify-end pl-1">
        {reviewers.slice(0, 3).reverse().map((reviewer, index) => (
          <span key={reviewer.user_id ?? reviewer.id} className={index === 0 ? '' : '-mr-1.5'}>
            <Avatar name={reviewer.display_name || reviewer.email || 'Teammate'} src={reviewer.avatar_url} size={22} className="ring-2 ring-background" />
          </span>
        ))}
        {reviewers.length > 3 && (
          <span className="-mr-1.5 flex h-[22px] min-w-[22px] items-center justify-center rounded-full bg-muted px-1 text-[9px] font-semibold ring-2 ring-background">
            +{reviewers.length - 3}
          </span>
        )}
      </span>
      <span className="truncate">{conversationReviewerLabel(names)}</span>
    </span>
  )
}

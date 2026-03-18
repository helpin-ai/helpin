import { MentionText } from '@/components/pm/MentionText'
import { RichTextMentionContent } from '@/components/pm/RichTextMentionContent'
import type { AssignableMember, WorkspaceTeam } from '@/lib/types'

interface CommentBodyProps {
  body: string
  members?: AssignableMember[]
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[]
  className?: string
}

/**
 * Renders a comment body — detects HTML content (from rich editor with images)
 * vs plain text (legacy comments) and renders accordingly.
 */
export function CommentBody({ body, members = [], teams = [], className }: CommentBodyProps) {
  const isHtml = body.startsWith('<') && (body.includes('<p>') || body.includes('<img'))

  if (isHtml) {
    return (
      <RichTextMentionContent
        html={body}
        members={members}
        teams={teams}
        className={`prose prose-sm dark:prose-invert max-w-none text-sm [&_img]:rounded-md [&_img]:max-w-full ${className ?? ''}`}
      />
    )
  }

  return (
    <p className={`text-sm ${className ?? ''}`}>
      <MentionText text={body} members={members} teams={teams} />
    </p>
  )
}

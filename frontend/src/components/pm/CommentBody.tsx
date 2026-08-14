import { MentionText } from '@/components/pm/MentionText'
import { RichTextMentionContent } from '@/components/pm/RichTextMentionContent'
import type { AssignableMember, WorkspaceTeam } from '@/lib/types'

interface CommentBodyProps {
  body: string
  members?: AssignableMember[]
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[]
  className?: string
  variant?: 'default' | 'pm'
}

/**
 * Renders a comment body — detects HTML content (from rich editor with images)
 * vs plain text (legacy comments) and renders accordingly.
 */
export function CommentBody({ body, members = [], teams = [], className, variant = 'default' }: CommentBodyProps) {
  const isHtml = body.startsWith('<') && (body.includes('<p>') || body.includes('<img'))

  if (isHtml) {
    return (
      <RichTextMentionContent
        html={body}
        members={members}
        teams={teams}
        variant={variant}
        className={`prose prose-sm dark:prose-invert max-w-none ${variant === 'pm' ? '' : 'text-[13px]'} [&_img]:rounded-md [&_img]:max-w-full ${className ?? ''}`}
      />
    )
  }

  return (
    <p className={`${variant === 'pm' ? 'pm-rich-text' : 'text-[13px]'} ${className ?? ''}`}>
      <MentionText text={body} members={members} teams={teams} />
    </p>
  )
}

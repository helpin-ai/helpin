import { MentionText } from '@/components/pm/MentionText'
import type { AssignableMember } from '@/lib/types'

interface CommentBodyProps {
  body: string
  members?: AssignableMember[]
  className?: string
}

/**
 * Renders a comment body — detects HTML content (from rich editor with images)
 * vs plain text (legacy comments) and renders accordingly.
 */
export function CommentBody({ body, members = [], className }: CommentBodyProps) {
  const isHtml = body.startsWith('<') && (body.includes('<p>') || body.includes('<img'))

  if (isHtml) {
    return (
      <div
        className={`prose prose-sm dark:prose-invert max-w-none text-sm [&_img]:rounded-md [&_img]:max-w-full ${className ?? ''}`}
        dangerouslySetInnerHTML={{ __html: body }}
      />
    )
  }

  return (
    <p className={`text-sm ${className ?? ''}`}>
      <MentionText text={body} members={members} />
    </p>
  )
}

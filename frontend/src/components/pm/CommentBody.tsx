import { useMemo } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import DOMPurify from 'dompurify'
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

interface MarkdownNode {
  type: string
  tagName?: string
  value?: string
  properties?: Record<string, unknown>
  children?: MarkdownNode[]
}

// Let the Markdown parser handle structure and links; resolve mentions only in
// prose, so code stays literal and autolinks cannot become nested anchors.
function rehypeCommentMentions() {
  return function visit(node: MarkdownNode) {
    if (['a', 'code', 'pre'].includes(node.tagName ?? '')) return
    node.children = node.children?.map(child => {
      if (child.type === 'text' && child.value?.includes('@')) {
        return { type: 'element', tagName: 'span', properties: {}, children: [child] }
      }
      visit(child)
      return child
    })
  }
}

/** Comments may contain rich-editor HTML or Markdown from agents/integrations. */
export function CommentBody({ body, members = [], teams = [], className, variant = 'default' }: CommentBodyProps) {
  const isHtml = /^\s*<(?:p|div|h[1-6]|ul|ol|li|blockquote|pre|table|img)\b/i.test(body)
  const safeHtml = useMemo(() => isHtml ? DOMPurify.sanitize(body) : '', [body, isHtml])

  if (isHtml) {
    return (
      <RichTextMentionContent
        html={safeHtml}
        members={members}
        teams={teams}
        variant={variant}
        className={`prose prose-sm dark:prose-invert max-w-none ${variant === 'pm' ? '' : 'text-[13px]'} [&_img]:rounded-md [&_img]:max-w-full ${className ?? ''}`}
      />
    )
  }

  return (
    <div className={`tiptap prose prose-sm dark:prose-invert max-w-none break-words [&_p]:whitespace-pre-line [&_li]:whitespace-pre-line [&_img]:rounded-md [&_img]:max-w-full [&_.task-list-item]:list-none ${variant === 'pm' ? 'pm-rich-text' : 'text-[13px]'} ${className ?? ''}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeCommentMentions]}
        components={{
          span: ({ children }) => <MentionText text={String(children)} members={members} teams={teams} />,
          a: ({ children, href }) => <a href={href} target="_blank" rel="noopener noreferrer">{children}</a>,
          table: ({ children }) => <div className="overflow-x-auto"><table>{children}</table></div>,
        }}
      >
        {body}
      </ReactMarkdown>
    </div>
  )
}

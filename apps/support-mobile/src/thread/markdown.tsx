import type { ComponentPropsWithoutRef } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { cn } from '@mobile/lib/cn'

const REMARK_PLUGINS = [remarkGfm]

const markdownComponents = {
  a: ({ href, children }: ComponentPropsWithoutRef<'a'>) => (
    // Links open in the system browser (Tauri intercepts _blank navigations).
    <a href={href} target="_blank" rel="noopener noreferrer" className="[overflow-wrap:anywhere] underline">
      {children}
    </a>
  ),
  table: ({ children }: ComponentPropsWithoutRef<'table'>) => (
    <div className="chat-markdown-table-wrap">
      <table>{children}</table>
    </div>
  ),
  img: ({ className, loading, ...props }: ComponentPropsWithoutRef<'img'>) => (
    // eslint-disable-next-line jsx-a11y/alt-text -- alt comes through ...props
    <img {...props} loading={loading ?? 'lazy'} className={cn('max-h-60 max-w-full rounded-lg object-cover', className)} />
  ),
}

/**
 * GitHub-flavoured Markdown renderer for message bodies — mirrors the web
 * thread (react-markdown + remark-gfm + `.prose-chat` styling) so bold, lists,
 * links, code, and tables render the same on mobile instead of as raw text.
 */
export function Markdown({ children, className }: { children: string; className?: string }) {
  return (
    <div className={cn('prose-chat', className)}>
      <ReactMarkdown remarkPlugins={REMARK_PLUGINS} components={markdownComponents}>
        {children}
      </ReactMarkdown>
    </div>
  )
}

import { useState } from 'react'
import { Check, Copy, ExternalLink, Sparkles } from 'lucide-react'
import { useDocsContext } from '@/contexts/DocsContext'
import { openAskAI } from '@/lib/askAI'
import { htmlToMarkdown } from '@/lib/htmlToMarkdown'

const pill = 'inline-flex items-center gap-1.5 rounded-full border border-border px-3 py-1 text-xs font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground'

/**
 * ArticleActions offers ways to use an article with AI: ask the help
 * assistant about it, copy it as Markdown, or open it in ChatGPT or Claude.
 */
export function ArticleActions({ articleId, title }: { articleId: string; title: string }) {
  const { config } = useDocsContext()
  const [copied, setCopied] = useState(false)
  // The page address is read on click: articles also render on the server.
  const prompt = () => encodeURIComponent(`Read ${window.location.href} so I can ask questions about it.`)

  const copyPage = async () => {
    const content = document.querySelector('.hc-prose')
    if (!content) return
    try {
      await navigator.clipboard.writeText(`# ${title}\n\n${htmlToMarkdown(content, window.location.href)}`)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      setCopied(false)
    }
  }

  return (
    <div className="mb-6 flex flex-wrap items-center gap-2">
      {config.ai_assistant_enabled && (
        <button type="button" onClick={() => openAskAI({ pageArticleId: articleId, pageTitle: title })} className={pill}>
          <Sparkles size={13} style={{ color: config.brand_color || undefined }} aria-hidden="true" />
          Ask AI about this page
        </button>
      )}
      <button type="button" onClick={() => void copyPage()} className={pill}>
        {copied ? <Check size={13} aria-hidden="true" /> : <Copy size={13} aria-hidden="true" />}
        {copied ? 'Copied' : 'Copy page'}
      </button>
      <a href="https://chatgpt.com/" target="_blank" rel="noopener noreferrer" className={pill} onClick={(event) => { event.currentTarget.href = `https://chatgpt.com/?hints=search&prompt=${prompt()}` }}>
        Open in ChatGPT <ExternalLink size={12} aria-hidden="true" />
      </a>
      <a href="https://claude.ai/new" target="_blank" rel="noopener noreferrer" className={pill} onClick={(event) => { event.currentTarget.href = `https://claude.ai/new?q=${prompt()}` }}>
        Open in Claude <ExternalLink size={12} aria-hidden="true" />
      </a>
    </div>
  )
}

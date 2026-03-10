import { useEffect, useRef, useCallback } from 'react'

interface ArticleContentProps {
  html?: string
}

export function ArticleContent({ html }: ArticleContentProps) {
  const contentRef = useRef<HTMLDivElement>(null)

  const enhanceContent = useCallback(() => {
    const container = contentRef.current
    if (!container) return

    // Ensure all h2/h3 headings have id attributes for TOC linking
    container.querySelectorAll('h2, h3').forEach((heading) => {
      if (!heading.id) {
        heading.id = (heading.textContent || '')
          .toLowerCase()
          .replace(/[^\w\s-]/g, '')
          .replace(/\s+/g, '-')
          .replace(/-+/g, '-')
          .trim()
      }
    })

    // Attach copy buttons to <pre> code blocks
    container.querySelectorAll('pre').forEach((pre) => {
      if (pre.querySelector('.hc-copy-btn')) return

      pre.style.position = 'relative'

      const btn = document.createElement('button')
      btn.className = 'hc-copy-btn'
      btn.textContent = 'Copy'
      btn.setAttribute('aria-label', 'Copy code')

      btn.addEventListener('click', () => {
        const code = pre.querySelector('code')?.textContent ?? pre.textContent ?? ''
        navigator.clipboard.writeText(code).then(() => {
          btn.textContent = 'Copied!'
          setTimeout(() => {
            btn.textContent = 'Copy'
          }, 2000)
        })
      })

      pre.appendChild(btn)
    })
  }, [])

  useEffect(() => {
    enhanceContent()
  }, [html, enhanceContent])

  if (!html) {
    return (
      <div className="rounded-lg border border-border/70 p-8 text-center text-[13px] text-muted-foreground">
        No content available for this article.
      </div>
    )
  }

  return (
    <div
      ref={contentRef}
      className="hc-prose"
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}

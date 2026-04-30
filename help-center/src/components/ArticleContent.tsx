import { useEffect, useRef, useCallback } from 'react'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefixBasepath } from '@/lib/pathUtils'
import { slugifyHeading, uniqueSlug } from '@/lib/toc'

interface ArticleContentProps {
  html?: string
}

export function ArticleContent({ html }: ArticleContentProps) {
  const { basepath } = useDocsContext()
  const contentRef = useRef<HTMLDivElement>(null)

  const enhanceContent = useCallback(() => {
    const container = contentRef.current
    if (!container) return

    // Assign id attributes to h2/h3 headings for TOC linking. Track
    // seen slugs in document order and suffix duplicates (-2, -3, …)
    // so repeated headings like "Why this method" (one per option)
    // get unique ids that match what extractTocFromHtml produces.
    const seen = new Map<string, number>()
    container.querySelectorAll('h2, h3').forEach((heading) => {
      const base = heading.id || slugifyHeading(heading.textContent || '')
      heading.id = uniqueSlug(base, seen)
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

  const renderedHtml = rewriteInternalHelpCenterLinks(html, basepath)

  return (
    <div
      ref={contentRef}
      className="hc-prose"
      dangerouslySetInnerHTML={{ __html: renderedHtml }}
    />
  )
}

function rewriteInternalHelpCenterLinks(html: string, basepath: string) {
  if (!basepath) return html

  return html.replace(
    /\s(href)=("([^"]*)"|'([^']*)')/gi,
    (match, attr: string, quoted: string, doubleValue?: string, singleValue?: string) => {
      const value = doubleValue ?? singleValue ?? ''
      if (!shouldPrefixHref(value, basepath)) return match

      const quote = quoted.startsWith('"') ? '"' : "'"
      return ` ${attr}=${quote}${prefixBasepath(basepath, value)}${quote}`
    },
  )
}

function shouldPrefixHref(href: string, basepath: string) {
  if (!href.startsWith('/')) return false
  if (href === basepath || href.startsWith(`${basepath}/`)) return false
  if (href.startsWith('//')) return false

  return (
    href === '/search' ||
    href.startsWith('/search?') ||
    href.startsWith('/c/') ||
    href.startsWith('/articles/') ||
    /^\/[a-z]{2}(?:-[a-z0-9]+)?(?:\/(?:c|articles)\/|\/search(?:\/|\?|$))/i.test(href)
  )
}

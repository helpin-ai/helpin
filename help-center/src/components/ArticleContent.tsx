import { useEffect, useRef, useCallback, useMemo } from 'react'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefixBasepath } from '@/lib/pathUtils'
import { slugifyHeading, uniqueSlug } from '@/lib/toc'

interface ArticleContentProps {
  html?: string
}

export function ArticleContent({ html }: ArticleContentProps) {
  const { basepath } = useDocsContext()
  const contentRef = useRef<HTMLDivElement>(null)
  // Keep scroll-spy updates from replacing image nodes and resetting reader state.
  const renderedContent = useMemo(
    () => ({
      __html: enhanceArticleHtml(rewriteInternalHelpCenterLinks(html ?? '', basepath)),
    }),
    [html, basepath],
  )

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
  }, [renderedContent, enhanceContent])

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
      dangerouslySetInnerHTML={renderedContent}
    />
  )
}

// Runs during SSR too, so phones get the fixed layout on first paint.
function enhanceArticleHtml(html: string) {
  return wrapTables(addImageLoadingHints(html))
}

// Wide tables scroll inside their own box instead of widening the whole page on phones.
function wrapTables(html: string) {
  return html
    .replace(/<table\b/gi, '<div class="hc-table-scroll"><table')
    .replace(/<\/table>/gi, '</table></div>')
}

// Adds loading hints and links each image to its full-size file, so readers on
// phones can tap a scaled-down screenshot to read it. Images already inside a
// link keep that link.
function addImageLoadingHints(html: string) {
  let imageIndex = 0
  let linkDepth = 0
  return html.replace(/<a\b[^>]*>|<\/a\s*>|<img\b([^>]*)>/gi, (tag, attributes?: string) => {
    if (attributes === undefined) {
      linkDepth = tag[1] === '/' ? Math.max(0, linkDepth - 1) : linkDepth + 1
      return tag
    }
    const isFirst = imageIndex === 0
    imageIndex += 1
    let next = attributes
    if (!/\bdecoding\s*=/i.test(next)) next += ' decoding="async"'
    if (!/\bloading\s*=/i.test(next)) {
      next += isFirst ? ' loading="eager"' : ' loading="lazy"'
    }
    if (isFirst && !/\bfetchpriority\s*=/i.test(next)) next += ' fetchpriority="high"'
    const img = `<img${next}>`
    const src = /\bsrc\s*=\s*"([^"]+)"/i.exec(attributes)?.[1]
    if (linkDepth > 0 || !src || !/^(https?:)?\//i.test(src)) return img
    const hasAlt = /\balt\s*=\s*"[^"]*\S[^"]*"/i.test(attributes)
    const label = hasAlt ? '' : ' aria-label="Open full-size image"'
    return `<a class="hc-image-link" href="${src}" target="_blank" rel="noopener noreferrer"${label}>${img}</a>`
  })
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

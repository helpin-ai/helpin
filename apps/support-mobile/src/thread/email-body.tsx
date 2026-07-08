import { useMemo, useState } from 'react'
import DOMPurify from 'dompurify'
import { cn } from '@mobile/lib/cn'
import { Pressable } from '@mobile/ui/pressable'
import { splitQuotedHtml } from './thread-helpers'

export interface EmailBodyProps {
  html: string
  subject?: string
  className?: string
}

/**
 * Unlike the desktop web app's `EmailBodyRenderer` (which renders inside a
 * sandboxed iframe so author `<style>`/`<link>` tags are safe), this renders
 * inline in the app's own DOM — so `<style>`/`<link>` are stripped outright
 * rather than allowed through, to avoid an email leaking global CSS into the
 * rest of the app. Inline `style` attributes are still allowed (DOMPurify
 * sanitizes dangerous values within them) since they're needed for basic
 * formatting.
 */
const EMAIL_HTML_CONFIG = {
  FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'form', 'input', 'button', 'link'],
  ALLOW_DATA_ATTR: false,
}

function sanitize(html: string): string {
  return DOMPurify.sanitize(html, EMAIL_HTML_CONFIG)
}

/**
 * Renders a sanitized email HTML body with quoted reply history collapsed
 * behind a "···" expander. Links open in the system browser (via
 * `window.open`) instead of navigating the app's own webview away.
 */
export function EmailBody({ html, subject, className }: EmailBodyProps) {
  const [expanded, setExpanded] = useState(false)
  const { visible, quoted } = useMemo(() => splitQuotedHtml(html), [html])
  const sanitizedVisible = useMemo(() => sanitize(visible || html), [visible, html])
  const sanitizedQuoted = useMemo(() => (quoted ? sanitize(quoted) : null), [quoted])

  const handleClick = (event: React.MouseEvent<HTMLDivElement>) => {
    const link = (event.target as HTMLElement).closest('a')
    if (!link) return
    const href = link.getAttribute('href')
    if (!href) return
    event.preventDefault()
    window.open(href, '_blank', 'noopener,noreferrer')
  }

  return (
    <div className={cn('selectable min-w-0', className)}>
      {subject && <p className="mb-1 text-body font-semibold text-foreground">{subject}</p>}
      <div
        className="email-body min-w-0 text-body [&_a]:break-words [&_a]:text-primary [&_a]:underline [&_img]:max-w-full [&_img]:rounded-md [&_table]:max-w-full"
        onClick={handleClick}
        // eslint-disable-next-line react/no-danger -- sanitized above via DOMPurify
        dangerouslySetInnerHTML={{ __html: sanitizedVisible }}
      />
      {sanitizedQuoted && (
        <>
          {expanded && (
            <div
              className="email-body mt-2 min-w-0 border-l-2 border-border/60 pl-2 text-body opacity-80 [&_a]:break-words [&_a]:text-primary [&_a]:underline [&_img]:max-w-full"
              onClick={handleClick}
              // eslint-disable-next-line react/no-danger -- sanitized above via DOMPurify
              dangerouslySetInnerHTML={{ __html: sanitizedQuoted }}
            />
          )}
          <Pressable
            aria-label={expanded ? 'Hide quoted content' : 'Show quoted content'}
            haptic="selection"
            onPress={() => setExpanded((current) => !current)}
            className="mt-1 flex h-auto min-h-0 w-auto min-w-0 items-center rounded-full px-1 py-1 text-footnote tracking-wider text-muted-foreground/70"
          >
            •••
          </Pressable>
        </>
      )}
    </div>
  )
}

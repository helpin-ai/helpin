export interface TocItem {
  id: string
  text: string
  level: number
}

function extractPlainText(html: string): string {
  if (typeof DOMParser !== 'undefined') {
    const doc = new DOMParser().parseFromString(html, 'text/html')
    return doc.body.textContent?.trim() ?? ''
  }

  return html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim()
}

export function slugifyHeading(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^\w\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .trim()
}

/**
 * Assigns a unique slug given a base and a mutable map of seen counts.
 * Multiple headings with the same visible text (e.g. "Why this method"
 * repeated once per tabbed section) would otherwise produce colliding
 * ids — each TOC entry would highlight simultaneously and anchor
 * links would all jump to the first occurrence. The suffix strategy
 * mirrors GitHub/Stripe: first occurrence wins the bare slug, later
 * ones get `-2`, `-3`, etc.
 *
 * This helper is also used by ArticleContent.tsx when it assigns
 * `id` attributes to the rendered DOM headings, so TOC slugs and
 * DOM ids stay in sync in document order.
 */
export function uniqueSlug(base: string, seen: Map<string, number>): string {
  const count = seen.get(base) ?? 0
  seen.set(base, count + 1)
  if (count === 0) return base
  return `${base}-${count + 1}`
}

export function extractTocFromHtml(html: string): TocItem[] {
  const matches = html.matchAll(
    /<h([23])(?:\s+id="([^"]*)")?[^>]*>(.*?)<\/h[23]>/gi,
  )
  const seen = new Map<string, number>()
  return Array.from(matches).map((m) => {
    const text = extractPlainText(m[3]!)
    const base = m[2] || slugifyHeading(text)
    return {
      level: parseInt(m[1]!, 10),
      id: uniqueSlug(base, seen),
      text,
    }
  })
}

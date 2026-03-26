export interface TocItem {
  id: string
  text: string
  level: number
}

function extractPlainText(html: string): string {
  const template = document.createElement('template')
  template.innerHTML = html
  return template.content.textContent?.trim() ?? ''
}

function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^\w\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .trim()
}

export function extractTocFromHtml(html: string): TocItem[] {
  const matches = html.matchAll(
    /<h([23])(?:\s+id="([^"]*)")?[^>]*>(.*?)<\/h[23]>/gi,
  )
  return Array.from(matches).map((m) => {
    const text = extractPlainText(m[3]!)
    return {
      level: parseInt(m[1]!, 10),
      id: m[2] || slugify(text),
      text,
    }
  })
}

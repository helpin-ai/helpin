export interface TocItem {
  id: string
  text: string
  level: number
}

export function extractTocFromHtml(html: string): TocItem[] {
  const matches = html.matchAll(
    /<h([23])\s+id="([^"]+)"[^>]*>(.*?)<\/h[23]>/gi,
  )
  return Array.from(matches).map((m) => ({
    level: parseInt(m[1]!, 10),
    id: m[2]!,
    text: m[3]!.replace(/<[^>]+>/g, ''),
  }))
}

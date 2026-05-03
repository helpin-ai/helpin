export interface ResolvedEmbed {
  provider: string
  title: string
  description: string
  url: string
  image_url?: string | null
}

const PROVIDERS: Array<{ host: string; provider: string; title: string }> = [
  { host: 'youtube.com', provider: 'YouTube', title: 'YouTube video' },
  { host: 'youtu.be', provider: 'YouTube', title: 'YouTube video' },
  { host: 'loom.com', provider: 'Loom', title: 'Loom recording' },
  { host: 'figma.com', provider: 'Figma', title: 'Figma file' },
  { host: 'github.com', provider: 'GitHub', title: 'GitHub link' },
  { host: 'linear.app', provider: 'Linear', title: 'Linear issue' },
  { host: 'notion.so', provider: 'Notion', title: 'Notion page' },
]

export function resolveEmbedUrl(input: string): ResolvedEmbed | null {
  try {
    const url = new URL(input.trim())
    if (url.protocol !== 'https:' && url.protocol !== 'http:') return null
    const host = url.hostname.replace(/^www\./, '')
    const matched = PROVIDERS.find((provider) => host === provider.host || host.endsWith(`.${provider.host}`))
    const title = matched?.title ?? host
    return {
      provider: matched?.provider ?? host,
      title,
      description: url.pathname === '/' ? host : decodeURIComponent(url.pathname.split('/').filter(Boolean).slice(0, 3).join(' / ')),
      url: url.toString(),
    }
  } catch {
    return null
  }
}

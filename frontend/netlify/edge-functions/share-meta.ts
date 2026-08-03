type EdgeContext = {
  next: () => Promise<Response>
}

type Fetcher = (input: string, init?: RequestInit) => Promise<Response>

const META_START = '<!-- helpin-meta:start -->'
const META_END = '<!-- helpin-meta:end -->'
const SHARE_DESCRIPTION = 'A document securely shared via Helpin.'

export function resolvePublicApiBase(hostname: string): string | null {
  if (hostname === 'app.helpin.ai') return 'https://api.helpin.ai/api'
  if (hostname === 'app.stage.helpin.ai') return 'https://api.stage.helpin.ai/api'
  if (hostname === 'localhost' || hostname === '127.0.0.1') return 'http://localhost:8080/api'
  return null
}

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

function truncateTitle(value: string, maxLength = 80): string {
  const characters = Array.from(value.trim())
  if (characters.length <= maxLength) return characters.join('')
  return `${characters.slice(0, maxLength - 1).join('').trimEnd()}…`
}

export function injectShareMetadata({
  html,
  requestUrl,
  documentTitle,
}: {
  html: string
  requestUrl: string
  documentTitle: string
}): string {
  const start = html.indexOf(META_START)
  const end = html.indexOf(META_END)
  if (start < 0 || end < start) return html

  const url = new URL(requestUrl)
  const shareUrl = `${url.origin}${url.pathname}`
  const imageUrl = `${url.origin}/og/helpin-shared-document.png`
  const title = `${truncateTitle(documentTitle)} — Shared via Helpin`
  const escapedTitle = escapeHtml(title)
  const escapedShareUrl = escapeHtml(shareUrl)
  const escapedImageUrl = escapeHtml(imageUrl)
  const imageAlt = 'A document securely shared via Helpin'

  const metadata = `${META_START}
    <title>${escapedTitle}</title>
    <meta name="application-name" content="Helpin" />
    <meta name="description" content="${SHARE_DESCRIPTION}" />
    <meta name="robots" content="noindex, nofollow, noarchive, nosnippet" />
    <meta name="googlebot" content="noindex, nofollow, noarchive, nosnippet" />
    <meta property="og:title" content="${escapedTitle}" />
    <meta property="og:description" content="${SHARE_DESCRIPTION}" />
    <meta property="og:type" content="article" />
    <meta property="og:url" content="${escapedShareUrl}" />
    <meta property="og:site_name" content="Helpin" />
    <meta property="og:locale" content="en_US" />
    <meta property="og:image" content="${escapedImageUrl}" />
    <meta property="og:image:secure_url" content="${escapedImageUrl}" />
    <meta property="og:image:type" content="image/png" />
    <meta property="og:image:width" content="1200" />
    <meta property="og:image:height" content="630" />
    <meta property="og:image:alt" content="${imageAlt}" />
    <meta name="twitter:card" content="summary_large_image" />
    <meta name="twitter:title" content="${escapedTitle}" />
    <meta name="twitter:description" content="${SHARE_DESCRIPTION}" />
    <meta name="twitter:image" content="${escapedImageUrl}" />
    <meta name="twitter:image:alt" content="${imageAlt}" />
    ${META_END}`

  return `${html.slice(0, start)}${metadata}${html.slice(end + META_END.length)}`
}

export async function handleShareMetadata(
  request: Request,
  context: EdgeContext,
  fetcher: Fetcher = fetch,
): Promise<Response> {
  const originResponse = await context.next()
  if (!originResponse.headers.get('content-type')?.toLowerCase().includes('text/html')) {
    return originResponse
  }

  const requestUrl = new URL(request.url)
  const apiBase = resolvePublicApiBase(requestUrl.hostname)
  if (!apiBase) return originResponse

  const encodedToken = requestUrl.pathname.match(/^\/share\/([^/]+)\/?$/)?.[1]
  if (!encodedToken) return originResponse

  let token: string
  try {
    token = decodeURIComponent(encodedToken)
  } catch {
    return originResponse
  }

  try {
    const apiResponse = await fetcher(`${apiBase}/docs/shared/${encodeURIComponent(token)}`, {
      headers: { accept: 'application/json' },
      signal: AbortSignal.timeout(2_000),
    })
    if (!apiResponse.ok) return originResponse

    const payload = await apiResponse.json() as { document?: { title?: unknown } }
    if (typeof payload.document?.title !== 'string' || !payload.document.title.trim()) {
      return originResponse
    }

    const html = await originResponse.clone().text()
    const transformed = injectShareMetadata({
      html,
      requestUrl: request.url,
      documentTitle: payload.document.title,
    })
    if (transformed === html) return originResponse

    const headers = new Headers(originResponse.headers)
    headers.delete('content-length')
    headers.delete('etag')
    headers.set('cache-control', 'private, no-store')
    headers.set('content-type', 'text/html; charset=utf-8')
    headers.set('x-robots-tag', 'noindex, nofollow, noarchive, nosnippet')
    return new Response(transformed, {
      status: originResponse.status,
      statusText: originResponse.statusText,
      headers,
    })
  } catch {
    return originResponse
  }
}

export default function shareMetadata(request: Request, context: EdgeContext) {
  return handleShareMetadata(request, context)
}

export const config = {
  path: '/share/*',
  onError: 'bypass',
}

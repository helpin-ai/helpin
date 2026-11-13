type EdgeContext = { next: () => Promise<Response> }
type Fetcher = (input: string, init?: RequestInit) => Promise<Response>

const META_START = '<!-- helpin-meta:start -->'
const META_END = '<!-- helpin-meta:end -->'

export function resolvePublicApiBase(hostname: string): string | null {
  if (hostname === 'app.helpin.ai') return 'https://api.helpin.ai/api'
  if (hostname === 'stage.helpin.ai' || hostname === 'app.stage.helpin.ai') return 'https://api.stage.helpin.ai/api'
  if (hostname === 'localhost' || hostname === '127.0.0.1') return 'http://localhost:8080/api'
  return null
}

/** The Go service owns content, eligibility, escaping, and the HTML template. */
export function injectShareMetadata({ html, head }: { html: string; head: string }): string {
  const start = html.indexOf(META_START)
  const end = html.indexOf(META_END)
  if (start < 0 || end < start) return html
  return `${html.slice(0, start)}${META_START}\n${head}\n${html.slice(end)}`
}

export async function loadPreviewHead(
  path: string,
  query: string,
  apiBase: string,
  fetcher: Fetcher = fetch,
): Promise<string | null> {
  const params = new URLSearchParams({ path, query })
  const response = await fetcher(`${apiBase.replace(/\/$/, '')}/link-previews/metadata?${params}`, {
    headers: { accept: 'application/json' },
    signal: AbortSignal.timeout(3_000),
  })
  if (!response.ok) return null
  const payload: unknown = await response.json()
  if (!payload || typeof payload !== 'object' || !('head' in payload)) return null
  const head = payload.head
  return typeof head === 'string' && head.length <= 16_384 && head.startsWith('<title>') ? head : null
}

export async function handleShareMetadata(
  request: Request,
  context: EdgeContext,
  fetcher: Fetcher = fetch,
): Promise<Response> {
  const requestUrl = new URL(request.url)
  const apiBase = resolvePublicApiBase(requestUrl.hostname)
  if (!apiBase || !['GET', 'HEAD'].includes(request.method)) return context.next()

  // The app origin also works when PUBLIC_API_BASE_URL is not set on the API.
  // Rechecking eligibility happens on the Go endpoint, including old image URLs.
  if (requestUrl.pathname === '/api/link-previews/image') {
    try {
      const response = await fetcher(`${apiBase}/link-previews/image${requestUrl.search}`, {
        method: request.method,
        signal: AbortSignal.timeout(6_000),
      })
      const headers = new Headers({
        'cache-control': 'private, no-store',
        'cdn-cache-control': 'no-store',
        'x-content-type-options': 'nosniff',
      })
      headers.set('content-type', response.headers.get('content-type') ?? 'image/png')
      return new Response(response.body, { status: response.status, headers })
    } catch {
      return new Response('Preview unavailable', { status: 503, headers: { 'cache-control': 'no-store' } })
    }
  }

  const originResponse = await context.next()
  if (!originResponse.headers.get('content-type')?.toLowerCase().includes('text/html')) return originResponse
  try {
    const head = await loadPreviewHead(requestUrl.pathname, requestUrl.search.slice(1), apiBase, fetcher)
    if (!head) return originResponse
    const html = await originResponse.clone().text()
    const transformed = injectShareMetadata({ html, head })
    if (transformed === html) return originResponse
    const headers = new Headers(originResponse.headers)
    headers.delete('content-length')
    headers.delete('content-encoding')
    headers.delete('etag')
    headers.set('cache-control', 'private, no-store')
    headers.set('cdn-cache-control', 'no-store')
    headers.set('content-type', 'text/html; charset=utf-8')
    headers.set('x-robots-tag', 'noindex, nofollow, noarchive, nosnippet')
    return new Response(request.method === 'HEAD' ? null : transformed, { status: originResponse.status, headers })
  } catch {
    return originResponse
  }
}

export default handleShareMetadata

export const config = {
  path: [
    '/', '/w/*', '/share/*', '/shared/*', '/portal/*', '/api/link-previews/image',
    '/login', '/register', '/forgot-password', '/reset-password', '/verify-email',
    '/join/*', '/team-reply/*', '/email-preferences/*', '/oauth/*', '/github/installed',
    '/onboarding', '/profile', '/workspaces', '/demo', '/billing-preview',
  ],
  onError: 'bypass',
}

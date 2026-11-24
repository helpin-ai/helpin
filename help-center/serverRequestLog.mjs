// One line per page request, written before rendering, so the request that
// crashes the process is the last line in `kubectl logs --previous`.
// Query values are dropped: portal links can carry tokens.
const STATIC_ASSET = /\.(?:js|mjs|css|map|png|jpe?g|gif|svg|ico|webp|avif|woff2?|ttf|txt|xml|json)$/i

export function requestLogLine(method, host, rawUrl) {
  const url = new URL(rawUrl || '/', 'http://local')
  if (url.pathname === '/healthz' || STATIC_ASSET.test(url.pathname)) return null
  const keys = [...new Set(url.searchParams.keys())].slice(0, 20).join(',')
  return `hc request ${method || 'GET'} ${host} ${url.pathname.slice(0, 300)}${keys ? ` query_keys=${keys}` : ''}`
}

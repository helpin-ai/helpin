export function formatRedirectTargetPath(targetCollectionSlug: string, targetArticleSlug?: string) {
  const collection = targetCollectionSlug.trim().replace(/^\/+|\/+$/g, '')
  const article = (targetArticleSlug ?? '').trim().replace(/^\/+|\/+$/g, '')

  if (collection && article) return `/${collection}/${article}`
  if (collection) return `/${collection}`
  if (article) return `/${article}`
  return '/'
}

export function normalizeRedirectSourcePathForDisplay(path: string) {
  const segments = path
    .trim()
    .split('/')
    .map((segment) => segment.trim())
    .filter((segment) => segment.length > 0)

  if (segments.length === 0) return '/'
  return `/${segments.join('/')}`
}

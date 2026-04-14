const ARTICLE_PUBLIC_ID_LENGTH = 8

export function buildArticleKey(slug: string, publicId: string) {
  const trimmedSlug = slug.trim().replace(/^\/+|\/+$/g, '')
  const trimmedPublicId = normalizeArticlePublicId(publicId)
  if (!trimmedSlug) return trimmedPublicId
  if (!trimmedPublicId) return trimmedSlug
  return `${trimmedSlug}-${trimmedPublicId}`
}

export function parseArticleKey(articleKey: string) {
  const trimmed = articleKey.trim().replace(/^\/+|\/+$/g, '')
  if (!trimmed) return null

  // Bare PublicID: if the entire key is a valid 8-char hex ID.
  if (trimmed.length === ARTICLE_PUBLIC_ID_LENGTH) {
    const bareId = normalizeArticlePublicId(trimmed)
    if (bareId.length === ARTICLE_PUBLIC_ID_LENGTH) {
      return { slug: '', publicId: bareId }
    }
  }

  const lastDash = trimmed.lastIndexOf('-')
  if (lastDash <= 0 || lastDash === trimmed.length - 1) {
    return null
  }

  const publicId = normalizeArticlePublicId(trimmed.slice(lastDash + 1))
  if (publicId.length !== ARTICLE_PUBLIC_ID_LENGTH) {
    return null
  }

  const slug = trimmed.slice(0, lastDash).trim()
  if (!slug) return null

  return { slug, publicId }
}

export function normalizeArticlePublicId(value: string) {
  const normalized = value.trim().toLowerCase()
  if (normalized.length !== ARTICLE_PUBLIC_ID_LENGTH) {
    return normalized
  }

  return /^[0-9a-f]+$/.test(normalized) ? normalized : ''
}


const COLLECTION_PUBLIC_ID_LENGTH = 8

export function buildCollectionKey(slug: string, publicId: string) {
  const trimmedSlug = slug.trim().replace(/^\/+|\/+$/g, '')
  const trimmedPublicId = normalizeCollectionPublicId(publicId)
  if (!trimmedSlug) return trimmedPublicId
  if (!trimmedPublicId) return trimmedSlug
  return `${trimmedSlug}-${trimmedPublicId}`
}

export function parseCollectionKey(collectionKey: string) {
  const trimmed = collectionKey.trim().replace(/^\/+|\/+$/g, '')
  if (!trimmed) return null

  const lastDash = trimmed.lastIndexOf('-')
  if (lastDash <= 0 || lastDash === trimmed.length - 1) {
    return null
  }

  const publicId = normalizeCollectionPublicId(trimmed.slice(lastDash + 1))
  if (publicId.length !== COLLECTION_PUBLIC_ID_LENGTH) {
    return null
  }

  const slug = trimmed.slice(0, lastDash).trim()
  if (!slug) return null

  return { slug, publicId }
}

export function normalizeCollectionPublicId(value: string) {
  const normalized = value.trim().toLowerCase()
  if (normalized.length !== COLLECTION_PUBLIC_ID_LENGTH) {
    return normalized
  }

  return /^[0-9a-f]+$/.test(normalized) ? normalized : ''
}

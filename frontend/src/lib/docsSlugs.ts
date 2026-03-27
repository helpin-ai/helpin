const slugInvalidChars = /[^a-z0-9-]/g

export function sanitizeDocsSlugInput(value: string): string {
  return value.toLowerCase().replace(slugInvalidChars, '-')
}

export function suggestDocsSlug(label: string, fallback = 'untitled'): string {
  const suggested = label
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')

  return suggested || fallback
}

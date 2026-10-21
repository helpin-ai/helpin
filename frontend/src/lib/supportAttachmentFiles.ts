// Shared file rules for support attachments. The server's attachment policy
// (max size and content types) is authoritative; these helpers apply it in
// the browser so customers and agents get feedback before uploading.

/** Server-provided support attachment rules. */
export interface SupportAttachmentPolicy {
  max_bytes: number
  content_types: string[]
}

// Most browsers cannot render HEIC/HEIF, so those photos are offered as
// downloads instead of inline previews.
const NON_PREVIEWABLE_IMAGE_TYPES = new Set(['image/heic', 'image/heif'])

// Types for files the browser reports without one (common for some video
// containers and HEIC photos chosen from the file system).
const EXTENSION_TYPES: Record<string, string> = {
  mp4: 'video/mp4',
  mov: 'video/quicktime',
  webm: 'video/webm',
  mpeg: 'video/mpeg',
  mpg: 'video/mpeg',
  avi: 'video/x-msvideo',
  mkv: 'video/x-matroska',
  heic: 'image/heic',
  heif: 'image/heif',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  png: 'image/png',
  gif: 'image/gif',
  webp: 'image/webp',
  svg: 'image/svg+xml',
  pdf: 'application/pdf',
  doc: 'application/msword',
  docx: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  xls: 'application/vnd.ms-excel',
  xlsx: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  txt: 'text/plain',
  csv: 'text/csv',
  md: 'text/markdown',
  json: 'application/json',
  zip: 'application/zip',
  gz: 'application/gzip',
  tar: 'application/x-tar',
}

// Browser-reported types that mean "unknown" or are non-standard aliases.
const UNRELIABLE_TYPES = new Set(['', 'application/octet-stream', 'video/avi'])

function extensionOf(name: string) {
  const dot = name.lastIndexOf('.')
  return dot >= 0 ? name.slice(dot + 1).toLowerCase() : ''
}

/** isPreviewableImageType reports whether a browser can show the image inline. */
export function isPreviewableImageType(type: string) {
  return type.startsWith('image/') && !NON_PREVIEWABLE_IMAGE_TYPES.has(type)
}

/**
 * withInferredFileType returns the file with a content type inferred from its
 * extension when the browser did not report a reliable one.
 */
export function withInferredFileType(file: File): File {
  if (!UNRELIABLE_TYPES.has(file.type)) return file
  const inferred = EXTENSION_TYPES[extensionOf(file.name)]
  if (!inferred) return file
  return new File([file], file.name, { type: inferred, lastModified: file.lastModified })
}

/**
 * acceptAttributeFor builds a file input `accept` value for the allowed
 * content types, adding extensions so files without a reported type (such as
 * .mkv or .heic) remain selectable.
 */
export function acceptAttributeFor(contentTypes: string[]) {
  const allowed = new Set(contentTypes)
  const extensions = Object.entries(EXTENSION_TYPES)
    .filter(([, type]) => allowed.has(type))
    .map(([extension]) => `.${extension}`)
  return [...contentTypes, ...extensions].join(',')
}

/** fileRejection returns why the policy refuses a file, or null. */
export function fileRejection(file: File, policy: SupportAttachmentPolicy): string | null {
  if (file.size <= 0) return `${file.name} is empty. Choose a different file.`
  if (file.size > policy.max_bytes) {
    return `${file.name} is larger than ${formatFileSize(policy.max_bytes)}. Choose a smaller file or share a link.`
  }
  if (!policy.content_types.includes(file.type)) {
    return `${file.name} isn’t a supported file type.`
  }
  return null
}

export function formatFileSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(bytes >= 10 * 1024 * 1024 ? 0 : 1)} MB`
}

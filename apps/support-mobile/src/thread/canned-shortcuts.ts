export interface ShortcutToken {
  /** The active token including the leading `!` (e.g. `!tha`). */
  query: string
  /** Index in the text where the token starts. */
  start: number
  /** Index in the text where the token ends (the cursor). */
  end: number
}

/**
 * Finds an active `!code` token ending at `cursor` — i.e. the caret is inside a
 * run that starts with `!` at the start of the text or right after whitespace.
 * Returns null when there's no such token (so the inline suggestion list hides).
 */
export function detectShortcutToken(text: string, cursor: number): ShortcutToken | null {
  const before = text.slice(0, Math.max(0, cursor))
  const match = before.match(/(?:^|\s)(![^\s]*)$/)
  if (!match) return null
  const token = match[1]
  return { query: token, start: cursor - token.length, end: cursor }
}

/**
 * Converts a canned response body (which may be HTML) into plain text suitable
 * for the mobile plain-text composer, preserving paragraph/line breaks (unlike
 * a naive tag strip that collapses everything onto one line). Plain-text or
 * markdown input passes through with its newlines intact.
 */
export function cannedToPlainText(body: string): string {
  const withBreaks = body
    .replace(/<\/(p|div|li|h[1-6])>/gi, '\n')
    .replace(/<br\s*\/?>/gi, '\n')
  const stripped = withBreaks.replace(/<[^>]+>/g, '')
  const decoded =
    typeof DOMParser !== 'undefined'
      ? (new DOMParser().parseFromString(stripped, 'text/html').body.textContent ?? stripped)
      : stripped
  return decoded
    .replace(/[ \t]+\n/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

export interface TextEdit {
  text: string
  cursor: number
}

/**
 * Replaces the half-open range [start, end) of `text` with `insert`, ensuring a
 * single trailing space, and returns the new text plus the caret position that
 * should follow it.
 */
export function replaceRange(text: string, start: number, end: number, insert: string): TextEdit {
  const withSpace = insert.endsWith(' ') || insert.endsWith('\n') ? insert : `${insert} `
  return {
    text: text.slice(0, start) + withSpace + text.slice(end),
    cursor: start + withSpace.length,
  }
}

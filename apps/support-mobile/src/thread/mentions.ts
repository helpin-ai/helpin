/**
 * Teammate @mention helpers for internal notes — mirrors the web thread's
 * mentionSuggestions (frontend/src/components/pm/mentionSuggestions.ts).
 * Reimplemented rather than read-in-place because the web module carries
 * web-only type imports; the logic here is small and stable.
 */

export interface MentionMember {
  id: string
  user_id?: string | null
  email: string
  display_name: string
  avatar_url?: string | null
  avatar_style?: string | null
  avatar_seed?: string | null
  avatar_background_mode?: string | null
  avatar_background_color?: string | null
  presence_status?: 'online' | 'away' | 'offline'
}

export interface MentionSuggestion {
  id: string
  handle: string
  label: string
  secondaryText: string
  avatarUrl?: string | null
  avatarStyle?: string | null
  avatarSeed?: string | null
  avatarBackgroundMode?: string | null
  avatarBackgroundColor?: string | null
  presenceStatus?: 'online' | 'away' | 'offline'
}

/** Normalizes a name/query into a mention handle (verbatim from web). */
export function normalizeMentionHandle(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._\-\s]/g, '')
    .replace(/\s+/g, '.')
    .replace(/^\.+|\.+$/g, '')
}

/** The @handle for a member: from display name, else email local-part, else id. */
export function memberMentionHandle(member: MentionMember): string {
  const fromName = normalizeMentionHandle(member.display_name)
  if (fromName) return fromName
  const fromEmail = normalizeMentionHandle(member.email.split('@')[0] ?? '')
  if (fromEmail) return fromEmail
  return normalizeMentionHandle(member.user_id ?? member.id)
}

/**
 * Ranked mention suggestions for a query. Empty query returns everyone; a query
 * matches on handle, label, or email, with handle-prefix matches ranked first.
 */
export function mentionSuggestions(query: string, members: MentionMember[], limit = 6): MentionSuggestion[] {
  const q = normalizeMentionHandle(query)
  const suggestions = members
    .map((member): MentionSuggestion | null => {
      const handle = memberMentionHandle(member)
      if (!handle) return null
      return {
        id: member.user_id ?? member.id,
        handle,
        label: member.display_name || member.email,
        secondaryText: member.email,
        avatarUrl: member.avatar_url,
        avatarStyle: member.avatar_style,
        avatarSeed: member.avatar_seed,
        avatarBackgroundMode: member.avatar_background_mode,
        avatarBackgroundColor: member.avatar_background_color,
        ...(member.presence_status ? { presenceStatus: member.presence_status } : {}),
      }
    })
    .filter((item): item is MentionSuggestion => item !== null)
    .filter((item) => {
      if (!q) return true
      return (
        item.handle.includes(q) ||
        item.label.toLowerCase().includes(q) ||
        item.secondaryText.toLowerCase().includes(q)
      )
    })
    .sort((a, b) => {
      if (q) {
        const aPrefix = a.handle.startsWith(q) ? 0 : 1
        const bPrefix = b.handle.startsWith(q) ? 0 : 1
        if (aPrefix !== bPrefix) return aPrefix - bPrefix
      }
      return a.label.localeCompare(b.label)
    })
  return suggestions.slice(0, limit)
}

export interface MentionToken {
  /** The query text after the `@` (may be empty). */
  query: string
  /** Index of the `@`. */
  start: number
  /** The cursor index (end of the token). */
  end: number
}

/**
 * Finds an active `@handle` token ending at `cursor` — the `@` must start the
 * text or follow whitespace, and the run after it is `[a-z0-9._-]*`.
 */
export function detectMentionToken(text: string, cursor: number): MentionToken | null {
  const before = text.slice(0, Math.max(0, cursor))
  const match = before.match(/(?:^|\s)@([a-z0-9._-]*)$/i)
  if (!match) return null
  const query = match[1]
  return { query, start: cursor - query.length - 1, end: cursor }
}

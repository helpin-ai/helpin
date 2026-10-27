import { api } from './api'
import type { AssistantSession, AssistantTurn } from './types'

const VISITOR_KEY = 'helpin_assistant_visitor'
const SESSION_KEY = 'helpin_assistant_session'

/**
 * assistantVisitorId identifies this browser to the help assistant, so a
 * conversation can only be read where it was started. It is random and holds
 * no personal data.
 */
export function assistantVisitorId(): string {
  try {
    const existing = window.localStorage.getItem(VISITOR_KEY)
    if (existing && /^[A-Za-z0-9_-]{8,64}$/.test(existing)) return existing
    const created = `v-${randomId()}`
    window.localStorage.setItem(VISITOR_KEY, created)
    return created
  } catch {
    return `v-${randomId()}`
  }
}

function randomId() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID().replace(/-/g, '')
  }
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
}

/** The conversation kept for this browser tab, if any. */
export function storedAssistantSession(subdomain: string): string | null {
  try {
    return window.sessionStorage.getItem(`${SESSION_KEY}:${subdomain}`)
  } catch {
    return null
  }
}

export function storeAssistantSession(subdomain: string, sessionId: string | null) {
  try {
    const key = `${SESSION_KEY}:${subdomain}`
    if (sessionId) window.sessionStorage.setItem(key, sessionId)
    else window.sessionStorage.removeItem(key)
  } catch {
    // Storage can be unavailable; the conversation then lasts for the page.
  }
}

export const helpAssistantService = {
  ask: (subdomain: string, payload: { visitor_id: string; session_id?: string; message: string; page_article_id?: string; locale?: string }) =>
    api.post<AssistantTurn>(`/hc/${subdomain}/assistant/messages`, payload),
  session: (subdomain: string, sessionId: string, visitorId: string) =>
    api.get<AssistantSession>(`/hc/${subdomain}/assistant/sessions/${encodeURIComponent(sessionId)}?visitor_id=${encodeURIComponent(visitorId)}`),
  feedback: (subdomain: string, messageId: string, payload: { visitor_id: string; session_id: string; helpful: boolean }) =>
    api.post<{ saved: boolean }>(`/hc/${subdomain}/assistant/messages/${encodeURIComponent(messageId)}/feedback`, payload),
}

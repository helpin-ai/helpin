import { useEffect, useMemo, useSyncExternalStore } from 'react'
import { AssistantConversation, type AssistantConversationState } from '@helpin-ai/shared'
import { customerPortalService } from '@/lib/services/customerPortalService'

const VISITOR_KEY = 'helpin.portal.assistant.visitor'

function visitorId() {
  try {
    const existing = window.localStorage.getItem(VISITOR_KEY)
    if (existing && /^[A-Za-z0-9_-]{8,64}$/.test(existing)) return existing
    const created = `v-${crypto.randomUUID().replace(/-/g, '')}`
    window.localStorage.setItem(VISITOR_KEY, created)
    return created
  } catch {
    return `v-${Math.random().toString(36).slice(2)}${Date.now().toString(36)}`
  }
}

function sessionKey(slug: string) {
  return `helpin.portal.${slug}.assistant.session`
}

export interface PortalAssistant extends AssistantConversationState {
  send: (text: string, pageArticleId?: string) => void
  retry: () => void
  feedback: (messageId: string, helpful: boolean) => void
  reset: () => void
}

/**
 * usePortalAssistant runs the portal's Ask AI conversation on the shared
 * conversation controller. A signed-in customer's session cookie lets the
 * assistant read help spaces for signed-in customers. When the assistant is
 * busy, the customer is offered a support request instead.
 */
export function usePortalAssistant(slug: string): PortalAssistant {
  const conversation = useMemo(() => new AssistantConversation({
    visitorId,
    ask: (body) => customerPortalService.assistantAsk(slug, body),
    session: (sessionId, visitor) => customerPortalService.assistantSession(slug, sessionId, visitor),
    feedback: (messageId, body) => customerPortalService.assistantFeedback(slug, messageId, body),
    fallback: async () => null,
    loadSessionId: () => {
      try {
        return window.sessionStorage.getItem(sessionKey(slug))
      } catch {
        return null
      }
    },
    saveSessionId: (sessionId) => {
      try {
        if (sessionId) window.sessionStorage.setItem(sessionKey(slug), sessionId)
        else window.sessionStorage.removeItem(sessionKey(slug))
      } catch {
        // The conversation then lasts for the page.
      }
    },
  }), [slug])

  useEffect(() => {
    void conversation.restore()
    return () => conversation.dispose()
  }, [conversation])

  const state = useSyncExternalStore(conversation.subscribe, conversation.getSnapshot, conversation.getSnapshot)
  return useMemo(() => ({
    ...state,
    send: (text: string, pageArticleId?: string) => conversation.send(text, pageArticleId),
    retry: () => conversation.retry(),
    feedback: (messageId: string, helpful: boolean) => conversation.feedback(messageId, helpful),
    reset: () => conversation.reset(),
  }), [conversation, state])
}

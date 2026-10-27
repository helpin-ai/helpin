import { useEffect, useMemo, useSyncExternalStore } from 'react'
import { useDocsContext } from '@/contexts/DocsContext'
import { assistantVisitorId, helpAssistantService, storeAssistantSession, storedAssistantSession } from '@/lib/helpAssistant'
import { buildCanonicalArticlePath } from '@/lib/locale'
import { helpCenterService } from '@/lib/services'
import { AssistantConversation, type AssistantConversationState } from '../../../packages/shared/src/help-assistant'

export interface HelpAssistantState extends AssistantConversationState {
  send: (text: string, pageArticleId?: string) => void
  retry: () => void
  feedback: (messageId: string, helpful: boolean) => void
  reset: () => void
}

/**
 * useHelpAssistant runs the help center's Ask AI conversation on the shared
 * conversation controller. When the assistant is busy, a question falls back
 * to the help center's one-shot AI answer.
 */
export function useHelpAssistant(): HelpAssistantState {
  const { subdomain, locale, multilingualEnabled } = useDocsContext()
  const conversation = useMemo(() => new AssistantConversation({
    visitorId: assistantVisitorId,
    ask: (body) => helpAssistantService.ask(subdomain, { ...body, locale }),
    session: (sessionId, visitorId) => helpAssistantService.session(subdomain, sessionId, visitorId),
    feedback: (messageId, body) => helpAssistantService.feedback(subdomain, messageId, body),
    fallback: async (question) => {
      const result = await helpCenterService.askAnswer(subdomain, locale, multilingualEnabled, { query: question })
      const response = result.data
      if (response?.status !== 'answered' || !response.answer) return null
      return {
        content: response.answer,
        citations: response.citations.map((citation) => ({
          article_id: citation.document_id,
          title: citation.title,
          url: buildCanonicalArticlePath(multilingualEnabled, locale, citation.slug, citation.public_id),
        })),
      }
    },
    loadSessionId: () => storedAssistantSession(subdomain),
    saveSessionId: (sessionId) => storeAssistantSession(subdomain, sessionId),
  }), [locale, multilingualEnabled, subdomain])

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

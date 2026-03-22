import type { SupportConversation } from './pmTypes';
import type { NavFilter } from '@/stores/supportInboxStore';

function isAIManagedConversation(conversation: SupportConversation): boolean {
  return conversation.ai_state === 'pending' || conversation.ai_state === 'resolved';
}

function isHumanInboxConversation(conversation: SupportConversation): boolean {
  return !isAIManagedConversation(conversation);
}

export function filterSupportConversations(
  conversations: SupportConversation[],
  options: {
    navFilter: NavFilter;
    userId?: string;
    searchQuery: string;
  }
): SupportConversation[] {
  const { navFilter, userId, searchQuery } = options;
  let result = [...conversations];

  if (navFilter === 'my_inbox' && userId) {
    result = result.filter((conversation) => isHumanInboxConversation(conversation) && conversation.opened_by_user_id === userId);
  } else if (navFilter === 'unassigned') {
    result = result.filter((conversation) => isHumanInboxConversation(conversation) && !conversation.assigned_agent_id && !conversation.opened_by_user_id);
  } else if (navFilter === 'ai_all') {
    result = result.filter((conversation) => conversation.ai_state != null);
  } else if (navFilter === 'ai_resolved') {
    result = result.filter((conversation) => conversation.ai_state === 'resolved');
  } else if (navFilter === 'ai_escalated') {
    result = result.filter((conversation) => conversation.ai_state === 'escalated');
  } else if (navFilter === 'ai_pending') {
    result = result.filter((conversation) => conversation.ai_state === 'pending');
  } else if (navFilter !== 'mentions') {
    result = result.filter(isHumanInboxConversation);
  }

  if (searchQuery.trim()) {
    const query = searchQuery.toLowerCase();
    result = result.filter(
      (conversation) =>
        conversation.subject.toLowerCase().includes(query) ||
        conversation.customer_name?.toLowerCase().includes(query) ||
        conversation.customer_email?.toLowerCase().includes(query) ||
        String(conversation.display_id).includes(query)
    );
  }

  result.sort((a, b) => {
    const resolvedStatuses = new Set(['resolved', 'closed']);
    const aResolved = resolvedStatuses.has(a.status) ? 1 : 0;
    const bResolved = resolvedStatuses.has(b.status) ? 1 : 0;
    if (aResolved !== bResolved) return aResolved - bResolved;
    return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
  });

  return result;
}

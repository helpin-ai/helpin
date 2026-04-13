import type { SupportConversation } from './pmTypes';
import type { NavFilter } from '@/stores/supportInboxStore';
import {
  isAIActiveConversation,
  isHumanQueueConversation,
  isResolvedByAIConversation,
} from '@/components/support/helpers';

function matchesMailboxScope(conversation: SupportConversation, mailboxScope: string): boolean {
  if (mailboxScope === 'all') {
    return true;
  }
  if (mailboxScope === 'shared') {
    return !conversation.mailbox_id;
  }
  return conversation.mailbox_id === mailboxScope;
}

export function filterSupportConversations(
  conversations: SupportConversation[],
  options: {
    navFilter: NavFilter;
    mailboxScope: string;
    statusFilter?: string;
    userId?: string;
    searchQuery: string;
  }
): SupportConversation[] {
  const { navFilter, mailboxScope, statusFilter = 'all', userId, searchQuery } = options;
  let result = conversations.filter((conversation) => matchesMailboxScope(conversation, mailboxScope));

  if (statusFilter !== 'all') {
    result = result.filter((conversation) => conversation.status === statusFilter);
  }

  if (navFilter === 'my_inbox' && userId) {
    result = result.filter((conversation) => isHumanQueueConversation(conversation) && conversation.opened_by_user_id === userId);
  } else if (navFilter === 'unassigned') {
    result = result.filter((conversation) => isHumanQueueConversation(conversation) && !conversation.assigned_agent_id && !conversation.opened_by_user_id);
  } else if (navFilter === 'ai_active') {
    result = result.filter(isAIActiveConversation);
  } else if (navFilter === 'resolved_by_ai') {
    result = result.filter(isResolvedByAIConversation);
  } else if (navFilter !== 'mentions') {
    result = result.filter(isHumanQueueConversation);
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
    const resolvedStatuses = new Set(['resolved']);
    const aResolved = resolvedStatuses.has(a.status) ? 1 : 0;
    const bResolved = resolvedStatuses.has(b.status) ? 1 : 0;
    if (aResolved !== bResolved) return aResolved - bResolved;
    return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
  });

  return result;
}

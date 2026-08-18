import type { CommandBarPageContext } from '@/lib/pmTypes';

interface SupportConversationContextSource {
  id: string;
  display_id?: number | null;
  subject?: string | null;
  title?: string | null;
  customer_name?: string | null;
  customer_email?: string | null;
}

export function buildSupportConversationPageContext(
  conversation: SupportConversationContextSource | null | undefined,
): CommandBarPageContext | null {
  if (!conversation?.id) return null;
  const displayTitle = [
    conversation.subject,
    conversation.title,
    conversation.customer_name,
    conversation.customer_email,
  ].find((value) => value?.trim())?.trim() ?? `Conversation ${conversation.id}`;

  return {
    entity_type: 'support_conversation',
    entity_id: conversation.id,
    ...(conversation.display_id ? { display_id: `#${conversation.display_id}` } : {}),
    display_title: displayTitle,
  };
}

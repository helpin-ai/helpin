import type { SupportConversation } from '@/lib/pmTypes';

/** isHiddenPortalIntake reports an anonymous portal request its submitter has not confirmed. */
export function isHiddenPortalIntake(conversation: SupportConversation) {
  return conversation.channel === 'portal' && conversation.source === 'portal' &&
    !conversation.portal_visible && !conversation.portal_visibility_changed_at && !conversation.anonymized_at;
}

import type { AIMessageMetadata, SupportConversation, SupportLinkPreview, SupportLinkSecurity, SupportMessage } from '@/lib/pmTypes';
import { getReplyDeliveryMode } from './replyDelivery';
export { AVATAR_COLORS, getAvatarColor } from '@/lib/avatarColor';
export { getInitials as getInitial } from '@/lib/utils';

export const HELPIN_AI_DISPLAY_NAME = 'Helpin AI';

export function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h`;
  const days = Math.floor(hrs / 24);
  return `${days}d`;
}

export function formatTimestamp(dateStr: string): string {
  return new Date(dateStr).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

export function formatMessageTime(dateStr: string): string {
  return new Date(dateStr).toLocaleTimeString(undefined, {
    hour: 'numeric',
    minute: '2-digit',
  });
}

export function getDayLabel(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
  // Compare calendar days so daylight-saving transitions do not shift the label.
  if (date.toDateString() === now.toDateString()) return 'Today';
  if (date.toDateString() === yesterday.toDateString()) return 'Yesterday';
  return date.toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    year: date.getFullYear() === now.getFullYear() ? undefined : 'numeric',
  });
}

export function isSameDay(a: string, b: string): boolean {
  const da = new Date(a);
  const db = new Date(b);
  return da.getFullYear() === db.getFullYear() && da.getMonth() === db.getMonth() && da.getDate() === db.getDate();
}

export function parseAIMessageMetadata(metadata?: string): AIMessageMetadata | null {
  if (!metadata) return null;
  try {
    const parsed = JSON.parse(metadata) as Partial<AIMessageMetadata> & { ai_auto_reply?: boolean };
    return parsed.ai_auto_reply ? (parsed as AIMessageMetadata) : null;
  } catch {
    return null;
  }
}

export function isAIMessage(message: Pick<SupportMessage, 'sender_type' | 'metadata'>): boolean {
  if (message.sender_type === 'ai') {
    return true;
  }
  return parseAIMessageMetadata(message.metadata) !== null;
}

export type SupportReceiptStatus = 'sending_email' | 'delivered' | 'sent_email' | 'delivered_email' | 'read' | 'read_email' | 'sent_outside_helpin' | null;

export function isExternalSupportEmailReply(metadata?: string): boolean {
  if (!metadata) return false;
  try {
    const parsed = JSON.parse(metadata) as { external_email_reply?: unknown };
    return parsed.external_email_reply === true;
  } catch {
    return false;
  }
}

export function getSupportReceiptStatus(
  message: SupportMessage,
  conversation: Pick<SupportConversation, 'source' | 'contact_last_seen_at'>,
): SupportReceiptStatus {
  if (isExternalSupportEmailReply(message.metadata)) return 'sent_outside_helpin';
  if (message.email_read_at) return 'read_email';
  if (message.email_delivery_status === 'opened') return 'read_email';
  if (message.email_delivery_status === 'delivered') return 'delivered_email';
  if (message.id.startsWith('optimistic-') && message.via_channel === 'email') return 'sending_email';
  if (getReplyDeliveryMode(message.metadata) === 'email_only') {
    return message.email_notified_at || message.email_delivery_status === 'sent' ? 'sent_email' : null;
  }
  const seen = conversation.contact_last_seen_at;
  if (conversation.source === 'widget' && seen && new Date(seen) >= new Date(message.created_at)) return 'read';
  if (message.email_notified_at) return 'sent_email';
  if (conversation.source === 'widget') return 'delivered';
  return null;
}

/** Explicit replies keep channel intent separate from actual email delivery. */
export function getExplicitEmailDeliveryState(message: SupportMessage): { label: string; failed: boolean; error?: string } {
  let metadata: { email_delivery_status?: string; email_delivery_error?: string } = {};
  try {
    metadata = JSON.parse(message.metadata ?? '{}') ?? {};
  } catch { /* Legacy or malformed metadata carries no delivery outcome. */ }
  const status = message.email_delivery_status || metadata.email_delivery_status;
  const error = message.email_delivery_error || metadata.email_delivery_error;
  if (status === 'spam_complaint') return { label: 'Marked as spam', failed: true, error };
  if (status === 'failed' || status === 'bounced') return { label: 'Failed', failed: true, error };
  if (status === 'blocked') return { label: 'Not sent', failed: true, error };
  if (message.email_read_at || status === 'opened') return { label: 'Opened', failed: false };
  if (status === 'delivered') return { label: 'Delivered', failed: false };
  if (message.email_notified_at || status === 'sent') return { label: 'Sent', failed: false };
  if (message.id.startsWith('optimistic-') || status === 'sending') return { label: 'Sending', failed: false };
  if (status === 'queued' || (message.cancellable_until && Date.parse(message.cancellable_until) > Date.now())) {
    return { label: 'Queued', failed: false };
  }
  return { label: 'Pending', failed: false };
}

export function getEffectiveSenderType(message: Pick<SupportMessage, 'sender_type' | 'metadata'>): SupportMessage['sender_type'] {
  return isAIMessage(message) ? 'ai' : message.sender_type;
}

export type EffectiveSupportFlowState =
  | 'ai_handling'
  | 'waiting_for_human'
  | 'after_hours_queue'
  | 'assigned_to_human'
  | 'resolved_by_ai'
  | 'resolved_by_human'
  | null;

export type ConversationStateBadge = {
  key: string;
  label: string;
  tone: 'blue' | 'emerald' | 'amber' | 'slate';
};

export function getEffectiveFlowState(conversation: Pick<SupportConversation, 'flow_state' | 'ai_state' | 'human_takeover'>): EffectiveSupportFlowState {
  if (conversation.human_takeover) {
    return 'assigned_to_human';
  }
  if (conversation.flow_state === 'queued_for_human') {
    return 'waiting_for_human';
  }
  if (conversation.flow_state) {
    return conversation.flow_state;
  }
  if (conversation.ai_state === 'pending') {
    return 'ai_handling';
  }
  if (conversation.ai_state === 'resolved') {
    return 'resolved_by_ai';
  }
  return null;
}

export function isAIActiveConversation(conversation: Pick<SupportConversation, 'flow_state' | 'ai_state' | 'human_takeover'>): boolean {
  return getEffectiveFlowState(conversation) === 'ai_handling';
}

export function isResolvedByAIConversation(conversation: Pick<SupportConversation, 'flow_state' | 'ai_state' | 'human_takeover'>): boolean {
  return getEffectiveFlowState(conversation) === 'resolved_by_ai';
}

export function isHumanQueueConversation(conversation: Pick<SupportConversation, 'flow_state' | 'ai_state' | 'human_takeover'>): boolean {
  return !isAIActiveConversation(conversation) && !isResolvedByAIConversation(conversation);
}

export function isHumanInboxConversation(
  conversation: Pick<SupportConversation, 'status' | 'flow_state' | 'ai_state' | 'human_takeover' | 'customer_requested_human_at'>,
): boolean {
  return (conversation.status === 'open' || conversation.status === 'waiting_on_customer') && (
    isHumanQueueConversation(conversation) ||
    conversation.ai_state === 'escalated' ||
    Boolean(conversation.customer_requested_human_at)
  );
}

export function isHumanResolvedConversation(
  conversation: Pick<SupportConversation, 'status' | 'flow_state' | 'ai_state' | 'human_takeover'>,
): boolean {
  return conversation.status === 'resolved' && !isResolvedByAIConversation(conversation);
}

export function isUserOwnedConversation(
  conversation: Pick<SupportConversation, 'assigned_user_id' | 'opened_by_user_id'>,
  userId?: string,
): boolean {
  if (!userId) return false;
  return conversation.assigned_user_id === userId || conversation.opened_by_user_id === userId;
}

export function isMineActionableConversation(
  conversation: Pick<SupportConversation, 'status' | 'flow_state' | 'ai_state' | 'human_takeover' | 'customer_requested_human_at' | 'assigned_user_id' | 'opened_by_user_id'>,
  userId?: string,
): boolean {
  if (!isUserOwnedConversation(conversation, userId)) return false;
  return isHumanInboxConversation(conversation) || conversation.status === 'waiting_on_customer';
}

export function getConversationStateBadges(
  conversation: Pick<SupportConversation, 'flow_state' | 'ai_state' | 'customer_requested_human_at' | 'human_takeover'>,
): ConversationStateBadge[] {
  const badges: ConversationStateBadge[] = [];
  const flowState = getEffectiveFlowState(conversation);

  if (flowState === 'ai_handling') {
    badges.push({ key: 'ai_active', label: 'AI Active', tone: 'blue' });
  }
  if (conversation.ai_state === 'escalated') {
    badges.push({ key: 'ai_escalated', label: 'Escalated by AI', tone: 'amber' });
  }
  if (flowState === 'after_hours_queue') {
    badges.push({ key: 'after_hours', label: 'After Hours', tone: 'slate' });
  }
  if (flowState === 'resolved_by_ai') {
    badges.push({ key: 'resolved_by_ai', label: 'Resolved by AI', tone: 'emerald' });
  }
  if (conversation.customer_requested_human_at) {
    badges.push({ key: 'requested_human', label: 'Requested Human', tone: 'amber' });
  }

  return badges;
}

export function parseSupportLinkPreviews(metadata?: string): SupportLinkPreview[] {
  if (!metadata) return [];
  try {
    const parsed = JSON.parse(metadata) as { link_previews?: unknown };
    if (!Array.isArray(parsed.link_previews)) {
      return [];
    }
    return parsed.link_previews.filter((preview): preview is SupportLinkPreview => {
      if (!preview || typeof preview !== 'object') return false;
      const candidate = preview as Partial<SupportLinkPreview>;
      return typeof candidate.url === 'string' && typeof candidate.title === 'string';
    });
  } catch {
    return [];
  }
}

export function normalizeSupportLinkHref(href: string): string | null {
  try {
    const parsed = new URL(href);
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null;
    if (parsed.username || parsed.password) return null;
    parsed.hash = '';
    return parsed.href;
  } catch {
    return null;
  }
}

export function parseSupportLinkSecurity(metadata?: string): SupportLinkSecurity[] {
  if (!metadata) return [];
  try {
    const parsed = JSON.parse(metadata) as { link_security?: unknown };
    if (!Array.isArray(parsed.link_security)) return [];
    const now = Date.now();
    return parsed.link_security.filter((entry): entry is SupportLinkSecurity => {
      if (!entry || typeof entry !== 'object') return false;
      const candidate = entry as Partial<SupportLinkSecurity>;
      if (typeof candidate.url !== 'string' || normalizeSupportLinkHref(candidate.url) === null) return false;
      if (!['no_match', 'malicious', 'unknown'].includes(candidate.status ?? '')) return false;
      if (typeof candidate.checked_at !== 'string' || Number.isNaN(Date.parse(candidate.checked_at))) return false;
      if (typeof candidate.expires_at !== 'string' || Date.parse(candidate.expires_at) <= now) return false;
      return !candidate.threat_types || candidate.threat_types.every((value) =>
        ['MALWARE', 'SOCIAL_ENGINEERING', 'UNWANTED_SOFTWARE'].includes(value));
    });
  } catch {
    return [];
  }
}

export function findSupportLinkSecurity(entries: SupportLinkSecurity[], href: string): SupportLinkSecurity | undefined {
  const normalized = normalizeSupportLinkHref(href);
  if (!normalized) return undefined;
  return entries.find((entry) => normalizeSupportLinkHref(entry.url) === normalized);
}

import type { AIMessageMetadata, SupportConversation, SupportLinkPreview, SupportMessage } from '@/lib/pmTypes';

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
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const messageDay = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const diffDays = Math.floor((today.getTime() - messageDay.getTime()) / 86400000);

  if (diffDays === 0) return 'Today';
  if (diffDays === 1) return 'Yesterday';
  if (diffDays < 7) {
    return date.toLocaleDateString(undefined, { weekday: 'long' });
  }
  return date.toLocaleDateString(undefined, { month: 'long', day: 'numeric', year: 'numeric' });
}

export function isSameDay(a: string, b: string): boolean {
  const da = new Date(a);
  const db = new Date(b);
  return da.getFullYear() === db.getFullYear() && da.getMonth() === db.getMonth() && da.getDate() === db.getDate();
}

export function getInitial(name?: string): string {
  if (!name) return '?';
  return name.charAt(0).toUpperCase();
}

// Generate a consistent color from a string (name, email, or anonymous ID)
export const AVATAR_COLORS = [
  'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400',
  'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
  'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
  'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-400',
  'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400',
  'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400',
  'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400',
  'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400',
];

export function getAvatarColor(seed: string): string {
  let hash = 0;
  for (let i = 0; i < seed.length; i++) {
    hash = seed.charCodeAt(i) + ((hash << 5) - hash);
  }
  return AVATAR_COLORS[Math.abs(hash) % AVATAR_COLORS.length];
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

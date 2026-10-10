import type { SupportConversation, SupportMessage } from '@/lib/pmTypes';
import { formatTimestamp } from './helpers';

type FollowUpStage = {
  kind: 'scheduled' | 'reviewing' | 'final' | 'closing' | 'stopped' | 'failed';
  deadline?: string;
};

export function getSupportFollowUpStage(
  conversation: SupportConversation,
  latestPublicMessage?: Pick<SupportMessage, 'id' | 'created_at'>,
): FollowUpStage | null {
  const followUp = conversation.ai_follow_up;
  if (
    !followUp ||
    !['open', 'waiting_on_customer'].includes(conversation.status) ||
    conversation.flow_state !== 'ai_handling' ||
    conversation.ai_state !== 'pending' ||
    conversation.human_takeover ||
    conversation.assigned_user_id ||
    conversation.opened_by_user_id ||
    conversation.customer_requested_human_at ||
    conversation.linked_task_id ||
    conversation.anonymized_at ||
    conversation.customer_awaiting_response ||
    conversation.last_public_sender_type !== 'ai'
  )
    return null;
  if (
    conversation.ai_resumed_at &&
    new Date(followUp.created_at) <= new Date(conversation.ai_resumed_at)
  )
    return null;
  const anchor =
    followUp.second_message_id || followUp.sent_message_id || followUp.source_message_id;
  if (!anchor || conversation.last_public_message_id !== anchor) return null;
  // Message websocket updates can arrive before the conversation's refetch.
  if (
    latestPublicMessage &&
    latestPublicMessage.id !== anchor &&
    conversation.last_public_message_at &&
    new Date(latestPublicMessage.created_at) >= new Date(conversation.last_public_message_at)
  )
    return null;
  switch (followUp.status) {
    case 'scheduled':
      return { kind: 'scheduled', deadline: followUp.due_at };
    case 'assessing':
      return { kind: 'reviewing' };
    case 'waiting':
      return (followUp.sequence_version ?? 1) >= 2 && !followUp.second_sent_at
        ? { kind: 'final', deadline: followUp.due_at }
        : { kind: 'closing', deadline: followUp.close_at ?? followUp.due_at };
    case 'failed':
      return { kind: 'failed' };
    case 'cancelled':
      return followUp.reason === 'cancelled_by_teammate' ? { kind: 'stopped' } : null;
    default:
      return null;
  }
}

// Only the explicit internal automation notice is replaced by the live status.
// Public reminders and ordinary teammate notes remain in the transcript.
export function isFollowUpStatusNote(message: SupportMessage): boolean {
  if (!message.is_internal || message.sender_type !== 'ai' || !message.metadata) return false;
  try {
    const metadata = JSON.parse(message.metadata);
    return (
      !!metadata?.support_follow_up_id && typeof metadata.follow_up_failure_reason === 'string'
    );
  } catch {
    return false;
  }
}

// Date/Intl use the viewer's timezone, matching the rest of the conversation.
export function formatFollowUpTime(value: string, now = new Date()): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'Time unavailable';
  if (date.getFullYear() !== now.getFullYear()) return formatTimestamp(value, now);
  const tomorrow = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
  const day =
    date.toDateString() === now.toDateString()
      ? 'Today'
      : date.toDateString() === tomorrow.toDateString()
        ? 'Tomorrow'
        : null;
  return day
    ? `${day}, ${date.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })}`
    : formatTimestamp(value, now);
}

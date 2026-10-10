import type { SupportMessage, TicketSource } from '@/lib/pmTypes';
import { isExternalSupportEmailReply, type SupportReceiptStatus } from './helpers';
import { getReplyDeliveryMode } from './replyDelivery';

export type DeliveryState = 'sent' | 'delivered' | 'read' | 'queued' | 'sending' | 'failed' | 'unknown' | 'external';
export interface ChannelDelivery {
  channel: 'chat' | 'email';
  state: DeliveryState;
  description: string;
}
export interface MessageDelivery {
  channels: ChannelDelivery[];
  state: DeliveryState;
}

function emailDelivery(message: SupportMessage, receipt?: SupportReceiptStatus): ChannelDelivery {
  let metadata: { email_delivery_status?: string; email_delivery_error?: string } = {};
  try { metadata = JSON.parse(message.metadata ?? '{}') ?? {}; }
  catch { /* Legacy messages may not have valid metadata. */ }
  const status = message.email_delivery_status || metadata.email_delivery_status;
  const error = message.email_delivery_error || metadata.email_delivery_error;
  const email = (state: DeliveryState, label: string): ChannelDelivery => ({
    channel: 'email', state, description: `Email ${label}${state === 'failed' && error ? ` · ${error}` : ''}`,
  });
  if (status === 'spam_complaint') return email('failed', 'marked as spam');
  if (status === 'blocked') return email('failed', 'not sent');
  if (status === 'failed' || status === 'bounced') return email('failed', 'failed');
  if (message.email_read_at || status === 'opened' || receipt === 'read_email') return email('read', 'opened');
  if (status === 'delivered' || receipt === 'delivered_email') return email('delivered', 'delivered');
  if (message.email_notified_at || status === 'sent' || receipt === 'sent_email') return email('sent', 'sent');
  if (status === 'sending' || receipt === 'sending_email') return email('sending', 'sending');
  if (status === 'queued' || (message.cancellable_until && Date.parse(message.cancellable_until) > Date.now())) return email('queued', 'queued');
  return email('unknown', 'status unavailable');
}

/** Keep each channel's outcome; a successful chat must not hide an email failure. */
export function getMessageDelivery(
  message: SupportMessage,
  { source, contactLastSeenAt, receiptStatus }: {
    source?: TicketSource;
    contactLastSeenAt?: string;
    receiptStatus?: SupportReceiptStatus;
  } = {},
): MessageDelivery | null {
  if (message.sender_type === 'customer' || message.is_internal || (message.message_type && message.message_type !== 'reply') || message.pending_send) return null;
  if (isExternalSupportEmailReply(message.metadata) || receiptStatus === 'sent_outside_helpin') {
    return { channels: [{ channel: 'email', state: 'external', description: 'Email sent outside Helpin · Delivery tracking unavailable' }], state: 'external' };
  }
  const mode = getReplyDeliveryMode(message.metadata);
  const email = emailDelivery(message, receiptStatus);
  const hasEmail = mode ? mode !== 'chat_only' : (
    message.via_channel === 'email' || source === 'email' || !!message.cancellable_until || email.state !== 'unknown'
  );
  const hasChat = mode ? mode !== 'email_only' : (
    source === 'widget' || message.via_channel === 'widget' || receiptStatus === 'read' || receiptStatus === 'sent' || receiptStatus === 'delivered'
  );
  const channels: ChannelDelivery[] = [];
  if (hasChat) {
    const seen = receiptStatus === 'read' || (source === 'widget' && !!contactLastSeenAt && Date.parse(contactLastSeenAt) >= Date.parse(message.created_at));
    // Persisted in the conversation means sent; only a visitor read cursor confirms seen.
    const state = message.id.startsWith('optimistic-') ? 'sending' : seen ? 'read' : 'sent';
    channels.push({ channel: 'chat', state, description: `Chat ${state === 'read' ? 'seen' : state}` });
  }
  if (hasEmail) {
    channels.push(message.id.startsWith('optimistic-') && email.state !== 'failed'
      ? { channel: 'email', state: 'sending', description: 'Email sending' }
      : email);
  }
  if (!channels.length) return null;
  // One receipt for the furthest successful channel, with failures kept on their channel icon.
  const state = (['read', 'delivered', 'sent', 'sending', 'queued', 'failed', 'unknown'] as const)
    .find(candidate => channels.some(channel => channel.state === candidate)) ?? 'unknown';
  return { channels, state };
}

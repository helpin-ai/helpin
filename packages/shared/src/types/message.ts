/**
 * Canonical set of system_event_type values the backend may set on a
 * message_type='system' row. Widget and admin surfaces branch on this value
 * instead of keyword-matching content so copy changes and i18n don't break
 * routing. Source of truth: server/internal/model/support_system_event.go.
 */
export const SYSTEM_EVENT_TYPES = [
  'teammate_joined',
  'assigned',
  'unassigned',
  'took',
  'agent_assigned',
  'mailbox_moved',
  'triage_routed',
  'triage_dismissed',
  'ai_escalated',
  'delayed_team_reply',
  'resolved',
  'reopened',
  'closed',
] as const;

export type SystemEventType = (typeof SYSTEM_EVENT_TYPES)[number];
export type AIReplyKind = 'answer' | 'clarify' | 'conversational' | 'confirmation' | 'greeting';

export interface Message {
  id: string;
  conversationId: string;
  role: 'customer' | 'agent' | 'ai' | 'system';
  content: string;
  senderId?: string;
  senderName?: string;
  senderAvatar?: string;
  systemEventType?: SystemEventType;
  /** One-time automated waiting update after a human handoff. */
  delayedTeamReply?: boolean;
  captureEmail?: boolean;
  sources?: AiSource[];
  aiConfidence?: number;
  /** AI-authored classification used to decide which response UI is appropriate. */
  aiReplyKind?: AIReplyKind;
  linkPreviews?: LinkPreview[];
  attachments?: Attachment[];
  viaChannel?: 'email' | 'widget';
  emailVisibleText?: string;
  emailQuotedText?: string;
  emailHasQuotedContent?: boolean;
  emailProjectionConfidence?: 'high' | 'medium' | 'none';
  emailProjectionVersion?: number;
  /** Transient client-side state while a validated AI reply is being revealed. */
  isStreaming?: boolean;
  /** Stable local key retained while an optimistic message is reconciled with the server. */
  clientId?: string;
  /** Transient delivery state for an outgoing optimistic message. */
  deliveryStatus?: 'sending';
  isInternal: boolean;
  createdAt: string;
}

export interface AiSource {
  docId: string;
  title: string;
  snippet: string;
  confidence: number;
  language: string;
}

export interface Attachment {
  id?: string;
  fileKey: string;
  fileName: string;
  fileType: string;
  fileSize: number;
  url?: string;
}

export interface LinkPreview {
  url: string;
  title: string;
  description?: string;
  site_name?: string;
  image_url?: string;
  host: string;
}

export interface PendingAttachment {
  error?: string;
  id: string;
  fileName: string;
  fileType: string;
  fileSize: number;
  progress: number;
  status: 'uploading' | 'uploaded' | 'error';
  previewUrl?: string;
  attachmentId?: string;
}

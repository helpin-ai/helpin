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
  'resolved',
  'reopened',
  'closed',
] as const;

export type SystemEventType = (typeof SYSTEM_EVENT_TYPES)[number];

export interface Message {
  id: string;
  conversationId: string;
  role: 'customer' | 'agent' | 'ai' | 'system';
  content: string;
  senderId?: string;
  senderName?: string;
  senderAvatar?: string;
  systemEventType?: SystemEventType;
  sources?: AiSource[];
  aiConfidence?: number;
  linkPreviews?: LinkPreview[];
  attachments?: Attachment[];
  viaChannel?: 'email' | 'widget';
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
  id: string;
  fileName: string;
  fileType: string;
  fileSize: number;
  progress: number;
  status: 'uploading' | 'uploaded' | 'error';
  previewUrl?: string;
  attachmentId?: string;
}

import type { Message as SharedMessage, WidgetConfig as SharedWidgetConfig, PendingAttachment as SharedPendingAttachment, SystemEventType as SharedSystemEventType, AIReplyKind as SharedAIReplyKind } from '@helpin-ai/shared';
import { SYSTEM_EVENT_TYPES as SHARED_SYSTEM_EVENT_TYPES } from '@helpin-ai/shared';

// Re-export shared types
export type Message = SharedMessage;
export type PendingAttachment = SharedPendingAttachment;
export type SystemEventType = SharedSystemEventType;
export type AIReplyKind = SharedAIReplyKind;
export const SYSTEM_EVENT_TYPES = SHARED_SYSTEM_EVENT_TYPES;

export interface WidgetConfig extends Omit<SharedWidgetConfig, 'availableTeammates' | 'features' | 'availability'> {
  availableTeammates?: ActiveTeammate[];
  features: SharedWidgetConfig['features'] & {
    aiFirst?: boolean;
    escalationMessage?: string;
    forceIdentify?: boolean;
  };
  availability?: {
    isOnline: boolean;
    statusText: string;
    replyTimeText: string;
    outsideHoursMessage?: string;
    nextOnlineAt?: string;
    replyTimePreset?: import('@helpin-ai/shared').ReplyTimePreset;
    replyTimeMinutes?: number;
    specialNoticeText?: string;
    mailboxId?: string;
  };
}

// Shared nested types used directly in widget-core
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

export interface ActiveTeammate {
  userId: string;
  name: string;
  avatarUrl?: string;
  status?: 'online' | 'away' | 'offline';
}

export interface Conversation {
  id: string;
  subject: string;
  status: string;
  flowState?: string;
  aiState?: string;
  handoffState?: 'live' | 'busy' | 'after_hours';
  handoffStartedAt?: string;
  lastMessage?: string;
  lastMessageAt?: string;
  unreadCount?: number;
  activeTeammate?: ActiveTeammate;
}

export interface WidgetAdapter {
  getMessages(conversationId: string): Message[];
  onMessagesUpdate(cb: (messages: Message[]) => void): () => void;

  sendMessage(content: string, attachments?: File[]): Promise<void>;
  startConversation(customer: CustomerInfo): Promise<string>;
  markConversationAsRead(conversationId: string): void;
  sendTypingIndicator(isTyping: boolean): void;

  getConfig(): WidgetConfig;
}

export interface CustomerInfo {
  name?: string;
  email?: string;
  externalId?: string;
  metadata?: Record<string, unknown>;
}

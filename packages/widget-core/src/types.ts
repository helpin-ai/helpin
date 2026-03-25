import type { Message as SharedMessage, WidgetConfig as SharedWidgetConfig, PendingAttachment as SharedPendingAttachment } from '@helpin/shared';

// Re-export shared types
export type Message = SharedMessage;
export type PendingAttachment = SharedPendingAttachment;

export interface WidgetConfig extends SharedWidgetConfig {
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

import type { Message as SharedMessage, WidgetConfig as SharedWidgetConfig, PendingAttachment as SharedPendingAttachment } from '@helpin/shared';

// Re-export shared types
export type Message = SharedMessage;
export type WidgetConfig = SharedWidgetConfig;
export type PendingAttachment = SharedPendingAttachment;

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

export interface Conversation {
  id: string;
  subject: string;
  status: string;
  lastMessage?: string;
  lastMessageAt?: string;
  unreadCount?: number;
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

import type { Message as SharedMessage, WidgetConfig as SharedWidgetConfig } from '@helpin/shared';

// Re-export shared types
export type Message = SharedMessage;
export type WidgetConfig = SharedWidgetConfig;

// Shared nested types used directly in widget-core
export interface AiSource {
  docId: string;
  title: string;
  snippet: string;
  confidence: number;
  language: string;
}

export interface Attachment {
  fileKey: string;
  fileName: string;
  fileType: string;
  fileSize: number;
}

export interface WidgetAdapter {
  getMessages(conversationId: string): Message[];
  onMessagesUpdate(cb: (messages: Message[]) => void): () => void;

  sendMessage(content: string, attachments?: File[]): Promise<void>;
  startConversation(customer: CustomerInfo): Promise<string>;
  markAsRead(messageId: string): void;
  sendTypingIndicator(isTyping: boolean): void;

  getConfig(): WidgetConfig;
}

export interface CustomerInfo {
  name?: string;
  email?: string;
  externalId?: string;
  metadata?: Record<string, unknown>;
}

export interface WidgetAdapter {
  getMessages(conversationId: string): Message[];
  onMessagesUpdate(cb: (messages: Message[]) => void): () => void;

  sendMessage(content: string, attachments?: File[]): Promise<void>;
  startConversation(customer: CustomerInfo): Promise<string>;
  markAsRead(messageId: string): void;
  sendTypingIndicator(isTyping: boolean): void;

  getConfig(): WidgetConfig;
}

export interface Message {
  id: string;
  conversationId: string;
  role: 'customer' | 'agent' | 'ai' | 'system';
  content: string;
  senderId?: string;
  sources?: AiSource[];
  aiConfidence?: number;
  attachments?: Attachment[];
  isInternal: boolean;
  createdAt: string;
}

export interface CustomerInfo {
  name?: string;
  email?: string;
  externalId?: string;
  metadata?: Record<string, unknown>;
}

export interface WidgetConfig {
  workspaceId: string;
  branding: {
    primaryColor: string;
    logoUrl?: string;
    welcomeMessage: string;
    widgetPosition: 'bottom-right' | 'bottom-left';
  };
  features: {
    aiEnabled: boolean;
    fileUploads: boolean;
    preChatForm: boolean;
    csatRating: boolean;
  };
}

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

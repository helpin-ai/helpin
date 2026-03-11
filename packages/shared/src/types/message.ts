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

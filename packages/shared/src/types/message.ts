export interface Message {
  id: string;
  conversationId: string;
  role: 'customer' | 'agent' | 'ai' | 'system';
  content: string;
  senderId?: string;
  senderName?: string;
  senderAvatar?: string;
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

declare interface AiSource {
    docId: string;
    title: string;
    snippet: string;
    confidence: number;
    language: string;
}

declare interface Attachment {
    fileKey: string;
    fileName: string;
    fileType: string;
    fileSize: number;
}

export declare interface Conversation {
    id: string;
    workspaceId: string;
    title?: string;
    status: 'open' | 'pending' | 'resolved' | 'closed';
    priority: 'low' | 'medium' | 'high' | 'urgent';
    assignedTo?: string;
    customerName?: string;
    customerEmail?: string;
    createdAt: string;
    updatedAt: string;
    lastMessageAt: string;
}

export declare interface Message {
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

export declare interface Organization {
    id: string;
    name: string;
    slug: string;
    logoUrl?: string;
    plan: 'free' | 'pro' | 'enterprise';
    createdAt: string;
}

export declare interface User {
    id: string;
    email: string;
    name: string;
    avatarUrl?: string;
    isOnline: boolean;
    lastSeenAt: string;
    createdAt: string;
}

export declare interface WidgetConfig {
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

export declare interface Workspace {
    id: string;
    organizationId: string;
    name: string;
    slug: string;
    customDomain?: string;
    defaultLanguage: string;
    supportedLanguages: string[];
    branding: WorkspaceBranding;
    createdAt: string;
}

declare interface WorkspaceBranding {
    primaryColor: string;
    logoUrl?: string;
    welcomeMessage: string;
    widgetPosition: 'bottom-right' | 'bottom-left';
}

export { }

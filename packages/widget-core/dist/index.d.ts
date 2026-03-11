import { FunctionComponent } from 'preact';

export declare interface AiSource {
    docId: string;
    title: string;
    snippet: string;
    confidence: number;
    language: string;
}

export declare interface Attachment {
    fileKey: string;
    fileName: string;
    fileType: string;
    fileSize: number;
}

export declare const ChatWindow: FunctionComponent<ChatWindowProps>;

declare interface ChatWindowProps {
    config: WidgetConfig;
    messages: Message[];
    isOpen: boolean;
    onClose: () => void;
    onSendMessage: (content: string) => void;
    onQuickReply: (content: string) => void;
    showPreChatForm: boolean;
    onPreChatSubmit: (data: {
        name: string;
        email: string;
    }) => void;
    isTyping?: boolean;
    quickReplies?: string[];
}

export declare const ComposeBar: FunctionComponent<ComposeBarProps>;

declare interface ComposeBarProps {
    onSend: (content: string) => void;
    disabled?: boolean;
    placeholder?: string;
}

export declare const CsatRating: FunctionComponent<CsatRatingProps>;

declare interface CsatRatingProps {
    onSubmit: (rating: number, feedback?: string) => void;
}

export declare interface CustomerInfo {
    name?: string;
    email?: string;
    externalId?: string;
    metadata?: Record<string, unknown>;
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

export declare const MessageBubble: FunctionComponent<MessageBubbleProps>;

declare interface MessageBubbleProps {
    message: Message;
}

export declare const MessageList: FunctionComponent<MessageListProps>;

declare interface MessageListProps {
    messages: Message[];
}

export declare const PreChatForm: FunctionComponent<PreChatFormProps>;

declare interface PreChatFormProps {
    requireEmail?: boolean;
    requireName?: boolean;
    welcomeMessage?: string;
    onSubmit: (data: {
        name: string;
        email: string;
    }) => void;
}

export declare const QuickReplies: FunctionComponent<QuickRepliesProps>;

declare interface QuickRepliesProps {
    replies: string[];
    onSelect: (reply: string) => void;
}

export declare const StreamingText: FunctionComponent<StreamingTextProps>;

declare interface StreamingTextProps {
    text: string;
    isStreaming: boolean;
    onComplete?: () => void;
}

export declare const TypingIndicator: FunctionComponent<TypingIndicatorProps>;

declare interface TypingIndicatorProps {
    label?: string;
}

export declare interface WidgetAdapter {
    getMessages(conversationId: string): Message[];
    onMessagesUpdate(cb: (messages: Message[]) => void): () => void;
    sendMessage(content: string, attachments?: File[]): Promise<void>;
    startConversation(customer: CustomerInfo): Promise<string>;
    markAsRead(messageId: string): void;
    sendTypingIndicator(isTyping: boolean): void;
    getConfig(): WidgetConfig;
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

export declare const WidgetHeader: FunctionComponent<WidgetHeaderProps>;

declare interface WidgetHeaderProps {
    workspaceName: string;
    logoUrl?: string;
    onClose: () => void;
    brandColor?: string;
    showBranding?: boolean;
}

export declare const WidgetLauncher: FunctionComponent<WidgetLauncherProps>;

declare interface WidgetLauncherProps {
    onClick: () => void;
    isOpen: boolean;
    unreadCount?: number;
    brandColor?: string;
}

export { }

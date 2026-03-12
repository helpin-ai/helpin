import { FunctionComponent } from 'preact';
import { Message as Message_2 } from '@helpin/shared';
import { WidgetConfig as WidgetConfig_2 } from '@helpin/shared';

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

export declare const BottomNav: FunctionComponent<BottomNavProps>;

declare interface BottomNavProps {
    activeView: WidgetView;
    onNavigate: (view: WidgetView) => void;
    brandColor?: string;
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
    initialView?: WidgetView;
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

export declare const HelpView: FunctionComponent<HelpViewProps>;

declare interface HelpViewProps {
    config: WidgetConfig;
    onNavigate: (view: 'messages') => void;
}

export declare const HomeView: FunctionComponent<HomeViewProps>;

declare interface HomeViewProps {
    config: WidgetConfig;
    onSendMessage: (content: string) => void;
    onNavigate: (view: 'messages' | 'help') => void;
    showPreChatForm: boolean;
    onPreChatSubmit: (data: {
        name: string;
        email: string;
    }) => void;
}

declare type LauncherIcon = 'chat_bubble' | 'question_mark' | 'help';

export declare type Message = Message_2;

export declare const MessageBubble: FunctionComponent<MessageBubbleProps>;

declare interface MessageBubbleProps {
    message: Message;
}

export declare const MessageList: FunctionComponent<MessageListProps>;

declare interface MessageListProps {
    messages: Message[];
}

export declare const MessagesView: FunctionComponent<MessagesViewProps>;

declare interface MessagesViewProps {
    config: WidgetConfig;
    messages: Message[];
    onSendMessage: (content: string) => void;
    onQuickReply: (content: string) => void;
    isTyping?: boolean;
    quickReplies?: string[];
    hasConversation: boolean;
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
    charDelayMs?: number;
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

export declare type WidgetConfig = WidgetConfig_2;

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
    buttonColor?: string;
    buttonIconColor?: string;
    icon?: LauncherIcon;
}

export declare type WidgetView = 'home' | 'messages' | 'help';

export { }

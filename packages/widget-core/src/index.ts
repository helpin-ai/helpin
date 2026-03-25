import { h, render } from 'preact';
import type { ActiveTeammate, Message, Conversation, WidgetConfig } from './types';
import type { WidgetView } from './components/BottomNav';
import { ChatWindow } from './components/ChatWindow';
import { WidgetLauncher } from './components/WidgetLauncher';

export type {
  WidgetAdapter,
  Message,
  Conversation,
  ActiveTeammate,
  CustomerInfo,
  WidgetConfig,
  AiSource,
  Attachment,
  PendingAttachment,
} from './types';

export type { WidgetView } from './components/BottomNav';

export { ChatWindow } from './components/ChatWindow';
export { MessageList } from './components/MessageList';
export { MessageBubble } from './components/MessageBubble';
export { ComposeBar } from './components/ComposeBar';
export { WidgetHeader } from './components/WidgetHeader';
export { WidgetLauncher } from './components/WidgetLauncher';
export { PreChatForm } from './components/PreChatForm';
export { QuickReplies } from './components/QuickReplies';
export { TypingIndicator } from './components/TypingIndicator';
export { CsatRating } from './components/CsatRating';
export { StreamingText } from './components/StreamingText';
export { BottomNav } from './components/BottomNav';
export { HomeView } from './components/HomeView';
export { MessagesView } from './components/MessagesView';
export { HelpView } from './components/HelpView';
export { HelpSpaceView } from './components/HelpSpaceView';
export { HelpCollectionView } from './components/HelpCollectionView';
export { HelpArticleView } from './components/HelpArticleView';
export { ConversationView } from './components/ConversationView';
export { ConversationListView } from './components/ConversationListView';
export { ImageLightbox } from './components/ImageLightbox';
export { loadEmojiCatalog } from './components/emoji-loader';
export type { EmojiCatalog } from './components/emoji-catalog';

// ─── Mount API ───────────────────────────────────────────────
// Consumers call mountWidget() instead of importing preact directly.
// This guarantees a single preact instance — no __H dual-instance errors.

export interface MountWidgetOptions {
  config: WidgetConfig;
  messages?: Message[];
  isOpen?: boolean;
  onClose?: () => void;
  onSendMessage?: (content: string, attachmentIds?: string[]) => void;
  onSendMessageFromHome?: (content: string) => void;
  onUploadAttachment?: (file: File, localId: string) => Promise<{ attachmentId: string; url: string } | null>;
  onQuickReply?: (content: string) => void;
  onTyping?: (content: string) => void;
  showPreChatForm?: boolean;
  onPreChatSubmit?: (data: { phone: string; email: string }) => void;
  isTyping?: boolean;
  isAIThinking?: boolean;
  onEscalateToHuman?: () => void;
  typingAgentName?: string;
  typingAgentAvatar?: string;
  activeTeammate?: ActiveTeammate;
  quickReplies?: string[];
  initialView?: WidgetView;
  showLauncher?: boolean;
  onLauncherClick?: () => void;
  unreadCount?: number;
  connectionStatus?: 'idle' | 'connecting' | 'connected' | 'disconnected' | 'failed';
  conversations?: Conversation[];
  activeConversation?: Conversation;
  onSelectConversation?: (conversationId: string) => void;
  onStartNewConversation?: () => void;
  onViewChange?: (view: WidgetView) => void;
  isConversationExpanded?: boolean;
  onToggleConversationExpanded?: () => void;
  transcriptEmail?: string;
  onRequestTranscript?: (email?: string) => Promise<{ success: boolean; message: string }>;
  widgetKey?: string;
  host?: string;
  openArticleRequest?: {
    key: number;
    articleSlug: string;
  };
  onImageClick?: (src: string, alt: string) => void;
}

export function mountWidget(container: HTMLElement, options: MountWidgetOptions): void {
  const {
    config,
    messages = [],
    isOpen = true,
    onClose = () => {},
    onSendMessage = () => {},
    onSendMessageFromHome = onSendMessage,
    onUploadAttachment,
    onQuickReply = () => {},
    onTyping,
    showPreChatForm = false,
    onPreChatSubmit = () => {},
    isTyping = false,
    isAIThinking = false,
    onEscalateToHuman,
    typingAgentName,
    typingAgentAvatar,
    activeTeammate,
    quickReplies = [],
    initialView = 'home',
    showLauncher = true,
    onLauncherClick,
    unreadCount = 0,
    conversations = [],
    activeConversation,
    onSelectConversation = () => {},
    onStartNewConversation = () => {},
    onViewChange,
    isConversationExpanded = false,
    onToggleConversationExpanded,
    transcriptEmail,
    onRequestTranscript,
    widgetKey,
    host,
    openArticleRequest,
    onImageClick,
  } = options;

  const tree = h(
    'div',
    { className: 'helpin-widget', style: { width: '100%', height: '100%' } },
    h(ChatWindow, {
      config,
      messages,
      isOpen,
      onClose,
      onSendMessage,
      onSendMessageFromHome,
      onUploadAttachment,
      onQuickReply,
      onTyping,
      showPreChatForm,
      onPreChatSubmit,
      isTyping,
      isAIThinking,
      onEscalateToHuman,
      typingAgentName,
      typingAgentAvatar,
      activeTeammate,
      quickReplies,
      initialView,
      conversations,
      activeConversation,
      onSelectConversation,
      onStartNewConversation,
      onViewChange,
      isConversationExpanded,
      onToggleConversationExpanded,
      transcriptEmail,
      onRequestTranscript,
      widgetKey,
      host,
      openArticleRequest,
      onImageClick,
    }),
    showLauncher
      ? h(WidgetLauncher, {
          onClick: onLauncherClick || onClose,
          isOpen,
          unreadCount,
          brandColor: config.branding?.primaryColor || '#6366f1',
          buttonColor: config.branding?.buttonColor,
          buttonIconColor: config.branding?.buttonIconColor,
          icon: config.branding?.launcherIcon || 'chat_bubble',
        })
      : null,
  );

  render(tree, container);
}

export function unmountWidget(container: HTMLElement): void {
  render(null, container);
}

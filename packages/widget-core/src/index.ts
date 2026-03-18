import { h, render } from 'preact';
import type { Message, Conversation, WidgetConfig } from './types';
import type { WidgetView } from './components/BottomNav';
import { ChatWindow } from './components/ChatWindow';
import { WidgetLauncher } from './components/WidgetLauncher';

export type {
  WidgetAdapter,
  Message,
  Conversation,
  CustomerInfo,
  WidgetConfig,
  AiSource,
  Attachment,
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

// ─── Mount API ───────────────────────────────────────────────
// Consumers call mountWidget() instead of importing preact directly.
// This guarantees a single preact instance — no __H dual-instance errors.

export interface MountWidgetOptions {
  config: WidgetConfig;
  messages?: Message[];
  isOpen?: boolean;
  onClose?: () => void;
  onSendMessage?: (content: string) => void;
  onSendMessageFromHome?: (content: string) => void;
  onQuickReply?: (content: string) => void;
  onTyping?: (content: string) => void;
  showPreChatForm?: boolean;
  onPreChatSubmit?: (data: { phone: string; email: string }) => void;
  isTyping?: boolean;
  typingAgentName?: string;
  typingAgentAvatar?: string;
  quickReplies?: string[];
  initialView?: WidgetView;
  showLauncher?: boolean;
  onLauncherClick?: () => void;
  unreadCount?: number;
  connectionStatus?: 'idle' | 'connecting' | 'connected' | 'disconnected' | 'failed';
  conversations?: Conversation[];
  onSelectConversation?: (conversationId: string) => void;
  onStartNewConversation?: () => void;
  onViewChange?: (view: WidgetView) => void;
  widgetKey?: string;
  host?: string;
  openArticleRequest?: {
    key: number;
    articleSlug: string;
  };
}

export function mountWidget(container: HTMLElement, options: MountWidgetOptions): void {
  const {
    config,
    messages = [],
    isOpen = true,
    onClose = () => {},
    onSendMessage = () => {},
    onSendMessageFromHome = onSendMessage,
    onQuickReply = () => {},
    onTyping,
    showPreChatForm = false,
    onPreChatSubmit = () => {},
    isTyping = false,
    typingAgentName,
    typingAgentAvatar,
    quickReplies = [],
    initialView = 'home',
    showLauncher = true,
    onLauncherClick,
    unreadCount = 0,
    conversations = [],
    onSelectConversation = () => {},
    onStartNewConversation = () => {},
    onViewChange,
    widgetKey,
    host,
    openArticleRequest,
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
      onQuickReply,
      onTyping,
      showPreChatForm,
      onPreChatSubmit,
      isTyping,
      typingAgentName,
      typingAgentAvatar,
      quickReplies,
      initialView,
      conversations,
      onSelectConversation,
      onStartNewConversation,
      onViewChange,
      widgetKey,
      host,
      openArticleRequest,
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

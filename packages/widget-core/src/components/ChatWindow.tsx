import { FunctionComponent } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import type { Message, Conversation, WidgetConfig } from '../types';
import { BottomNav, type WidgetBaseView, WidgetView } from './BottomNav';
import { HomeView } from './HomeView';
import { MessagesView } from './MessagesView';
import { HelpView } from './HelpView';
import { HelpSpaceView } from './HelpSpaceView';
import { HelpCollectionView } from './HelpCollectionView';
import { HelpArticleView } from './HelpArticleView';
import { ConversationView } from './ConversationView';
import { ConversationListView } from './ConversationListView';
import { XIcon } from './icons';

type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'failed';

interface ChatWindowProps {
  config: WidgetConfig;
  messages: Message[];
  isOpen: boolean;
  onClose: () => void;
  onSendMessage: (content: string) => void;
  onSendMessageFromHome?: (content: string) => void;
  onQuickReply: (content: string) => void;
  onTyping?: (content: string) => void;
  showPreChatForm: boolean;
  onPreChatSubmit: (data: { phone: string; email: string }) => void;
  isTyping?: boolean;
  isAIThinking?: boolean;
  typingAgentName?: string;
  typingAgentAvatar?: string;
  quickReplies?: string[];
  initialView?: WidgetView;
  connectionStatus?: ConnectionStatus;
  onRetryConnection?: () => void;
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

export const ChatWindow: FunctionComponent<ChatWindowProps> = ({
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
  isTyping = false,
  isAIThinking = false,
  typingAgentName,
  typingAgentAvatar,
  quickReplies = [],
  initialView = 'home',
  connectionStatus = 'idle',
  onRetryConnection,
  conversations = [],
  onSelectConversation = () => {},
  onStartNewConversation,
  onViewChange,
  widgetKey,
  host,
  openArticleRequest,
}) => {
  const initialPreviousView: WidgetBaseView =
    initialView === 'messages' || initialView === 'help' || initialView === 'home'
      ? initialView
      : initialView === 'conversation'
        ? 'home'
        : 'help';
  const [activeView, setActiveView] = useState<WidgetView>(initialView);
  const [previousView, setPreviousView] = useState<WidgetBaseView>(initialPreviousView);
  const [activeHelpSpaceSlug, setActiveHelpSpaceSlug] = useState<string | null>(null);
  const [activeCollectionSlug, setActiveCollectionSlug] = useState<string | null>(null);
  const [activeArticleSlug, setActiveArticleSlug] = useState<string | null>(null);
  const [shouldRender, setShouldRender] = useState(isOpen);
  const [isVisible, setIsVisible] = useState(isOpen);

  // Sync activeView when initialView prop changes (e.g. first-open → conversation)
  useEffect(() => {
    setActiveView(initialView);
  }, [initialView]);

  useEffect(() => {
    if (!openArticleRequest) {
      return;
    }

    setActiveArticleSlug(openArticleRequest.articleSlug);
    setActiveCollectionSlug(null);
    setActiveHelpSpaceSlug(null);
    setActiveView('help-article');
    onViewChange?.('help-article');
  }, [onViewChange, openArticleRequest]);

  useEffect(() => {
    let frameId: number | undefined;
    let timeoutId: ReturnType<typeof globalThis.setTimeout> | undefined;

    if (isOpen) {
      setShouldRender(true);
      if (typeof window !== 'undefined') {
        frameId = window.requestAnimationFrame(() => setIsVisible(true));
      } else {
        setIsVisible(true);
      }
    } else if (shouldRender) {
      setIsVisible(false);
      timeoutId = globalThis.setTimeout(() => setShouldRender(false), 220);
    }

    return () => {
      if (frameId !== undefined && typeof window !== 'undefined') {
        window.cancelAnimationFrame(frameId);
      }
      if (timeoutId !== undefined) {
        globalThis.clearTimeout(timeoutId);
      }
    };
  }, [isOpen, shouldRender]);

  if (!shouldRender) return null;

  const position = config.branding?.widgetPosition || 'bottom-right';
  const brandColor = config.branding?.primaryColor || '#6366f1';
  const showBranding = config.branding?.showBranding ?? true;
  const colorScheme = config.branding?.colorScheme || 'light';
  const helpSpaces = config.helpSpaces ?? [];
  const activeHelpSpace = helpSpaces.find((space) => space.slug === activeHelpSpaceSlug) || null;
  const positionClass = position.includes('left')
    ? 'helpin-chat-window--left'
    : 'helpin-chat-window--right';
  const expandedClass = activeView === 'help-article' ? 'helpin-chat-window--expanded' : '';

  const handleNavigate = (view: WidgetBaseView) => {
    if (view !== 'help') {
      setActiveHelpSpaceSlug(null);
      setActiveCollectionSlug(null);
      setActiveArticleSlug(null);
    }
    setActiveView(view as WidgetView);
    onViewChange?.(view as WidgetView);
  };

  const handleStartConversation = (fromView: WidgetBaseView) => {
    setPreviousView(fromView);
    setActiveView('conversation');
  };

  const handleSendFromHome = (content: string) => {
    (onSendMessageFromHome ?? onSendMessage)(content);
    handleStartConversation('home');
  };

  const handleOpenHelpSpace = (spaceSlug: string) => {
    setActiveHelpSpaceSlug(spaceSlug);
    setActiveCollectionSlug(null);
    setActiveArticleSlug(null);
    setActiveView('help-space');
    onViewChange?.('help-space');
  };

  const handleOpenHelpCollection = (collectionSlug: string) => {
    setActiveHelpSpaceSlug((current) => current ?? helpSpaces[0]?.slug ?? null);
    setActiveCollectionSlug(collectionSlug);
    setActiveArticleSlug(null);
    setActiveView('help-collection');
    onViewChange?.('help-collection');
  };

  const handleOpenHelpArticle = (articleSlug: string) => {
    setActiveArticleSlug(articleSlug);
    setActiveView('help-article');
    onViewChange?.('help-article');
  };

  return (
    <div
      className={`helpin-chat-window ${positionClass} ${expandedClass} ${isVisible ? 'helpin-chat-window--visible' : 'helpin-chat-window--hidden'} helpin-theme-${colorScheme}`}
    >
      {/* Close button — hidden in conversation view (has its own) and messages view with conversation list */}
      {activeView !== 'conversation' && !(activeView === 'messages' && conversations.length > 0) && (
        <button className="helpin-window-close" onClick={onClose} aria-label="Close">
          <XIcon size={18} />
        </button>
      )}

      {/* Connection status banner */}
      {connectionStatus === 'connecting' && (
        <div className="helpin-connection-banner helpin-connection-banner--connecting">
          Connecting...
        </div>
      )}
      {connectionStatus === 'failed' && (
        <div className="helpin-connection-banner helpin-connection-banner--failed">
          <span>Unable to connect. Support may be unavailable.</span>
          {onRetryConnection && (
            <button className="helpin-connection-retry" onClick={onRetryConnection}>
              Retry
            </button>
          )}
        </div>
      )}

      {/* View content */}
      <div className="helpin-view-container">
        {activeView === 'home' && (
          <HomeView
            config={config}
            onSendMessage={handleSendFromHome}
            onNavigate={(view) => {
              if (view === 'conversation') {
                setPreviousView('home');
                setActiveView('conversation');
                // Start a fresh conversation (reset active conversation in SDK)
                if (onStartNewConversation) {
                  onStartNewConversation();
                }
                return;
              }
              handleNavigate(view);
            }}
          />
        )}
        {activeView === 'conversation' && (
          <ConversationView
            config={config}
            messages={messages}
            onSendMessage={onSendMessage}
            onTyping={onTyping}
            isTyping={isTyping}
            isAIThinking={isAIThinking}
            typingAgentName={typingAgentName}
            typingAgentAvatar={typingAgentAvatar}
            onBack={() => setActiveView(previousView)}
            onClose={onClose}
            showPreChatForm={showPreChatForm}
            onPreChatSubmit={onPreChatSubmit}
          />
        )}
        {activeView === 'messages' && (
          conversations.length > 0 ? (
            <ConversationListView
              config={config}
              conversations={conversations}
              onSelectConversation={(id) => {
                onSelectConversation(id);
                setPreviousView('messages');
                setActiveView('conversation');
              }}
              onStartConversation={onStartNewConversation || (() => handleStartConversation('messages'))}
              onClose={onClose}
            />
          ) : (
            <MessagesView
              config={config}
              messages={messages}
              onSendMessage={onSendMessage}
              onQuickReply={onQuickReply}
              onStartConversation={() => handleStartConversation('messages')}
              isTyping={isTyping}
              quickReplies={quickReplies}
              hasConversation={messages.length > 0}
            />
          )
        )}
        {activeView === 'help' && (
          <HelpView
            config={config}
            host={host}
            widgetKey={widgetKey}
            onContact={() => handleStartConversation('help')}
            onSelectSpace={handleOpenHelpSpace}
            onSelectCollection={handleOpenHelpCollection}
          />
        )}
        {activeView === 'help-space' && activeHelpSpace && host && widgetKey && (
          <HelpSpaceView
            host={host}
            widgetKey={widgetKey}
            space={activeHelpSpace}
            showBack={helpSpaces.length > 1}
            onBack={() => handleNavigate('help')}
            onSelectCollection={handleOpenHelpCollection}
          />
        )}
        {activeView === 'help-collection' && activeCollectionSlug && host && widgetKey && (
          <HelpCollectionView
            host={host}
            widgetKey={widgetKey}
            collectionSlug={activeCollectionSlug}
            onBack={() => {
              setActiveArticleSlug(null);
              if (helpSpaces.length > 1 && activeHelpSpaceSlug) {
                setActiveView('help-space');
                onViewChange?.('help-space');
                return;
              }
              setActiveView('help');
              onViewChange?.('help');
            }}
            onSelectArticle={handleOpenHelpArticle}
          />
        )}
        {activeView === 'help-article' && activeArticleSlug && host && widgetKey && (
          <HelpArticleView
            host={host}
            widgetKey={widgetKey}
            articleSlug={activeArticleSlug}
            onBack={() => {
              if (activeCollectionSlug) {
                setActiveView('help-collection');
                onViewChange?.('help-collection');
                return;
              }
              setActiveArticleSlug(null);
              setActiveView('help');
              onViewChange?.('help');
            }}
          />
        )}
      </div>

      {/* Powered by footer */}
      {showBranding && activeView !== 'conversation' && activeView !== 'messages' && (
        <div className="helpin-powered-by">
          <span>Powered by</span>
          <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor" className="helpin-powered-by-icon">
            <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" />
          </svg>
          <span className="helpin-powered-by-name">Helpin</span>
        </div>
      )}

      {/* Bottom navigation */}
      {activeView !== 'conversation' && (
        <BottomNav
          activeView={activeView}
          onNavigate={handleNavigate}
          brandColor={brandColor}
        />
      )}
    </div>
  );
};

import { FunctionComponent } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import type { ActiveTeammate, Message, Conversation, WidgetConfig } from '../types';
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
import helpinMarkUrl from '../assets/helpin-mark.svg';

type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'failed';
const HELPIN_BRANDING_URL = 'https://helpin.ai/?utm_source=helpin_widget&utm_medium=widget&utm_campaign=powered_by';

interface ChatWindowProps {
  config: WidgetConfig;
  messages: Message[];
  isOpen: boolean;
  onClose: () => void;
  onSendMessage: (content: string, attachmentIds?: string[]) => void;
  onSendMessageFromHome?: (content: string) => void;
  onUploadAttachment?: (file: File, localId: string) => Promise<{ attachmentId: string; url: string } | null>;
  onQuickReply: (content: string) => void;
  onTyping?: (content: string) => void;
  showPreChatForm: boolean;
  onPreChatSubmit: (data: { phone: string; email: string }) => void;
  isTyping?: boolean;
  isAIThinking?: boolean;
  typingAgentName?: string;
  typingAgentAvatar?: string;
  activeTeammate?: ActiveTeammate;
  onEscalateToHuman?: () => void;
  quickReplies?: string[];
  initialView?: WidgetView;
  connectionStatus?: ConnectionStatus;
  onRetryConnection?: () => void;
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

export const ChatWindow: FunctionComponent<ChatWindowProps> = ({
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
  isTyping = false,
  isAIThinking = false,
  typingAgentName,
  typingAgentAvatar,
  activeTeammate,
  onEscalateToHuman,
  quickReplies = [],
  initialView = 'home',
  connectionStatus = 'idle',
  onRetryConnection,
  conversations = [],
  activeConversation,
  onSelectConversation = () => {},
  onStartNewConversation,
  onViewChange,
  isConversationExpanded = false,
  onToggleConversationExpanded,
  transcriptEmail,
  onRequestTranscript,
  widgetKey,
  host,
  openArticleRequest,
  onImageClick,
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
  const [humanSupportRequested, setHumanSupportRequested] = useState(false);

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
  const homeTeammates = (config.availableTeammates ?? []).slice(0, 4);

  // Enrich conversations with activeTeammate fallback.
  // The SDK may strip activeTeammate when refreshing the list, so we fall back to:
  // 1. The current activeTeammate prop (if this is the active conversation)
  // 2. The first available teammate from config
  const fallbackTeammate = homeTeammates.length > 0 ? homeTeammates[0] : undefined;
  const enrichedConversations = conversations.map((conv) => ({
    ...conv,
    activeTeammate: conv.activeTeammate
      || (activeConversation?.id === conv.id ? activeTeammate : undefined)
      || fallbackTeammate,
  }));
  const enrichedRecentConversation = enrichedConversations.length > 0 ? enrichedConversations[0] : undefined;
  const positionClass = position.includes('left')
    ? 'helpin-chat-window--left'
    : 'helpin-chat-window--right';
  const expandedClass =
    activeView === 'help-article' || (activeView === 'conversation' && isConversationExpanded)
      ? 'helpin-chat-window--expanded'
      : '';

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
    setHumanSupportRequested(false);
    setActiveView('conversation');
  };

  const handleStartNewConversation = (fromView: WidgetBaseView) => {
    handleStartConversation(fromView);
    onStartNewConversation?.();
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
      {activeView === 'home' && (
        <div className="helpin-window-actions helpin-window-actions--home">
          <button
            className="helpin-window-close helpin-window-close--home"
            onClick={onClose}
            aria-label="Close"
          >
            <XIcon size={18} />
          </button>
        </div>
      )}

      {/* Close button — hidden in conversation view (has its own) and messages view with conversation list */}
      {activeView !== 'home' && activeView !== 'conversation' && activeView !== 'help' && !(activeView === 'messages' && conversations.length > 0) && (
        <button
          className="helpin-window-close"
          onClick={onClose}
          aria-label="Close"
        >
          <XIcon size={18} />
        </button>
      )}

      {/* Connection status banner */}
      {connectionStatus === 'connecting' && (
        <div className="helpin-connection-banner helpin-connection-banner--connecting">
          Connecting...
        </div>
      )}
      {connectionStatus === 'disconnected' && (
        <div className="helpin-connection-banner helpin-connection-banner--disconnected">
          Connection lost. Reconnecting...
        </div>
      )}
      {connectionStatus === 'failed' && (
        <div className="helpin-connection-banner helpin-connection-banner--failed">
          <span>We've been offline for a while. We'll keep trying in the background, or reconnect now.</span>
          {onRetryConnection && (
            <button className="helpin-connection-retry" onClick={onRetryConnection}>
              Reconnect
            </button>
          )}
        </div>
      )}

      {/* View content */}
      <div className="helpin-view-container">
        {activeView === 'home' && (
          <HomeView
            config={config}
            teammates={homeTeammates}
            recentConversation={enrichedRecentConversation}
            onSendMessage={handleSendFromHome}
            onNavigate={(view) => {
              if (view === 'conversation') {
                handleStartNewConversation('home');
                return;
              }
              handleNavigate(view);
            }}
            onSelectConversation={(id) => {
              onSelectConversation(id);
              setActiveView('conversation');
              onViewChange?.('conversation');
            }}
          />
        )}
        {activeView === 'conversation' && (
          <ConversationView
            config={config}
            conversation={activeConversation}
            messages={messages}
            activeTeammate={activeTeammate}
            onSendMessage={onSendMessage}
            onUploadAttachment={onUploadAttachment}
            onTyping={onTyping}
            isTyping={isTyping}
            isAIThinking={isAIThinking}
            onEscalateToHuman={onEscalateToHuman ? () => {
              setHumanSupportRequested(true);
              onEscalateToHuman();
            } : undefined}
            showHumanAvailability={humanSupportRequested}
            typingAgentName={typingAgentName}
            typingAgentAvatar={typingAgentAvatar}
            onBack={() => setActiveView(previousView)}
            onClose={onClose}
            isExpanded={isConversationExpanded}
            onToggleExpanded={onToggleConversationExpanded}
            transcriptEmail={transcriptEmail}
            onRequestTranscript={onRequestTranscript}
            showPreChatForm={showPreChatForm}
            onPreChatSubmit={onPreChatSubmit}
            onImageClick={onImageClick}
            connectionStatus={connectionStatus}
          />
        )}
        {activeView === 'messages' && (
          conversations.length > 0 ? (
            <ConversationListView
              config={config}
              conversations={enrichedConversations}
              onSelectConversation={(id) => {
                onSelectConversation(id);
                setPreviousView('messages');
                setActiveView('conversation');
              }}
              onStartConversation={() => handleStartNewConversation('messages')}
              onClose={onClose}
            />
          ) : (
            <MessagesView
              config={config}
              messages={messages}
              onSendMessage={onSendMessage}
              onQuickReply={onQuickReply}
              onStartConversation={() => handleStartNewConversation('messages')}
              isTyping={isTyping}
              quickReplies={quickReplies}
              hasConversation={messages.length > 0}
              connectionStatus={connectionStatus}
            />
          )
        )}
        {activeView === 'help' && (
          <HelpView
            config={config}
            host={host}
            widgetKey={widgetKey}
            onClose={onClose}
            onContact={() => handleStartNewConversation('help')}
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
        <a
          href={HELPIN_BRANDING_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="helpin-powered-by"
        >
          <span>Powered by</span>
          <img src={helpinMarkUrl} alt="" aria-hidden="true" className="helpin-powered-by-icon" />
          <span className="helpin-powered-by-name">Helpin</span>
        </a>
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

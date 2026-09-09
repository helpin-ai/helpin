import { FunctionComponent } from 'preact';
import { useEffect, useRef, useState } from 'preact/hooks';
import type { ActiveTeammate, Message, Conversation, WidgetConfig } from '../types';
import { BottomNav, type WidgetBaseView, WidgetView } from './BottomNav';
import { HomeView } from './HomeView';
import { MessagesView } from './MessagesView';
import { HelpView } from './HelpView';
import { HelpSpaceView } from './HelpSpaceView';
import { HelpCollectionView } from './HelpCollectionView';
import {
  computeHelpCollectionBackTarget,
  pushHelpCollectionOnDrilldown,
} from './helpNavigationStack';
import { HelpArticleView } from './HelpArticleView';
import { ConversationView } from './ConversationView';
import { ConversationListView } from './ConversationListView';
import { BrandAttribution } from './BrandAttribution';
import { XIcon } from './icons';

type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'failed';
type ResolvedColorScheme = 'light' | 'dark';

const getSystemColorScheme = (): ResolvedColorScheme => {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return 'light';
  }

  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
};

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
  contactCaptureCompleted?: boolean;
  onCaptureEmail?: (email: string) => Promise<void>;
  onPreChatSubmit: (data: { phone: string; email: string }) => void;
  isTyping?: boolean;
  isAIThinking?: boolean;
  aiProgressLabel?: string;
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
  /** Visitor's durable browser id, forwarded for search attribution. */
  anonymousId?: string;
  host?: string;
  openArticleRequest?: {
    key: number;
    articleKey?: string;
    articleSlug?: string;
  };
  onImageClick?: (src: string, alt: string) => void;
  onAnswerFeedback?: (messageId: string, helpful: boolean) => void;
  queuedMessageCount?: number;
  csatSubmitted?: boolean;
  onCsatSubmit?: (rating: number, feedback?: string) => void;
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
  contactCaptureCompleted = false,
  onPreChatSubmit,
  onCaptureEmail,
  isTyping = false,
  isAIThinking = false,
  aiProgressLabel,
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
  anonymousId,
  host,
  openArticleRequest,
  onImageClick,
  onAnswerFeedback,
  queuedMessageCount = 0,
  csatSubmitted = false,
  onCsatSubmit,
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
  const [activeArticleKey, setActiveArticleKey] = useState<string | null>(null);
  const handledArticleRequestKeyRef = useRef<number | null>(null);
  const onViewChangeRef = useRef(onViewChange);
  // Breadcrumb stack of ancestor collection slugs the user drilled
  // through to reach activeCollectionSlug, oldest-first. Pop on back
  // to walk up the tree one level at a time. Empty when the user is
  // viewing a top-level collection or has navigated directly via the
  // help-space list.
  const [helpCollectionStack, setHelpCollectionStack] = useState<string[]>([]);
  const [shouldRender, setShouldRender] = useState(isOpen);
  const [isVisible, setIsVisible] = useState(isOpen);
  const [humanSupportRequested, setHumanSupportRequested] = useState(false);
  const configuredColorScheme = config.branding?.colorScheme || 'light';
  const [systemColorScheme, setSystemColorScheme] = useState<ResolvedColorScheme>(getSystemColorScheme);
  const colorScheme = configuredColorScheme === 'system'
    ? systemColorScheme
    : configuredColorScheme;

  // Sync activeView when initialView prop changes (e.g. first-open → conversation)
  useEffect(() => {
    setActiveView(initialView);
  }, [initialView]);

  useEffect(() => {
    onViewChangeRef.current = onViewChange;
  }, [onViewChange]);

  useEffect(() => {
    setHumanSupportRequested(false);
  }, [activeConversation?.id]);

  useEffect(() => {
    if (
      configuredColorScheme !== 'system'
      || typeof window === 'undefined'
      || typeof window.matchMedia !== 'function'
    ) {
      return;
    }

    const colorSchemeQuery = window.matchMedia('(prefers-color-scheme: dark)');
    const syncColorScheme = () => {
      setSystemColorScheme(colorSchemeQuery.matches ? 'dark' : 'light');
    };

    syncColorScheme();
    if (typeof colorSchemeQuery.addEventListener === 'function') {
      colorSchemeQuery.addEventListener('change', syncColorScheme);
      return () => colorSchemeQuery.removeEventListener('change', syncColorScheme);
    }

    colorSchemeQuery.addListener(syncColorScheme);
    return () => colorSchemeQuery.removeListener(syncColorScheme);
  }, [configuredColorScheme]);

  useEffect(() => {
    const requestedArticleKey = openArticleRequest?.articleKey ?? openArticleRequest?.articleSlug;
    const requestKey = openArticleRequest?.key;
    if (!requestedArticleKey || requestKey === undefined || handledArticleRequestKeyRef.current === requestKey) {
      return;
    }

    // openArticleRequest is a command, not persistent navigation state. The
    // SDK can rerender the widget with the same request object after a user
    // clicks Back or Messages; consume each key once so that rerender cannot
    // force the article back onto the screen.
    handledArticleRequestKeyRef.current = requestKey;
    setActiveArticleKey(requestedArticleKey);
    setActiveCollectionSlug(null);
    setActiveHelpSpaceSlug(null);
    setActiveView('help-article');
    onViewChangeRef.current?.('help-article');
  }, [openArticleRequest?.articleKey, openArticleRequest?.articleSlug, openArticleRequest?.key]);

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
      setActiveArticleKey(null);
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
    setActiveArticleKey(null);
    setHelpCollectionStack([]);
    setActiveView('help-space');
    onViewChange?.('help-space');
  };

  // handleOpenHelpCollection opens a collection view. When the user
  // is already on a collection page (help-collection view) and clicks
  // a child tile, we push the current slug onto the breadcrumb stack
  // so the back button can walk up the tree one level at a time.
  // Direct navigation from home or help-space resets the stack so
  // subsequent back navigation doesn't surface unrelated ancestors.
  const handleOpenHelpCollection = (collectionSlug: string) => {
    setActiveHelpSpaceSlug((current) => current ?? helpSpaces[0]?.slug ?? null);
    setHelpCollectionStack(
      pushHelpCollectionOnDrilldown(
        helpCollectionStack,
        activeView,
        activeCollectionSlug,
        collectionSlug,
      ),
    );
    setActiveCollectionSlug(collectionSlug);
    setActiveArticleKey(null);
    setActiveView('help-collection');
    onViewChange?.('help-collection');
  };

  // handleBackFromHelpCollection pops the nearest ancestor from the
  // breadcrumb stack and opens it. When the stack is empty the user
  // has reached the collection root, so we fall through to the space
  // view (or home when there's only one space configured). Logic
  // lives in a pure helper so it can be unit tested without the
  // full widget render tree.
  const handleBackFromHelpCollection = () => {
    setActiveArticleKey(null);
    const target = computeHelpCollectionBackTarget(
      helpCollectionStack,
      helpSpaces.length,
      activeHelpSpaceSlug,
    );
    switch (target.kind) {
      case 'collection':
        setHelpCollectionStack(target.remainingStack);
        setActiveCollectionSlug(target.slug);
        setActiveView('help-collection');
        onViewChange?.('help-collection');
        return;
      case 'help-space':
        setActiveView('help-space');
        onViewChange?.('help-space');
        return;
      case 'help':
        setActiveView('help');
        onViewChange?.('help');
    }
  };

  const handleOpenHelpArticle = (articleSlug: string) => {
    setActiveArticleKey(articleSlug);
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
      {activeView !== 'home' && activeView !== 'conversation' && activeView !== 'help' && activeView !== 'help-space' && activeView !== 'help-collection' && activeView !== 'help-article' && !(activeView === 'messages' && conversations.length > 0) && (
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
          {queuedMessageCount > 0
            ? `Reconnecting… ${queuedMessageCount === 1 ? '1 message is saved' : `${queuedMessageCount} messages are saved`}.`
            : 'Connection lost. Reconnecting…'}
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
            aiProgressLabel={aiProgressLabel}
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
            contactCaptureCompleted={contactCaptureCompleted}
            onPreChatSubmit={onPreChatSubmit}
            onCaptureEmail={onCaptureEmail}
            onImageClick={onImageClick}
            onAnswerFeedback={onAnswerFeedback}
            connectionStatus={connectionStatus}
            queuedMessageCount={queuedMessageCount}
            csatSubmitted={csatSubmitted}
            onCsatSubmit={onCsatSubmit}
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
            anonymousId={anonymousId}
            onClose={onClose}
            onSelectSpace={handleOpenHelpSpace}
            onSelectCollection={handleOpenHelpCollection}
            onSelectArticle={handleOpenHelpArticle}
          />
        )}
        {activeView === 'help-space' && activeHelpSpace && host && widgetKey && (
          <HelpSpaceView
            key={activeHelpSpace.slug}
            host={host}
            widgetKey={widgetKey}
            space={activeHelpSpace}
            showBack={helpSpaces.length > 1}
            onBack={() => handleNavigate('help')}
            onClose={onClose}
            onSelectCollection={handleOpenHelpCollection}
          />
        )}
        {activeView === 'help-collection' && activeCollectionSlug && host && widgetKey && (
          <HelpCollectionView
            key={activeCollectionSlug}
            host={host}
            widgetKey={widgetKey}
            collectionSlug={activeCollectionSlug}
            spaceSlug={activeHelpSpaceSlug ?? helpSpaces[0]?.slug}
            onBack={handleBackFromHelpCollection}
            onSelectCollection={handleOpenHelpCollection}
            onSelectArticle={handleOpenHelpArticle}
            onClose={onClose}
          />
        )}
        {activeView === 'help-article' && activeArticleKey && host && widgetKey && (
          <HelpArticleView
            key={activeArticleKey}
            host={host}
            widgetKey={widgetKey}
            articleKey={activeArticleKey}
            onBack={() => {
              if (activeCollectionSlug) {
                setActiveView('help-collection');
                onViewChange?.('help-collection');
                return;
              }
              setActiveArticleKey(null);
              setActiveView('help');
              onViewChange?.('help');
            }}
            onClose={onClose}
          />
        )}
      </div>

      {/* Powered by footer */}
      {showBranding && activeView !== 'conversation' && activeView !== 'messages' && (
        <BrandAttribution
          label="Powered by"
          className="helpin-powered-by"
          workspaceId={config.workspaceId}
          workspaceName={config.workspaceName}
          content="chat_widget_footer"
        />
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

import { FunctionComponent } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import type { Message, Conversation, WidgetConfig } from '../types';
import { BottomNav, type WidgetBaseView, WidgetView } from './BottomNav';
import { HomeView } from './HomeView';
import { MessagesView } from './MessagesView';
import { HelpView } from './HelpView';
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
  onTyping?: () => void;
  showPreChatForm: boolean;
  onPreChatSubmit: (data: { name: string; email: string }) => void;
  isTyping?: boolean;
  quickReplies?: string[];
  initialView?: WidgetView;
  connectionStatus?: ConnectionStatus;
  onRetryConnection?: () => void;
  conversations?: Conversation[];
  onSelectConversation?: (conversationId: string) => void;
  onStartNewConversation?: () => void;
  onViewChange?: (view: WidgetView) => void;
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
  quickReplies = [],
  initialView = 'home',
  connectionStatus = 'idle',
  onRetryConnection,
  conversations = [],
  onSelectConversation = () => {},
  onStartNewConversation,
  onViewChange,
}) => {
  const [activeView, setActiveView] = useState<WidgetView>(initialView);
  const [previousView, setPreviousView] = useState<WidgetBaseView>(
    initialView === 'conversation' ? 'home' : initialView,
  );
  const [shouldRender, setShouldRender] = useState(isOpen);
  const [isVisible, setIsVisible] = useState(isOpen);

  // Sync activeView when initialView prop changes (e.g. first-open → conversation)
  useEffect(() => {
    setActiveView(initialView);
  }, [initialView]);

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
  const positionClass = position.includes('left')
    ? 'helpin-chat-window--left'
    : 'helpin-chat-window--right';

  const handleNavigate = (view: WidgetBaseView) => {
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

  return (
    <div
      className={`helpin-chat-window ${positionClass} ${isVisible ? 'helpin-chat-window--visible' : 'helpin-chat-window--hidden'} helpin-theme-${colorScheme}`}
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
            showPreChatForm={showPreChatForm}
            onPreChatSubmit={onPreChatSubmit}
          />
        )}
        {activeView === 'conversation' && (
          <ConversationView
            config={config}
            messages={messages}
            onSendMessage={onSendMessage}
            onTyping={onTyping}
            isTyping={isTyping}
            onBack={() => setActiveView(previousView)}
            onClose={onClose}
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
            onNavigate={(view) => {
              if (view === 'conversation') {
                handleStartConversation('help');
                return;
              }
              handleNavigate(view);
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

import { FunctionComponent } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import type { Message, WidgetConfig } from '../types';
import { BottomNav, type WidgetBaseView, WidgetView } from './BottomNav';
import { HomeView } from './HomeView';
import { MessagesView } from './MessagesView';
import { HelpView } from './HelpView';
import { ConversationView } from './ConversationView';

interface ChatWindowProps {
  config: WidgetConfig;
  messages: Message[];
  isOpen: boolean;
  onClose: () => void;
  onSendMessage: (content: string) => void;
  onQuickReply: (content: string) => void;
  showPreChatForm: boolean;
  onPreChatSubmit: (data: { name: string; email: string }) => void;
  isTyping?: boolean;
  quickReplies?: string[];
  initialView?: WidgetView;
}

export const ChatWindow: FunctionComponent<ChatWindowProps> = ({
  config,
  messages,
  isOpen,
  onClose,
  onSendMessage,
  onQuickReply,
  showPreChatForm,
  onPreChatSubmit,
  isTyping = false,
  quickReplies = [],
  initialView = 'home',
}) => {
  const [activeView, setActiveView] = useState<WidgetView>(initialView);
  const [previousView, setPreviousView] = useState<WidgetBaseView>(
    initialView === 'conversation' ? 'home' : initialView,
  );
  const [shouldRender, setShouldRender] = useState(isOpen);
  const [isVisible, setIsVisible] = useState(isOpen);

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
  };

  const handleStartConversation = (fromView: WidgetBaseView) => {
    setPreviousView(fromView);
    setActiveView('conversation');
  };

  const handleSendFromHome = (content: string) => {
    onSendMessage(content);
    handleStartConversation('home');
  };

  return (
    <div
      className={`helpin-chat-window ${positionClass} ${isVisible ? 'helpin-chat-window--visible' : 'helpin-chat-window--hidden'} helpin-theme-${colorScheme}`}
    >
      {/* Close button */}
      {activeView !== 'conversation' && (
        <button className="helpin-window-close" onClick={onClose} aria-label="Close">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
            <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" />
          </svg>
        </button>
      )}

      {/* View content */}
      <div className="helpin-view-container">
        {activeView === 'home' && (
          <HomeView
            config={config}
            onSendMessage={handleSendFromHome}
            onNavigate={(view) => {
              if (view === 'conversation') {
                handleStartConversation('home');
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
            onBack={() => setActiveView(previousView)}
            onClose={onClose}
          />
        )}
        {activeView === 'messages' && (
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
      {showBranding && activeView !== 'conversation' && (
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
          onNavigate={setActiveView}
          brandColor={brandColor}
        />
      )}
    </div>
  );
};

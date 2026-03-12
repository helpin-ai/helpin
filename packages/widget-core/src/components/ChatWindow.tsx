import { FunctionComponent } from 'preact';
import { useState } from 'preact/hooks';
import type { Message, WidgetConfig } from '../types';
import { BottomNav, WidgetView } from './BottomNav';
import { HomeView } from './HomeView';
import { MessagesView } from './MessagesView';
import { HelpView } from './HelpView';

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

  if (!isOpen) return null;

  const position = config.branding?.widgetPosition || 'bottom-right';
  const brandColor = config.branding?.primaryColor || '#6366f1';
  const showBranding = config.branding?.showBranding ?? true;
  const colorScheme = config.branding?.colorScheme || 'light';

  const handleNavigate = (view: WidgetView | 'messages' | 'help') => {
    setActiveView(view as WidgetView);
  };

  const handleSendFromHome = (content: string) => {
    onSendMessage(content);
    setActiveView('messages');
  };

  return (
    <div
      className={`helpin-chat-window helpin-theme-${colorScheme}`}
      style={{
        [position.includes('left') ? 'left' : 'right']: '20px',
        bottom: '20px',
      }}
    >
      {/* Close button */}
      <button className="helpin-window-close" onClick={onClose} aria-label="Close">
        <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
          <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" />
        </svg>
      </button>

      {/* View content */}
      <div className="helpin-view-container">
        {activeView === 'home' && (
          <HomeView
            config={config}
            onSendMessage={handleSendFromHome}
            onNavigate={handleNavigate}
            showPreChatForm={showPreChatForm}
            onPreChatSubmit={onPreChatSubmit}
          />
        )}
        {activeView === 'messages' && (
          <MessagesView
            config={config}
            messages={messages}
            onSendMessage={onSendMessage}
            onQuickReply={onQuickReply}
            isTyping={isTyping}
            quickReplies={quickReplies}
            hasConversation={messages.length > 0}
          />
        )}
        {activeView === 'help' && (
          <HelpView config={config} onNavigate={handleNavigate} />
        )}
      </div>

      {/* Powered by footer */}
      {showBranding && (
        <div className="helpin-powered-by">
          <span>Powered by</span>
          <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor" className="helpin-powered-by-icon">
            <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" />
          </svg>
          <span className="helpin-powered-by-name">Helpin</span>
        </div>
      )}

      {/* Bottom navigation */}
      <BottomNav
        activeView={activeView}
        onNavigate={setActiveView}
        brandColor={brandColor}
      />
    </div>
  );
};

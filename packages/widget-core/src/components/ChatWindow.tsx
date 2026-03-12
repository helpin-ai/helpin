import { FunctionComponent, Fragment } from 'preact';
import type { Message, WidgetConfig } from '../types';
import { WidgetHeader } from './WidgetHeader';
import { PreChatForm } from './PreChatForm';
import { MessageList } from './MessageList';
import { ComposeBar } from './ComposeBar';
import { QuickReplies } from './QuickReplies';
import { TypingIndicator } from './TypingIndicator';

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
}) => {
  if (!isOpen) return null;

  const position = config.branding?.widgetPosition || 'bottom-right';
  const brandColor = config.branding?.primaryColor || '#6366f1';
  const showBranding = config.branding?.showBranding ?? true;

  return (
    <div
      className="helpin-chat-window"
      style={{
        [position.includes('left') ? 'left' : 'right']: '20px',
        bottom: '20px',
      }}
    >
      <WidgetHeader
        workspaceName={config.workspaceId || 'Support'}
        logoUrl={config.branding?.logoUrl}
        onClose={onClose}
        brandColor={brandColor}
        showBranding={config.branding?.showBranding ?? true}
      />

      <div className="helpin-chat-content">
        {showPreChatForm ? (
          <PreChatForm
            requireEmail={config.features?.preChatForm}
            welcomeMessage={config.branding?.welcomeMessage}
            onSubmit={onPreChatSubmit}
          />
        ) : (
          <Fragment>
            <MessageList messages={messages} />
            {isTyping && <TypingIndicator />}
            {quickReplies.length > 0 && (
              <QuickReplies replies={quickReplies} onSelect={onQuickReply} />
            )}
            <ComposeBar
              onSend={onSendMessage}
              disabled={config.features?.aiEnabled}
            />
          </Fragment>
        )}
      </div>
      {showBranding && (
        <div className="helpin-footer-branding">Powered by Helpin</div>
      )}
    </div>
  );
};

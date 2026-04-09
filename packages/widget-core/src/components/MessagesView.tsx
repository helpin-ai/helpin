import { FunctionComponent } from 'preact';
import type { Message, WidgetConfig } from '../types';
import { MessageList } from './MessageList';
import { ComposeBar } from './ComposeBar';
import { TypingIndicator } from './TypingIndicator';
import { QuickReplies } from './QuickReplies';
import { MessageSquareIcon, CircleHelpIcon } from './icons';

interface MessagesViewProps {
  config: WidgetConfig;
  messages: Message[];
  onSendMessage: (content: string) => void;
  onQuickReply: (content: string) => void;
  onStartConversation: () => void;
  isTyping?: boolean;
  quickReplies?: string[];
  hasConversation: boolean;
  connectionStatus?: 'idle' | 'connecting' | 'connected' | 'disconnected' | 'failed';
}

export const MessagesView: FunctionComponent<MessagesViewProps> = ({
  config,
  messages,
  onSendMessage,
  onQuickReply,
  onStartConversation,
  isTyping = false,
  quickReplies = [],
  hasConversation,
  connectionStatus = 'connected',
}) => {
  const composeDisabled = connectionStatus === 'connecting' || connectionStatus === 'disconnected' || connectionStatus === 'failed';
  const composePlaceholder = connectionStatus === 'failed'
    ? 'Offline. Reconnecting in the background...'
    : connectionStatus === 'disconnected'
      ? 'Connection lost. Reconnecting...'
      : connectionStatus === 'connecting'
        ? 'Connecting to support...'
        : 'Ask a question...';

  if (!hasConversation || messages.length === 0) {
    return (
      <div className="helpin-messages-view">
        <div className="helpin-messages-header">
          <span className="helpin-messages-title">Messages</span>
        </div>
        <div className="helpin-messages-empty">
          <MessageSquareIcon size={48} class="helpin-messages-empty-icon" />
          <h3 className="helpin-messages-empty-title">No messages</h3>
          <p className="helpin-messages-empty-desc">Messages from the team will be shown here</p>
        </div>
        <div className="helpin-messages-ask-container">
          <button className="helpin-messages-ask-btn" onClick={onStartConversation}>
            <span>Ask a question</span>
            <CircleHelpIcon size={18} />
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="helpin-messages-view">
      <div className="helpin-messages-header">
        <span className="helpin-messages-title">Messages</span>
      </div>
      <div className="helpin-messages-thread">
        <MessageList messages={messages} />
        {isTyping && <TypingIndicator />}
        {quickReplies.length > 0 && (
          <QuickReplies replies={quickReplies} onSelect={onQuickReply} />
        )}
      </div>
      <ComposeBar
        onSend={onSendMessage}
        showBranding={config.branding?.showBranding ?? true}
        disabled={composeDisabled}
        placeholder={composePlaceholder}
      />
    </div>
  );
};

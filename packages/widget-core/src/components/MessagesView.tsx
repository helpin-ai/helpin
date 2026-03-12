import { FunctionComponent } from 'preact';
import type { Message, WidgetConfig } from '../types';
import { MessageList } from './MessageList';
import { ComposeBar } from './ComposeBar';
import { TypingIndicator } from './TypingIndicator';
import { QuickReplies } from './QuickReplies';

interface MessagesViewProps {
  config: WidgetConfig;
  messages: Message[];
  onSendMessage: (content: string) => void;
  onQuickReply: (content: string) => void;
  isTyping?: boolean;
  quickReplies?: string[];
  hasConversation: boolean;
}

const CHAT_ICON = 'M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z';

export const MessagesView: FunctionComponent<MessagesViewProps> = ({
  config: _config,
  messages,
  onSendMessage,
  onQuickReply,
  isTyping = false,
  quickReplies = [],
  hasConversation,
}) => {
  if (!hasConversation || messages.length === 0) {
    return (
      <div className="helpin-messages-view">
        <div className="helpin-messages-header">
          <span className="helpin-messages-title">Messages</span>
        </div>
        <div className="helpin-messages-empty">
          <svg viewBox="0 0 24 24" width="48" height="48" fill="currentColor" className="helpin-messages-empty-icon">
            <path d={CHAT_ICON} />
          </svg>
          <h3 className="helpin-messages-empty-title">No messages</h3>
          <p className="helpin-messages-empty-desc">Messages from the team will be shown here</p>
        </div>
        <div className="helpin-messages-new-container">
          <ComposeBar
            onSend={onSendMessage}
            placeholder="Ask a question"
          />
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
      <ComposeBar onSend={onSendMessage} />
    </div>
  );
};

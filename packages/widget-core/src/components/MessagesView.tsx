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
  onStartConversation: () => void;
  isTyping?: boolean;
  quickReplies?: string[];
  hasConversation: boolean;
}

const CHAT_ICON = 'M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z';
const QUESTION_ICON = 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z';

export const MessagesView: FunctionComponent<MessagesViewProps> = ({
  config: _config,
  messages,
  onSendMessage,
  onQuickReply,
  onStartConversation,
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
        <div className="helpin-messages-ask-container">
          <button className="helpin-messages-ask-btn" onClick={onStartConversation}>
            <span>Ask a question</span>
            <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
              <path d={QUESTION_ICON} />
            </svg>
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
      <ComposeBar onSend={onSendMessage} />
    </div>
  );
};

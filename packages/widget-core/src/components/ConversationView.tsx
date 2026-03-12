import { FunctionComponent } from 'preact';
import { useState } from 'preact/hooks';
import type { Message, WidgetConfig } from '../types';
import { MessageList } from './MessageList';
import { ComposeBar } from './ComposeBar';

interface ConversationViewProps {
  config: WidgetConfig;
  messages: Message[];
  onSendMessage: (content: string) => void;
  onBack: () => void;
  onClose?: () => void;
}

const BACK_ICON = 'M15.41 7.41 14 6l-6 6 6 6 1.41-1.41L10.83 12z';
const MORE_ICON = 'M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z';
const CLOSE_ICON = 'M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z';

export const ConversationView: FunctionComponent<ConversationViewProps> = ({
  config,
  messages,
  onSendMessage,
  onBack,
  onClose,
}) => {
  const [introCreatedAt] = useState(() => new Date().toISOString());

  const workspaceName = config.workspaceName || 'Support';
  const logoUrl = config.branding?.logoUrl;
  const welcomeMessage = config.branding?.welcomeMessage || 'Hi there. How can we help?';
  const hasTeamReply = messages.some((message) => message.role !== 'customer');

  const displayMessages = hasTeamReply || messages.length === 0
    ? messages.length === 0
      ? [{
          id: '__intro__',
          conversationId: '__intro__',
          role: 'agent' as const,
          content: welcomeMessage,
          isInternal: false,
          createdAt: introCreatedAt,
        }]
      : messages
    : [{
        id: '__intro__',
        conversationId: '__intro__',
        role: 'agent' as const,
        content: welcomeMessage,
        isInternal: false,
        createdAt: introCreatedAt,
      }, ...messages];

  return (
    <div className="helpin-conversation-view">
      <div className="helpin-conversation-header">
        <button
          type="button"
          className="helpin-conversation-back"
          onClick={onBack}
          aria-label="Back"
        >
          <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
            <path d={BACK_ICON} />
          </svg>
        </button>

        <div className="helpin-conversation-brand">
          {logoUrl ? (
            <img src={logoUrl} alt={workspaceName} className="helpin-conversation-logo" />
          ) : (
            <div className="helpin-conversation-logo-placeholder">
              <span>{workspaceName.charAt(0).toUpperCase()}</span>
            </div>
          )}

          <div className="helpin-conversation-brand-copy">
            <span className="helpin-conversation-title">{workspaceName}</span>
            <span className="helpin-conversation-subtitle">The team can also help</span>
          </div>
        </div>

        <div className="helpin-conversation-header-actions">
          <button
            type="button"
            className="helpin-conversation-header-btn"
            aria-label="More options"
          >
            <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
              <path d={MORE_ICON} />
            </svg>
          </button>
          {onClose && (
            <button
              type="button"
              className="helpin-conversation-header-btn"
              onClick={onClose}
              aria-label="Close"
            >
              <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                <path d={CLOSE_ICON} />
              </svg>
            </button>
          )}
        </div>
      </div>

      <div className="helpin-conversation-thread">
        <MessageList
          messages={displayMessages}
          showDateSeparators={false}
          config={config}
        />
      </div>

      <ComposeBar onSend={onSendMessage} />
    </div>
  );
};

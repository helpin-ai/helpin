import { FunctionComponent } from 'preact';
import { useState } from 'preact/hooks';
import type { Message, WidgetConfig } from '../types';
import { MessageList } from './MessageList';
import { ComposeBar } from './ComposeBar';
import { ChevronLeftIcon, MoreVerticalIcon, XIcon } from './icons';

interface ConversationViewProps {
  config: WidgetConfig;
  messages: Message[];
  onSendMessage: (content: string) => void;
  onBack: () => void;
  onClose?: () => void;
}

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
          <ChevronLeftIcon size={20} />
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
            {config.features?.aiEnabled && (
              <span className="helpin-conversation-subtitle">The team can also help</span>
            )}
          </div>
        </div>

        <div className="helpin-conversation-header-actions">
          <button
            type="button"
            className="helpin-conversation-header-btn"
            aria-label="More options"
          >
            <MoreVerticalIcon size={20} />
          </button>
          {onClose && (
            <button
              type="button"
              className="helpin-conversation-header-btn"
              onClick={onClose}
              aria-label="Close"
            >
              <XIcon size={20} />
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

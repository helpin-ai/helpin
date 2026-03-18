import { FunctionComponent } from 'preact';
import { useMemo, useState } from 'preact/hooks';
import type { Message, WidgetConfig } from '../types';
import { MessageList } from './MessageList';
import { ComposeBar } from './ComposeBar';
import { TypingIndicator } from './TypingIndicator';
import { PreChatForm } from './PreChatForm';
import { ChevronLeftIcon, MoreVerticalIcon, XIcon } from './icons';

interface ConversationViewProps {
  config: WidgetConfig;
  messages: Message[];
  onSendMessage: (content: string) => void;
  onTyping?: (content: string) => void;
  isTyping?: boolean;
  isAIThinking?: boolean;
  typingAgentName?: string;
  typingAgentAvatar?: string;
  onBack: () => void;
  onClose?: () => void;
  showPreChatForm?: boolean;
  onPreChatSubmit?: (data: { phone: string; email: string }) => void;
}

export const ConversationView: FunctionComponent<ConversationViewProps> = ({
  config,
  messages,
  onSendMessage,
  onTyping,
  isTyping = false,
  isAIThinking = false,
  typingAgentName,
  typingAgentAvatar,
  onBack,
  onClose,
  showPreChatForm = false,
  onPreChatSubmit,
}) => {
  const [introCreatedAt] = useState(() => new Date().toISOString());
  const [preChatDone, setPreChatDone] = useState(false);

  const workspaceName = config.workspaceName || 'Support';
  const logoUrl = config.branding?.logoUrl;
  const welcomeMessage = config.branding?.welcomeMessage || 'Hi there. How can we help?';
  const hasTeamReply = messages.some((message) => message.role !== 'customer');
  const hasCustomerMessage = messages.some((message) => message.role === 'customer');

  // Derive the most recent responding agent from messages.
  const activeAgent = useMemo(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      const m = messages[i];
      if ((m.role === 'agent' || m.role === 'ai') && m.senderName) {
        return { name: m.senderName, avatar: m.senderAvatar };
      }
    }
    return null;
  }, [messages]);

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
          {activeAgent?.avatar ? (
            <img src={activeAgent.avatar} alt={activeAgent.name} className="helpin-conversation-logo helpin-agent-avatar" />
          ) : activeAgent?.name ? (
            <div className="helpin-conversation-logo-placeholder">
              <span>{activeAgent.name.charAt(0).toUpperCase()}</span>
            </div>
          ) : logoUrl ? (
            <img src={logoUrl} alt={workspaceName} className="helpin-conversation-logo" />
          ) : (
            <div className="helpin-conversation-logo-placeholder">
              <span>{workspaceName.charAt(0).toUpperCase()}</span>
            </div>
          )}

          <div className="helpin-conversation-brand-copy">
            {activeAgent?.name ? (
              <>
                <span className="helpin-conversation-title">{activeAgent.name}</span>
                <span className="helpin-conversation-subtitle">from {workspaceName}</span>
              </>
            ) : (
              <>
                <span className="helpin-conversation-title">{config.features?.aiEnabled ? 'Helpin' : workspaceName}</span>
                {config.features?.aiEnabled && (
                  <span className="helpin-conversation-subtitle">Our bot will reply instantly</span>
                )}
              </>
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
              className="helpin-window-close-inline"
              onClick={onClose}
              aria-label="Close"
            >
              <XIcon size={16} />
            </button>
          )}
        </div>
      </div>

      <div className="helpin-conversation-thread">
        <MessageList
          messages={displayMessages}
          showDateSeparators={true}
          config={config}
        />
        {showPreChatForm && !preChatDone && hasCustomerMessage && onPreChatSubmit && (
          <PreChatForm
            config={config}
            onSubmit={(data) => {
              onPreChatSubmit(data);
              setPreChatDone(true);
            }}
          />
        )}
      </div>

      {isTyping && !isAIThinking && (
        <TypingIndicator
          agentName={typingAgentName}
          agentAvatar={typingAgentAvatar}
        />
      )}
      {isAIThinking && (
        <div className="helpin-ai-thinking">
          <div className="helpin-ai-thinking-icon">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 2a4 4 0 0 1 4 4c0 1.95-1.4 3.58-3.25 3.93L12 22" />
              <path d="M12 2a4 4 0 0 0-4 4c0 1.95 1.4 3.58 3.25 3.93" />
            </svg>
          </div>
          <span className="helpin-ai-thinking-text">Thinking</span>
          <span className="helpin-ai-thinking-dots"><span>.</span><span>.</span><span>.</span></span>
        </div>
      )}
      <ComposeBar onSend={onSendMessage} onTyping={onTyping} />
    </div>
  );
};

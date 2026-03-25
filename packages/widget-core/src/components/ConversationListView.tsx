import { FunctionComponent } from 'preact';
import type { Conversation, WidgetConfig } from '../types';
import { MessageSquareIcon, ChevronRightIcon, XIcon, SendIcon } from './icons';

interface ConversationListViewProps {
  config: WidgetConfig;
  conversations: Conversation[];
  onSelectConversation: (conversationId: string) => void;
  onStartConversation: () => void;
  onClose?: () => void;
}

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffMs = now - then;
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return 'Just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDay = Math.floor(diffHr / 24);
  if (diffDay < 7) return `${diffDay}d ago`;
  return new Date(dateStr).toLocaleDateString();
}

export const ConversationListView: FunctionComponent<ConversationListViewProps> = ({
  config,
  conversations,
  onSelectConversation,
  onStartConversation,
  onClose,
}) => {
  const brandColor = config.branding?.primaryColor || '#6366f1';
  const companyName = config.workspaceName || 'Support';
  const ctaStyle = {
    backgroundColor: brandColor,
    borderColor: brandColor,
    color: '#ffffff',
  } as Record<string, string>;

  if (conversations.length === 0) {
    return (
      <div className="helpin-conversations-view">
        <div className="helpin-conversations-header">
          <div className="helpin-conversations-header-spacer" />
          <span className="helpin-conversations-title">Messages</span>
          {onClose && (
            <button className="helpin-window-close-inline" onClick={onClose} aria-label="Close">
              <XIcon size={16} />
            </button>
          )}
        </div>
        <div className="helpin-conversations-empty">
          <MessageSquareIcon size={48} class="helpin-conversations-empty-icon" />
          <h3 className="helpin-conversations-empty-title">No conversations yet</h3>
          <p className="helpin-conversations-empty-desc">Start a conversation and we'll get back to you</p>
        </div>
        <div className="helpin-conversations-new-container">
          <button
            className="helpin-conversations-new-btn"
            onClick={onStartConversation}
            style={ctaStyle}
          >
            <span>Send us a message</span>
            <SendIcon size={16} />
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="helpin-conversations-view">
      <div className="helpin-conversations-header">
        <div className="helpin-conversations-header-spacer" />
        <span className="helpin-conversations-title">Messages</span>
        {onClose && (
          <button className="helpin-window-close-inline" onClick={onClose} aria-label="Close">
            <XIcon size={16} />
          </button>
        )}
      </div>
      <div className="helpin-conversations-list">
        {conversations.map((conv) => (
          <button
            key={conv.id}
            className="helpin-conversation-item"
            onClick={() => onSelectConversation(conv.id)}
          >
            <div className="helpin-conversation-item-avatar" style={{ backgroundColor: brandColor }}>
              <MessageSquareIcon size={14} />
            </div>
            <div className="helpin-conversation-item-content">
              <span className={`helpin-conversation-item-preview${conv.unreadCount ? ' helpin-conversation-item-preview--unread' : ''}`}>
                {conv.lastMessage
                  ? (conv.lastMessage.length > 50
                      ? conv.lastMessage.slice(0, 50) + '...'
                      : conv.lastMessage)
                  : (conv.subject || 'Untitled conversation')}
              </span>
              <span className="helpin-conversation-item-meta">
                {companyName}
                {conv.lastMessageAt && ` \u00B7 ${timeAgo(conv.lastMessageAt)}`}
              </span>
            </div>
            {conv.unreadCount ? (
              <span className="helpin-conversation-item-badge">
                {conv.unreadCount > 99 ? '99+' : conv.unreadCount}
              </span>
            ) : (
              <ChevronRightIcon size={16} class="helpin-conversation-item-arrow" />
            )}
          </button>
        ))}
      </div>
      <div className="helpin-conversations-new-container">
        <button
          className="helpin-conversations-new-btn"
          onClick={onStartConversation}
          style={ctaStyle}
        >
          <span>Send us a message</span>
          <SendIcon size={16} />
        </button>
      </div>
    </div>
  );
};

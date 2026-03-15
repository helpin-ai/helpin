import { FunctionComponent } from 'preact';
import type { Conversation, WidgetConfig } from '../types';
import { MessageSquareIcon, ChevronRightIcon } from './icons';

interface ConversationListViewProps {
  config: WidgetConfig;
  conversations: Conversation[];
  onSelectConversation: (conversationId: string) => void;
  onStartConversation: () => void;
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

const STATUS_DOT: Record<string, string> = {
  open: '#22c55e',
  in_progress: '#3b82f6',
  waiting: '#f59e0b',
  resolved: '#9ca3af',
  closed: '#6b7280',
};

export const ConversationListView: FunctionComponent<ConversationListViewProps> = ({
  config,
  conversations,
  onSelectConversation,
  onStartConversation,
}) => {
  const brandColor = config.branding?.primaryColor || '#6366f1';

  if (conversations.length === 0) {
    return (
      <div className="helpin-conversations-view">
        <div className="helpin-conversations-header">
          <span className="helpin-conversations-title">Conversations</span>
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
            style={{ backgroundColor: brandColor }}
          >
            <MessageSquareIcon size={16} />
            <span>New conversation</span>
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="helpin-conversations-view">
      <div className="helpin-conversations-header">
        <span className="helpin-conversations-title">Conversations</span>
        <button
          className="helpin-conversations-new-small"
          onClick={onStartConversation}
          style={{ color: brandColor }}
        >
          + New
        </button>
      </div>
      <div className="helpin-conversations-list">
        {conversations.map((conv) => (
          <button
            key={conv.id}
            className="helpin-conversation-item"
            onClick={() => onSelectConversation(conv.id)}
          >
            <div className="helpin-conversation-item-left">
              <span
                className="helpin-conversation-status-dot"
                style={{ backgroundColor: STATUS_DOT[conv.status] || '#9ca3af' }}
              />
              <div className="helpin-conversation-item-content">
                <span className="helpin-conversation-item-subject">
                  {conv.subject || 'Untitled conversation'}
                </span>
                {conv.lastMessage && (
                  <span className="helpin-conversation-item-preview">
                    {conv.lastMessage.length > 60
                      ? conv.lastMessage.slice(0, 60) + '...'
                      : conv.lastMessage}
                  </span>
                )}
              </div>
            </div>
            <div className="helpin-conversation-item-right">
              {conv.lastMessageAt && (
                <span className="helpin-conversation-item-time">
                  {timeAgo(conv.lastMessageAt)}
                </span>
              )}
              {(conv.unreadCount ?? 0) > 0 && (
                <span
                  className="helpin-conversation-item-badge"
                  style={{ backgroundColor: brandColor }}
                >
                  {conv.unreadCount}
                </span>
              )}
              <ChevronRightIcon size={14} class="helpin-conversation-item-arrow" />
            </div>
          </button>
        ))}
      </div>
    </div>
  );
};

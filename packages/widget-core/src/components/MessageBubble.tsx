import { FunctionComponent } from 'preact';
import type { Message, WidgetConfig } from '../types';

interface MessageBubbleProps {
  message: Message;
  config?: WidgetConfig;
}

function escapeHtml(str: string): string {
  const div = document.createElement('div');
  div.appendChild(document.createTextNode(str));
  return div.innerHTML;
}

function formatRelativeTime(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffMs = now - then;
  const diffMin = Math.floor(diffMs / 60000);

  if (diffMin < 1) return 'Just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHours = Math.floor(diffMin / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  return new Date(dateStr).toLocaleDateString();
}

export const MessageBubble: FunctionComponent<MessageBubbleProps> = ({ message, config }) => {
  const isCustomer = message.role === 'customer';
  const isAI = message.role === 'ai';
  const isAgent = message.role === 'agent';
  const isSystem = message.role === 'system';

  const bubbleClass = [
    'helpin-message-bubble',
    isCustomer && 'helpin-message--customer',
    isAgent && 'helpin-message--agent',
    isAI && 'helpin-message--ai',
    isSystem && 'helpin-message--system',
    message.isInternal && 'helpin-message--internal',
  ]
    .filter(Boolean)
    .join(' ');

  const roleLabel = isAI ? 'AI Agent' : isAgent ? 'Agent' : isSystem ? 'System' : '';
  const senderName = isCustomer ? '' : (config?.workspaceName || 'Support');

  return (
    <div
      className={`helpin-message-row ${isCustomer ? 'helpin-message-row--customer' : 'helpin-message-row--agent'}`}
      role="listitem"
      aria-label={`${roleLabel || 'You'} message`}
    >
      <div className={bubbleClass}>
        <div
          className="helpin-message-content"
          dangerouslySetInnerHTML={{ __html: escapeHtml(message.content) }}
        />

        {message.sources && message.sources.length > 0 && (
          <div className="helpin-message-sources">
            {message.sources.map((source, idx) => (
              <div key={idx} className="helpin-source-item">
                {source.title}
              </div>
            ))}
          </div>
        )}

        {message.aiConfidence !== undefined && (
          <div className="helpin-message-confidence">
            Confidence: {Math.round(message.aiConfidence * 100)}%
          </div>
        )}
      </div>

      {!isCustomer && senderName && (
        <div className="helpin-message-attribution">
          <span className="helpin-message-sender">{senderName}</span>
          {roleLabel && (
            <>
              <span className="helpin-message-attr-dot">&middot;</span>
              <span>{roleLabel}</span>
            </>
          )}
          <span className="helpin-message-attr-dot">&middot;</span>
          <span>{formatRelativeTime(message.createdAt)}</span>
        </div>
      )}
    </div>
  );
};

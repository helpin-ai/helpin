import { FunctionComponent } from 'preact';
import type { Message, WidgetConfig } from '../types';

interface MessageBubbleProps {
  message: Message;
  config?: WidgetConfig;
  isFirstInGroup?: boolean;
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
  const d = new Date(dateStr);
  return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
}

export const MessageBubble: FunctionComponent<MessageBubbleProps> = ({
  message,
  config,
  isFirstInGroup = true,
}) => {
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

  const agentName = message.senderName;
  const agentAvatar = message.senderAvatar;
  const displayName = isCustomer ? '' : (agentName || config?.workspaceName || 'Support');
  const tooltipText = formatRelativeTime(message.createdAt);

  return (
    <div
      className={`helpin-message-row ${isCustomer ? 'helpin-message-row--customer' : 'helpin-message-row--agent'} ${isFirstInGroup ? '' : 'helpin-message-row--consecutive'}`}
      role="listitem"
      aria-label={`${displayName || 'You'} message`}
    >
      {!isCustomer ? (
        <>
          {/* Agent: avatar + name row, shown only on first message */}
          {isFirstInGroup && (
            <div className="helpin-message-agent-header">
              {agentAvatar ? (
                <img src={agentAvatar} alt={displayName} className="helpin-message-avatar" />
              ) : displayName ? (
                <span className="helpin-message-avatar-placeholder">
                  {displayName.charAt(0).toUpperCase()}
                </span>
              ) : null}
              {displayName && (
                <span className="helpin-message-agent-name">{displayName}</span>
              )}
            </div>
          )}

          {/* Bubble — indented to align with name, timestamp on hover */}
          <div className="helpin-message-agent-bubble-wrap">
            <div className={bubbleClass} data-tooltip={tooltipText}>
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
          </div>
        </>
      ) : (
        /* Customer bubble — right-aligned, no avatar */
        <div className={bubbleClass} data-tooltip={tooltipText}>
          <div
            className="helpin-message-content"
            dangerouslySetInnerHTML={{ __html: escapeHtml(message.content) }}
          />
        </div>
      )}
    </div>
  );
};

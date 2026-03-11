import { FunctionComponent } from 'preact';
import type { Message } from '../types';

interface MessageBubbleProps {
  message: Message;
}

function escapeHtml(str: string): string {
  const div = document.createElement('div');
  div.appendChild(document.createTextNode(str));
  return div.innerHTML;
}

export const MessageBubble: FunctionComponent<MessageBubbleProps> = ({ message }) => {
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

  const roleLabel = isCustomer ? 'You' : isAI ? 'AI assistant' : isAgent ? 'Support agent' : 'System';

  const formatTime = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  return (
    <div className={bubbleClass} role="listitem" aria-label={`${roleLabel} message`}>
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

      <div className="helpin-message-time" aria-label={`Sent at ${formatTime(message.createdAt)}`}>
        {formatTime(message.createdAt)}
      </div>
    </div>
  );
};

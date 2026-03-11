import { h, FunctionComponent } from 'preact';
import type { Message } from '../types';

interface MessageBubbleProps {
  message: Message;
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

  const formatTime = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  return (
    <div className={bubbleClass}>
      <div className="helpin-message-content">{message.content}</div>
      
      {message.sources && message.sources.length > 0 && (
        <div className="helpin-message-sources">
          {message.sources.map((source, idx) => (
            <div key={idx} className="helpin-source-item">
              📄 {source.title}
            </div>
          ))}
        </div>
      )}
      
      {message.aiConfidence !== undefined && (
        <div className="helpin-message-confidence">
          Confidence: {Math.round(message.aiConfidence * 100)}%
        </div>
      )}
      
      <div className="helpin-message-time">{formatTime(message.createdAt)}</div>
    </div>
  );
};

import { FunctionComponent } from 'preact';
import { useRef, useEffect } from 'preact/hooks';
import type { Message, WidgetConfig } from '../types';
import { MessageBubble } from './MessageBubble';

interface MessageListProps {
  messages: Message[];
  showDateSeparators?: boolean;
  config?: WidgetConfig;
  onImageClick?: (src: string, alt: string) => void;
}

export const MessageList: FunctionComponent<MessageListProps> = ({
  messages,
  showDateSeparators = true,
  config,
  onImageClick,
}) => {
  const listRef = useRef<HTMLDivElement>(null);
  const isInitialMount = useRef(true);

  useEffect(() => {
    if (listRef.current) {
      const el = listRef.current;
      if (isInitialMount.current) {
        el.scrollTop = el.scrollHeight;
        isInitialMount.current = false;
        return;
      }
      const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 100;
      if (isNearBottom) {
        el.scrollTop = el.scrollHeight;
      }
    }
  }, [messages]);

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    const today = new Date();
    const yesterday = new Date(today);
    yesterday.setDate(yesterday.getDate() - 1);

    if (date.toDateString() === today.toDateString()) {
      return 'Today';
    } else if (date.toDateString() === yesterday.toDateString()) {
      return 'Yesterday';
    }
    return date.toLocaleDateString(undefined, {
      weekday: 'long',
      day: 'numeric',
      month: 'long',
    });
  };

  const getDateSeparator = (dateStr: string, idx: number): string | null => {
    if (!showDateSeparators) return null;
    if (idx === 0) return formatDate(dateStr);
    const prevDate = new Date(messages[idx - 1].createdAt);
    const currDate = new Date(dateStr);
    if (prevDate.toDateString() !== currDate.toDateString()) {
      return formatDate(dateStr);
    }
    return null;
  };

  const getSenderGroupKey = (message: Message): string => {
    if (message.role === 'customer') {
      return 'customer';
    }
    // teammate_joined renders as the flat Intercom-style pill — a distinct
    // layout with no bubble header. Force it into its own group so the
    // following reply shows its full sender header instead of collapsing
    // into the pill's "group".
    if (message.role === 'system' && message.systemEventType === 'teammate_joined') {
      return `pill:${message.id}`;
    }
    if (message.role === 'system' && !message.senderName && !message.senderAvatar) {
      return 'system';
    }
    if (message.role === 'ai') {
      return 'incoming:Helpin AI';
    }
    return `incoming:${message.senderName || config?.workspaceName || 'Support Agent'}`;
  };

  return (
    <div className="helpin-message-list" ref={listRef} role="list" aria-label="Messages">
      {messages.map((message, idx) => {
        const dateSeparator = getDateSeparator(message.createdAt, idx);
        // Consecutive = same sender identity for incoming messages.
        const prev = idx > 0 ? messages[idx - 1] : null;
        const isFirstInGroup = !prev
          || getSenderGroupKey(prev) !== getSenderGroupKey(message)
          || !!dateSeparator;
        return (
          <div key={message.id}>
            {dateSeparator && (
              <div className="helpin-date-separator">
                <span>{dateSeparator}</span>
              </div>
            )}
            <MessageBubble message={message} config={config} isFirstInGroup={isFirstInGroup} onImageClick={onImageClick} />
          </div>
        );
      })}
    </div>
  );
};

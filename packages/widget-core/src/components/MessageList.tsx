import { h, FunctionComponent } from 'preact';
import { useRef, useEffect } from 'preact/hooks';
import type { Message } from '../types';
import { MessageBubble } from './MessageBubble';

interface MessageListProps {
  messages: Message[];
}

export const MessageList: FunctionComponent<MessageListProps> = ({ messages }) => {
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (listRef.current) {
      listRef.current.scrollTop = listRef.current.scrollHeight;
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
    return date.toLocaleDateString();
  };

  const getDateSeparator = (dateStr: string, idx: number): string | null => {
    if (idx === 0) return formatDate(dateStr);
    const prevDate = new Date(messages[idx - 1].createdAt);
    const currDate = new Date(dateStr);
    if (prevDate.toDateString() !== currDate.toDateString()) {
      return formatDate(dateStr);
    }
    return null;
  };

  return (
    <div className="helpin-message-list" ref={listRef}>
      {messages.map((message, idx) => {
        const dateSeparator = getDateSeparator(message.createdAt, idx);
        return (
          <div key={message.id}>
            {dateSeparator && (
              <div className="helpin-date-separator">
                <span>{dateSeparator}</span>
              </div>
            )}
            <MessageBubble message={message} />
          </div>
        );
      })}
    </div>
  );
};

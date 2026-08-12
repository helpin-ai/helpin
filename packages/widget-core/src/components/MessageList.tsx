import { FunctionComponent } from 'preact';
import { useRef, useEffect, useState } from 'preact/hooks';
import type { Message, WidgetConfig } from '../types';
import { MessageBubble } from './MessageBubble';

interface MessageListProps {
  messages: Message[];
  showDateSeparators?: boolean;
  config?: WidgetConfig;
  onImageClick?: (src: string, alt: string) => void;
  onAnswerFeedback?: (messageId: string, helpful: boolean) => void;
}

function normalizeMessageText(content: string): string {
  return content
    .replace(/[*_`#>]/g, '')
    .replace(/[^\p{L}\p{N}\s]/gu, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .toLowerCase();
}

function isGreetingOnlyCustomerMessage(message: Message): boolean {
  const text = normalizeMessageText(message.content);
  return /^(?:hi|hello|hey|hiya|howdy|hola|buenas|buenos días|buenas tardes|buenas noches|bonjour|salut|hallo|guten tag|ciao|olá|ola|oi|namaste)(?: (?:there|everyone|team|all|helpin))?$/.test(text);
}

function isLikelyGreeting(message: Message): boolean {
  if (message.sources && message.sources.length > 0) return false;
  const text = normalizeMessageText(message.content);
  if (!/^(?:hi|hello|hey|hola|bonjour|salut|hallo|ciao|olá|ola|oi)(?:\s|$)/.test(text)) return false;
  return /(?:thanks? for (?:reaching out|contacting us)|how (?:can|may) (?:i|we) help|what can (?:i|we) help|en (?:qué|que) puedo ayudarte|(?:cómo|como) puedo ayudarte|comment puis je vous aider|wie kann ich (?:dir|ihnen) helfen|como posso ajudar)/.test(text);
}

export const MessageList: FunctionComponent<MessageListProps> = ({
  messages,
  showDateSeparators = true,
  config,
  onImageClick,
  onAnswerFeedback,
}) => {
  const listRef = useRef<HTMLDivElement>(null);
  const isInitialMount = useRef(true);
  const isNearBottomRef = useRef(true);
  const [showJumpToLatest, setShowJumpToLatest] = useState(false);
  let hasCustomerQuestion = false;
  let latestFeedbackMessageId: string | null = null;
  for (const message of messages) {
    if (message.role === 'customer' && message.content.trim().length > 0) {
      hasCustomerQuestion = !isGreetingOnlyCustomerMessage(message);
      latestFeedbackMessageId = null;
      continue;
    }
    if (
      hasCustomerQuestion
      && message.role === 'ai'
      && message.content.trim().length > 0
      && !message.isStreaming
      && !message.systemEventType
      && !isLikelyGreeting(message)
    ) {
      latestFeedbackMessageId = message.id;
    }
  }

  const scrollToLatest = () => {
    const el = listRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
    isNearBottomRef.current = true;
    setShowJumpToLatest(false);
  };

  const handleScroll = () => {
    const el = listRef.current;
    if (!el) return;
    const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 100;
    isNearBottomRef.current = isNearBottom;
    if (isNearBottom) setShowJumpToLatest(false);
  };

  useEffect(() => {
    if (listRef.current) {
      if (isInitialMount.current) {
        scrollToLatest();
        isInitialMount.current = false;
        return;
      }
      if (isNearBottomRef.current) {
        scrollToLatest();
      } else {
        setShowJumpToLatest(true);
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
    <div className="helpin-message-list-shell">
      <div
        className="helpin-message-list helpin-message-list--smooth-enter"
        ref={listRef}
        role="list"
        aria-label="Messages"
        onScroll={handleScroll}
      >
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
            <MessageBubble
              message={message}
              config={config}
              isFirstInGroup={isFirstInGroup}
              onImageClick={onImageClick}
              onAnswerFeedback={message.id === latestFeedbackMessageId ? onAnswerFeedback : undefined}
            />
          </div>
        );
      })}
      </div>
      {showJumpToLatest && (
        <button type="button" className="helpin-jump-to-latest" onClick={scrollToLatest}>
          <span aria-hidden="true">↓</span>
          Jump to latest
        </button>
      )}
    </div>
  );
};

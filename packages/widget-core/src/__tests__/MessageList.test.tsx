import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/preact';
import { MessageList } from '../components/MessageList';
import type { Message } from '../types';

describe('MessageList', () => {
  const createMessages = (count: number): Message[] => {
    return Array.from({ length: count }, (_, i) => ({
      id: `msg-${i}`,
      conversationId: 'conv-1',
      role: 'customer' as const,
      content: `Message ${i}`,
      isInternal: false,
      createdAt: new Date(Date.now() - (count - i) * 60000).toISOString(),
    }));
  };

  it('renders empty list', () => {
    const { container } = render(<MessageList messages={[]} />);
    expect(container.querySelector('.helpin-message-list')).toBeTruthy();
  });

  it('renders list of messages', () => {
    const messages = createMessages(3);
    const { container } = render(<MessageList messages={messages} />);
    
    const bubbles = container.querySelectorAll('.helpin-message-bubble');
    expect(bubbles.length).toBe(3);
  });

  it('displays message content', () => {
    const messages: Message[] = [
      {
        id: 'msg-1',
        conversationId: 'conv-1',
        role: 'customer',
        content: 'Hello world',
        isInternal: false,
        createdAt: new Date().toISOString(),
      },
    ];
    
    const { container } = render(<MessageList messages={messages} />);
    expect(container.textContent).toContain('Hello world');
  });

  it('shows date separator for first message', () => {
    const messages = createMessages(1);
    const { container } = render(<MessageList messages={messages} />);
    
    const separator = container.querySelector('.helpin-date-separator');
    expect(separator).toBeTruthy();
  });

  it('does not show separator for messages on same day', () => {
    const now = new Date();
    const messages: Message[] = [
      {
        id: 'msg-1',
        conversationId: 'conv-1',
        role: 'customer',
        content: 'First',
        isInternal: false,
        createdAt: now.toISOString(),
      },
      {
        id: 'msg-2',
        conversationId: 'conv-1',
        role: 'customer',
        content: 'Second',
        isInternal: false,
        createdAt: now.toISOString(),
      },
    ];
    
    const { container } = render(<MessageList messages={messages} />);
    const separators = container.querySelectorAll('.helpin-date-separator');
    expect(separators.length).toBe(1);
  });

  it('renders different message roles', () => {
    const messages: Message[] = [
      {
        id: 'msg-1',
        conversationId: 'conv-1',
        role: 'customer',
        content: 'Customer message',
        isInternal: false,
        createdAt: new Date().toISOString(),
      },
      {
        id: 'msg-2',
        conversationId: 'conv-1',
        role: 'agent',
        content: 'Agent message',
        isInternal: false,
        createdAt: new Date().toISOString(),
      },
      {
        id: 'msg-3',
        conversationId: 'conv-1',
        role: 'ai',
        content: 'AI message',
        isInternal: false,
        createdAt: new Date().toISOString(),
      },
    ];
    
    const { container } = render(<MessageList messages={messages} />);
    
    expect(container.querySelector('.helpin-message--customer')).toBeTruthy();
    expect(container.querySelector('.helpin-message--agent')).toBeTruthy();
    expect(container.querySelector('.helpin-message--ai')).toBeTruthy();
  });

  it('marks the message list for one smooth thread-level reveal', () => {
    const messages = createMessages(3);
    const { container } = render(<MessageList messages={messages} />);

    const list = container.querySelector('.helpin-message-list');

    expect(list?.classList.contains('helpin-message-list--smooth-enter')).toBe(true);
    expect(container.querySelector('.helpin-message-list-item')).toBeNull();
  });
});

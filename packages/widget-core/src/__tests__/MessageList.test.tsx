import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render } from '@testing-library/preact';
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

  it('preserves reading position and offers a jump to latest control for new messages', () => {
    const initialMessages = createMessages(3);
    const rendered = render(<MessageList messages={initialMessages} />);
    const list = rendered.getByRole('list') as HTMLDivElement;

    Object.defineProperties(list, {
      scrollHeight: { configurable: true, value: 1000 },
      clientHeight: { configurable: true, value: 300 },
      scrollTop: { configurable: true, writable: true, value: 120 },
    });
    fireEvent.scroll(list);

    rendered.rerender(<MessageList messages={[...initialMessages, ...createMessages(1).map(message => ({ ...message, id: 'msg-new' }))]} />);

    const jumpButton = rendered.getByRole('button', { name: /jump to latest/i });
    expect(list.scrollTop).toBe(120);

    fireEvent.click(jumpButton);
    expect(list.scrollTop).toBe(1000);
    expect(rendered.queryByRole('button', { name: /jump to latest/i })).toBeNull();
  });

  it('shows feedback only on the latest substantive AI answer after a customer question', () => {
    const onAnswerFeedback = vi.fn();
    const messages: Message[] = [
      {
        id: 'customer-1', conversationId: 'conv-1', role: 'customer', content: 'How do I export my data?',
        isInternal: false, createdAt: new Date().toISOString(),
      },
      {
        id: 'greeting-1', conversationId: 'conv-1', role: 'ai',
        content: 'Hi there! 👋 Thanks for reaching out. How can I help you today?',
        isInternal: false, createdAt: new Date().toISOString(),
      },
      {
        id: 'answer-1', conversationId: 'conv-1', role: 'ai', content: 'Open Settings, then choose Export data.',
        isInternal: false, createdAt: new Date().toISOString(),
      },
      {
        id: 'customer-2', conversationId: 'conv-1', role: 'customer', content: 'Can I export CSV?',
        isInternal: false, createdAt: new Date().toISOString(),
      },
      {
        id: 'answer-2', conversationId: 'conv-1', role: 'ai', content: 'Yes. Select CSV before starting the export.',
        isInternal: false, createdAt: new Date().toISOString(),
      },
    ];

    const rendered = render(<MessageList messages={messages} onAnswerFeedback={onAnswerFeedback} />);

    expect(rendered.queryAllByText('Helpful?')).toHaveLength(1);
    fireEvent.click(rendered.getByRole('button', { name: 'This answer was helpful' }));
    expect(onAnswerFeedback).toHaveBeenCalledWith('answer-2', true);
  });

  it('does not show feedback before a customer has asked a question', () => {
    const rendered = render(<MessageList
      messages={[{
        id: 'server-greeting', conversationId: 'conv-1', role: 'ai',
        content: 'Hi there! Thanks for reaching out. How can I help you today?',
        isInternal: false, createdAt: new Date().toISOString(),
      }]}
      onAnswerFeedback={() => {}}
    />);

    expect(rendered.queryByText('Helpful?')).toBeNull();
  });

  it('does not show feedback for a greeting-only exchange in Spanish', () => {
    const rendered = render(<MessageList
      messages={[
        {
          id: 'customer-greeting', conversationId: 'conv-1', role: 'customer', content: 'Hola',
          isInternal: false, createdAt: new Date().toISOString(),
        },
        {
          id: 'ai-greeting', conversationId: 'conv-1', role: 'ai',
          content: '¡Hola! 👋 ¿En qué puedo ayudarte hoy?',
          isInternal: false, createdAt: new Date().toISOString(),
        },
      ]}
      onAnswerFeedback={() => {}}
    />);

    expect(rendered.queryByText('Helpful?')).toBeNull();
  });
});

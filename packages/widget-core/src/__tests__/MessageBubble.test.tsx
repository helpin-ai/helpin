import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/preact';
import { MessageBubble } from '../components/MessageBubble';
import type { Message } from '../types';

describe('MessageBubble', () => {
  const createMessage = (overrides: Partial<Message> = {}): Message => ({
    id: 'msg-1',
    conversationId: 'conv-1',
    role: 'customer',
    content: 'Hello world',
    isInternal: false,
    createdAt: '2024-01-15T10:00:00Z',
    ...overrides,
  });

  it('renders customer message', () => {
    const { container } = render(<MessageBubble message={createMessage({ role: 'customer' })} />);
    expect(container.textContent).toContain('Hello world');
    expect(container.querySelector('.helpin-message--customer')).toBeTruthy();
  });

  it('renders agent message', () => {
    const { container } = render(<MessageBubble message={createMessage({ role: 'agent' })} />);
    expect(container.querySelector('.helpin-message--agent')).toBeTruthy();
  });

  it('renders AI message', () => {
    const { container } = render(<MessageBubble message={createMessage({ role: 'ai' })} />);
    expect(container.querySelector('.helpin-message--ai')).toBeTruthy();
  });

  it('renders system message', () => {
    const { container } = render(<MessageBubble message={createMessage({ role: 'system' })} />);
    expect(container.querySelector('.helpin-message--system')).toBeTruthy();
  });

  it('renders internal note', () => {
    const { container } = render(<MessageBubble message={createMessage({ isInternal: true })} />);
    expect(container.querySelector('.helpin-message--internal')).toBeTruthy();
  });

  it('displays AI sources', () => {
    const message = createMessage({
      role: 'ai',
      sources: [
        { docId: 'doc-1', title: 'Test Doc', snippet: 'Test snippet', confidence: 0.9, language: 'en' },
      ],
    });
    const { container } = render(<MessageBubble message={message} />);
    expect(container.textContent).toContain('Test Doc');
  });

  it('displays confidence score', () => {
    const message = createMessage({
      role: 'ai',
      aiConfidence: 0.85,
    });
    const { container } = render(<MessageBubble message={message} />);
    expect(container.textContent).toContain('85%');
  });
});

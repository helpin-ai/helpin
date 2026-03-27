import { describe, it, expect } from 'vitest';
import { fireEvent, render } from '@testing-library/preact';
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

  it('displays AI source count and reveals titles in popover', () => {
    const message = createMessage({
      role: 'ai',
      sources: [
        { docId: 'doc-1', title: 'Test Doc', snippet: 'Test snippet', confidence: 0.9, language: 'en' },
      ],
    });
    const { container, getByRole, queryByText } = render(<MessageBubble message={message} />);
    expect(container.textContent).toContain('1 source');
    expect(queryByText('Test Doc')).toBeNull();

    fireEvent.click(getByRole('button', { name: '1 source' }));
    expect(queryByText('Test Doc')).toBeTruthy();
  });

  it('displays confidence score', () => {
    const message = createMessage({
      role: 'ai',
      sources: [
        { docId: 'doc-1', title: 'Test Doc', snippet: 'Test snippet', confidence: 0.85, language: 'en' },
      ],
      aiConfidence: 0.85,
    });
    const { container } = render(<MessageBubble message={message} />);
    expect(container.textContent).toContain('85%');
  });

  it('does not display confidence without sources', () => {
    const message = createMessage({
      role: 'ai',
      aiConfidence: 0.85,
    });
    const { container } = render(<MessageBubble message={message} />);
    expect(container.textContent).not.toContain('85%');
    expect(container.querySelector('.helpin-message-confidence')).toBeNull();
  });

  it('displays email channel badge', () => {
    const message = createMessage({
      role: 'agent',
      viaChannel: 'email',
    });
    const { container } = render(<MessageBubble message={message} />);
    expect(container.textContent).toContain('Via email');
  });

  it('renders link previews for support messages', () => {
    const message = createMessage({
      role: 'agent',
      linkPreviews: [
        {
          url: 'https://example.com/pricing',
          title: 'Pricing',
          description: 'Compare plans and limits.',
          host: 'example.com',
        },
      ],
    });
    const { container, getByRole } = render(<MessageBubble message={message} />);
    expect(container.textContent).toContain('Pricing');
    expect((getByRole('link', { name: /pricing/i }) as HTMLAnchorElement).href).toContain('https://example.com/pricing');
  });

  it('uses outgoing preview styling for customer links and incoming styling for agent links', () => {
    const customerMessage = createMessage({
      role: 'customer',
      linkPreviews: [
        {
          url: 'https://example.com/customer',
          title: 'Customer Link',
          host: 'example.com',
        },
      ],
    });
    const agentMessage = createMessage({
      role: 'agent',
      linkPreviews: [
        {
          url: 'https://example.com/agent',
          title: 'Agent Link',
          host: 'example.com',
        },
      ],
    });

    const customerRender = render(<MessageBubble message={customerMessage} />);
    expect(customerRender.container.querySelector('.helpin-link-preview--outgoing')).toBeTruthy();

    const agentRender = render(<MessageBubble message={agentMessage} />);
    expect(agentRender.container.querySelector('.helpin-link-preview--outgoing')).toBeNull();
  });
});

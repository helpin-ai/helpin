import { describe, it, expect, vi } from 'vitest';
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

  it('renders an optimistic customer message with a muted sending state', () => {
    const { container } = render(<MessageBubble message={createMessage({
      clientId: 'temp-1',
      deliveryStatus: 'sending',
    })} />);

    expect(container.querySelector('.helpin-message--sending')).toBeTruthy();
    expect(container.querySelector('.helpin-message-row--optimistic')).toBeTruthy();
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

    fireEvent.click(getByRole('button', { name: 'Based on 1 source' }));
    expect(queryByText('Test Doc')).toBeTruthy();
  });

  it('uses source attribution instead of exposing an exact confidence score', () => {
    const message = createMessage({
      role: 'ai',
      sources: [
        { docId: 'doc-1', title: 'Test Doc', snippet: 'Test snippet', confidence: 0.85, language: 'en' },
      ],
      aiConfidence: 0.85,
    });
    const { container } = render(<MessageBubble message={message} />);
    expect(container.textContent).toContain('Based on 1 source');
    expect(container.textContent).not.toContain('85%');
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

  it('collects lightweight feedback on completed AI answers', () => {
    const onAnswerFeedback = vi.fn();
    const message = createMessage({ role: 'ai', id: 'answer-1' });
    const { getByRole, getByText, queryByRole } = render(
      <MessageBubble message={message} onAnswerFeedback={onAnswerFeedback} />,
    );

    fireEvent.click(getByRole('button', { name: 'This answer was helpful' }));

    expect(onAnswerFeedback).toHaveBeenCalledWith('answer-1', true);
    expect(getByText('Thanks for the feedback')).toBeTruthy();
    expect(queryByRole('button', { name: 'This answer was not helpful' })).toBeNull();
  });

  it('does not show answer feedback on the static AI welcome message', () => {
    const onAnswerFeedback = vi.fn();
    const message = createMessage({
      id: '__intro__',
      conversationId: '__intro__',
      role: 'ai',
      content: 'Hi there! How can we help you today?',
    });
    const { queryByText, queryByRole } = render(
      <MessageBubble message={message} onAnswerFeedback={onAnswerFeedback} />,
    );

    expect(queryByText('Helpful?')).toBeNull();
    expect(queryByRole('button', { name: 'This answer was helpful' })).toBeNull();
  });

  it('displays email channel badge', () => {
    const message = createMessage({
      role: 'agent',
      viaChannel: 'email',
    });
    const { container } = render(<MessageBubble message={message} />);
    expect(container.textContent).toContain('Via email');
  });

  it('hides quoted email history until the visitor expands it', () => {
    const message = createMessage({
      role: 'agent',
      viaChannel: 'email',
      content: 'Legacy full email with Old quoted body',
      emailVisibleText: 'Fresh email reply',
      emailQuotedText: 'On Tuesday someone wrote:\n\nOld quoted body',
      emailHasQuotedContent: true,
      emailProjectionConfidence: 'high',
      emailProjectionVersion: 1,
    });

    const { container, getByRole } = render(<MessageBubble message={message} />);
    expect(container.textContent).toContain('Fresh email reply');
    expect(container.textContent).not.toContain('Old quoted body');

    fireEvent.click(getByRole('button', { name: 'Show previous messages' }));
    expect(container.textContent).toContain('Old quoted body');
    expect(getByRole('button', { name: 'Hide previous messages' })).toBeTruthy();
  });

  it('does not show a previous-messages toggle for explicit false or missing quote text', () => {
    const explicitFalse = render(<MessageBubble message={createMessage({
      viaChannel: 'email',
      emailVisibleText: 'Only reply',
      emailHasQuotedContent: false,
      emailProjectionVersion: 1,
    })} />);
    expect(explicitFalse.queryByRole('button', { name: 'Show previous messages' })).toBeNull();

    const missingText = render(<MessageBubble message={createMessage({
      viaChannel: 'email',
      emailVisibleText: 'Only reply',
      emailHasQuotedContent: true,
      emailQuotedText: '',
      emailProjectionVersion: 1,
    })} />);
    expect(missingText.queryByRole('button', { name: 'Show previous messages' })).toBeNull();
  });

  it('keeps email attachments visible while quoted history is collapsed or expanded', () => {
    const message = createMessage({
      viaChannel: 'email',
      emailVisibleText: 'See the files below',
      emailQuotedText: 'Earlier email',
      emailHasQuotedContent: true,
      attachments: [
        { id: 'image-1', fileKey: 'image-1', fileName: 'one.png', fileType: 'image/png', fileSize: 123, url: 'https://cdn.example.com/one.png' },
        { id: 'image-2', fileKey: 'image-2', fileName: 'two.png', fileType: 'image/png', fileSize: 456, url: 'https://cdn.example.com/two.png' },
        { id: 'file-1', fileKey: 'file-1', fileName: 'report.pdf', fileType: 'application/pdf', fileSize: 789, url: 'https://cdn.example.com/report.pdf' },
      ],
    });

    const rendered = render(<MessageBubble message={message} />);
    expect(rendered.getAllByRole('img')).toHaveLength(2);
    expect(rendered.getByRole('link', { name: /report\.pdf/i })).toBeTruthy();

    fireEvent.click(rendered.getByRole('button', { name: 'Show previous messages' }));
    expect(rendered.getAllByRole('img')).toHaveLength(2);
    expect(rendered.getByRole('link', { name: /report\.pdf/i })).toBeTruthy();
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
          image_url: 'https://example.com/pricing.png',
        },
      ],
    });
    const { container, getByRole } = render(<MessageBubble message={message} />);
    expect(container.textContent).toContain('Pricing');
    expect(container.textContent).not.toContain('Compare plans and limits.');
    expect(container.querySelector('img[src="https://example.com/pricing.png"]')).toBeNull();
    expect((getByRole('link', { name: /pricing/i }) as HTMLAnchorElement).href).toContain('https://example.com/pricing');
  });

  it('labels HTTP preview cards as not secure', () => {
    const message = createMessage({
      role: 'agent',
      linkPreviews: [{ url: 'http://example.com/pricing', title: 'Pricing', host: 'example.com' }],
    });
    const { container } = render(<MessageBubble message={message} />);
    expect(container.querySelector('.helpin-link-preview-security')?.textContent).toBe('Not secure');
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

  it('renders an empty streaming AI bubble with a live cursor', () => {
    const message = createMessage({ role: 'ai', content: '', isStreaming: true });
    const { container } = render(<MessageBubble message={message} />);

    expect(container.querySelector('.helpin-message--streaming')).toBeTruthy();
    expect(container.querySelector('.helpin-streaming-cursor')).toBeTruthy();
    expect(container.querySelector('.helpin-message-bubble')).toBeTruthy();
  });
});

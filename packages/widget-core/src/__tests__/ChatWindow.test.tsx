import { describe, expect, it, vi } from 'vitest';
import { act, render } from '@testing-library/preact';
import { ChatWindow } from '../components/ChatWindow';

const baseConfig = {
  workspaceId: 'ws_123',
  workspaceName: 'Acme',
  branding: {
    primaryColor: '#6366f1',
    welcomeMessage: 'How can we help?',
    widgetPosition: 'bottom-right' as const,
    showBranding: true,
    colorScheme: 'light' as const,
  },
  features: {
    aiEnabled: false,
    fileUploads: false,
    preChatForm: false,
    csatRating: false,
  },
};

describe('ChatWindow', () => {
  it('does not render when closed', () => {
    const { container } = render(
      <ChatWindow
        config={baseConfig}
        messages={[]}
        isOpen={false}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
      />,
    );

    expect(container.querySelector('.helpin-chat-window')).toBeFalsy();
  });

  it('uses a right-position modifier class without inline offsets', () => {
    const { container } = render(
      <ChatWindow
        config={baseConfig}
        messages={[]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
      />,
    );

    const windowEl = container.querySelector('.helpin-chat-window') as HTMLElement;
    expect(windowEl.className).toContain('helpin-chat-window--right');
    expect(windowEl.style.bottom).toBe('');
    expect(windowEl.style.right).toBe('');
  });

  it('keeps the window mounted briefly while closing for the exit transition', () => {
    vi.useFakeTimers();

    const { container, rerender } = render(
      <ChatWindow
        config={baseConfig}
        messages={[]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
      />,
    );

    rerender(
      <ChatWindow
        config={baseConfig}
        messages={[]}
        isOpen={false}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
      />,
    );

    const windowEl = container.querySelector('.helpin-chat-window') as HTMLElement;
    expect(windowEl).toBeTruthy();
    expect(windowEl.className).toContain('helpin-chat-window--hidden');

    act(() => {
      vi.advanceTimersByTime(220);
    });
    expect(container.querySelector('.helpin-chat-window')).toBeFalsy();

    vi.useRealTimers();
  });

  it('uses a left-position modifier class', () => {
    const { container } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          branding: {
            ...baseConfig.branding,
            widgetPosition: 'bottom-left',
          },
        }}
        messages={[]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
      />,
    );

    const windowEl = container.querySelector('.helpin-chat-window') as HTMLElement;
    expect(windowEl.className).toContain('helpin-chat-window--left');
  });

  it('renders the dedicated conversation view without tab navigation', () => {
    const { container, getByText } = render(
      <ChatWindow
        config={baseConfig}
        messages={[]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        initialView="conversation"
      />,
    );

    expect(getByText('The team can also help')).toBeTruthy();
    expect(getByText('How can we help?')).toBeTruthy();
    expect(container.querySelector('.helpin-bottom-nav')).toBeFalsy();
    expect(container.querySelector('.helpin-conversation-back')).toBeTruthy();
  });

  it('calls onClose when the close button is clicked', () => {
    const handleClose = vi.fn();
    const { container } = render(
      <ChatWindow
        config={baseConfig}
        messages={[]}
        isOpen={true}
        onClose={handleClose}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
      />,
    );

    container
      .querySelector('.helpin-window-close')
      ?.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    expect(handleClose).toHaveBeenCalled();
  });
});

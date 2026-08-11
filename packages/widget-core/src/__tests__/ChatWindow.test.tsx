import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render } from '@testing-library/preact';
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
    aiFirst: false,
    showTalkToHuman: false,
    escalationMessage: 'Let me connect you with a team member who can help further.',
    fileUploads: false,
    preChatForm: false,
    requirePhone: false,
    csatRating: false,
    forceIdentify: false,
  },
  availability: {
    isOnline: true,
    statusText: 'Online now',
    replyTimeText: 'We typically reply in a few minutes',
  },
};

describe('ChatWindow', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const sampleMessage = {
    id: 'msg-1',
    conversationId: 'conv-1',
    role: 'customer' as const,
    content: 'Need help',
    isInternal: false,
    createdAt: new Date().toISOString(),
  };

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

  it('uses the configured brand color and a contrasting icon color for the active rail item', () => {
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

    const navigation = container.querySelector('.helpin-bottom-nav') as HTMLElement;
    expect(navigation.style.getPropertyValue('--helpin-nav-active-color')).toBe('#6366f1');
    expect(navigation.style.getPropertyValue('--helpin-nav-active-foreground')).toBe('#ffffff');
    expect(container.querySelector('.helpin-bottom-nav-item--active')?.getAttribute('aria-current')).toBe('page');
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

  it('renders the Helpin logo mark with the powered-by text', () => {
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
      />,
    );

    expect(getByText('Powered by')).toBeTruthy();
    expect(getByText('Helpin')).toBeTruthy();
    const attribution = container.querySelector('a.helpin-powered-by.helpin-brand-attribution');
    expect(attribution).not.toBeNull();
    const attributionUrl = new URL(attribution?.getAttribute('href') ?? '');
    expect(attributionUrl.searchParams.get('utm_source')).toBe('acme-ws-123');
    expect(attributionUrl.searchParams.get('utm_medium')).toBe('referral');
    expect(attributionUrl.searchParams.get('utm_campaign')).toBe('powered_by_helpin');
    expect(attributionUrl.searchParams.get('utm_content')).toBe('chat_widget_footer');
    expect(attribution?.getAttribute('href')).not.toContain('amp;');
    expect(attribution?.children).toHaveLength(2);
    expect(attribution?.querySelector('.helpin-brand-attribution-brand')).not.toBeNull();
    const mark = attribution?.querySelector('.helpin-brand-attribution-icon');
    expect(mark).not.toBeNull();
    expect(mark?.tagName.toLowerCase()).toBe('svg');
    expect(mark?.getAttribute('fill')).toBe('currentColor');
    expect(mark?.getAttribute('aria-hidden')).toBe('true');
  });

  it('uses the inline header close button instead of a floating close button in help docs subviews', () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(JSON.stringify([]), { status: 200 })),
    );

    const handleClose = vi.fn();
    const { container, getByText } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          helpSpaces: [
            { id: 'space-1', name: 'Product Docs', slug: 'product-docs' },
            { id: 'space-2', name: 'Developer Docs', slug: 'developer-docs' },
          ],
        }}
        messages={[]}
        isOpen={true}
        initialView="help"
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        onClose={handleClose}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
      />,
    );

    fireEvent.click(getByText('Product Docs'));

    expect(container.querySelector('.helpin-window-close')).toBeNull();
    const inlineClose = container.querySelector('.helpin-help-header .helpin-window-close-inline');
    expect(inlineClose).toBeTruthy();

    fireEvent.click(inlineClose as Element);

    expect(handleClose).toHaveBeenCalled();
  });

  it('uses AI-first copy on the home view without leading with human availability', () => {
    const { getByText } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          features: {
            ...baseConfig.features,
            aiEnabled: true,
            aiFirst: true,
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

    expect(getByText('Ask a question')).toBeTruthy();
    expect(getByText('Get an instant answer from our AI assistant')).toBeTruthy();
  });

  it('keeps showing the support roster on the home view when one teammate is assigned', () => {
    const { container } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          availableTeammates: [
            { userId: 'user-1', name: 'CS Azhar', avatarUrl: 'https://example.com/avatar.png', status: 'online' as const },
            { userId: 'user-2', name: 'Nora Support', avatarUrl: 'https://example.com/nora.png', status: 'away' as const },
          ],
        }}
        messages={[]}
        activeTeammate={{ userId: 'user-1', name: 'CS Azhar', avatarUrl: 'https://example.com/avatar.png', status: 'online' }}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
      />,
    );

    expect(container.querySelectorAll('.helpin-home-header-presence')).toHaveLength(2);
    expect(container.querySelector('.helpin-presence-dot--online')).toBeTruthy();
    expect(container.querySelector('.helpin-presence-dot--away')).toBeTruthy();
  });

  it('starts a fresh conversation from the home CTA', () => {
    const handleStartNewConversation = vi.fn();
    const { getByText } = render(
      <ChatWindow
        config={baseConfig}
        messages={[]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        onStartNewConversation={handleStartNewConversation}
      />,
    );

    fireEvent.click(getByText('Send us a message'));

    expect(handleStartNewConversation).toHaveBeenCalledTimes(1);
  });

  it('does not show Contact us in the Help tab', () => {
    const { getByText, queryByText } = render(
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

    fireEvent.click(getByText('Help center'));

    expect(queryByText('Contact us')).toBeNull();
  });

  it('starts a fresh conversation from empty Messages view', () => {
    const handleStartNewConversation = vi.fn();
    const { getByText } = render(
      <ChatWindow
        config={baseConfig}
        messages={[]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        onStartNewConversation={handleStartNewConversation}
        initialView="messages"
      />,
    );

    fireEvent.click(getByText('Ask a question'));

    expect(handleStartNewConversation).toHaveBeenCalledTimes(1);
  });

  it('does not show talk to human on the first customer message', () => {
    const { queryByText } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          features: {
            ...baseConfig.features,
            showTalkToHuman: true,
          },
        }}
        messages={[sampleMessage]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        onEscalateToHuman={() => {}}
        initialView="conversation"
      />,
    );

    expect(queryByText('Talk to a human')).toBeNull();
  });

  it('shows talk to human when enabled after an AI reply and the conversation is idle', () => {
    const { getByText } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          features: {
            ...baseConfig.features,
            showTalkToHuman: true,
          },
        }}
        messages={[
          sampleMessage,
          {
            id: 'msg-2',
            conversationId: 'conv-1',
            role: 'ai' as const,
            content: 'I found the setup steps.',
            senderName: 'Helpin AI',
            isInternal: false,
            createdAt: new Date().toISOString(),
          },
        ]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        onEscalateToHuman={() => {}}
        initialView="conversation"
      />,
    );

    expect(getByText('Talk to a human')).toBeTruthy();
  });

  it('reveals human handoff status only after the visitor asks for a human', () => {
    const { getByText, queryByText } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          features: {
            ...baseConfig.features,
            aiEnabled: true,
            aiFirst: true,
            showTalkToHuman: true,
            escalationMessage: "I'm handing this over to a human teammate now.",
          },
        }}
        messages={[
          sampleMessage,
          {
            id: 'msg-2',
            conversationId: 'conv-1',
            role: 'ai' as const,
            content: 'I can help with that.',
            senderName: 'Helpin AI',
            isInternal: false,
            createdAt: new Date().toISOString(),
          },
        ]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        onEscalateToHuman={() => {}}
        initialView="conversation"
      />,
    );

    expect(queryByText('We typically reply in a few minutes')).toBeNull();

    fireEvent.click(getByText('Talk to a human'));

    expect(getByText('We typically reply in a few minutes')).toBeTruthy();
    expect(queryByText('Talk to a human')).toBeNull();
  });

  it('shows waiting for teammate after an escalated system handoff message', () => {
    const { getByText, container } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          availableTeammates: [
            { userId: 'user-1', name: 'CS Azhar', status: 'online' as const },
            { userId: 'user-2', name: 'Nora Support', status: 'away' as const },
          ],
        }}
        messages={[
          sampleMessage,
          {
            id: 'msg-2',
            conversationId: 'conv-1',
            role: 'system' as const,
            content: 'Let me connect you with a team member who can help further.',
            isInternal: false,
            createdAt: new Date().toISOString(),
          },
        ]}
        activeConversation={{
          id: 'conv-1',
          subject: 'Need help',
          status: 'open',
          aiState: 'escalated',
          flowState: 'waiting_for_human',
        }}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        initialView="conversation"
      />,
    );

    expect(getByText('Let me connect you with a team member who can help further.')).toBeTruthy();
    expect(getByText('A team member will reply soon')).toBeTruthy();
    expect(container.querySelectorAll('.helpin-waiting-teammate-avatar').length).toBe(2);
  });

  it('hides talk to human once an escalation message is already in the thread', () => {
    const { getByText, queryByText } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          features: {
            ...baseConfig.features,
            showTalkToHuman: true,
            escalationMessage: 'A teammate will join shortly.',
          },
        }}
        messages={[
          sampleMessage,
          {
            id: 'msg-2',
            conversationId: 'conv-1',
            role: 'system' as const,
            content: 'A teammate will join shortly.',
            systemEventType: 'ai_escalated' as const,
            isInternal: false,
            createdAt: new Date().toISOString(),
          },
        ]}
        activeConversation={{
          id: 'conv-1',
          subject: 'Need help',
          status: 'open',
        }}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        onEscalateToHuman={() => {}}
        initialView="conversation"
      />,
    );

    expect(getByText('A teammate will join shortly.')).toBeTruthy();
    expect(getByText('A team member will reply soon')).toBeTruthy();
    expect(queryByText('Talk to a human')).toBeNull();
  });

  it('groups consecutive Helpin AI handoff and reply messages under one sender label', () => {
    const { container } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          features: {
            ...baseConfig.features,
            aiEnabled: true,
            aiFirst: true,
          },
        }}
        messages={[
          sampleMessage,
          {
            id: 'msg-2',
            conversationId: 'conv-1',
            role: 'system' as const,
            content: 'Let me connect you with a team member who can help further.',
            senderName: 'Helpin AI',
            isInternal: false,
            createdAt: new Date().toISOString(),
          },
          {
            id: 'msg-3',
            conversationId: 'conv-1',
            role: 'ai' as const,
            content: 'A teammate will respond shortly.',
            isInternal: false,
            createdAt: new Date().toISOString(),
          },
        ]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        initialView="conversation"
      />,
    );

    const senderLabels = Array.from(container.querySelectorAll('.helpin-message-agent-name'))
      .map((node) => node.textContent)
      .filter((text) => text === 'Helpin AI');
    expect(senderLabels).toHaveLength(1);
  });

  it('shows the active teammate in conversation header before a human reply is sent', () => {
    const { getByText, container } = render(
      <ChatWindow
        config={baseConfig}
        messages={[]}
        activeTeammate={{ userId: 'user-1', name: 'CS Azhar', status: 'away' }}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        initialView="conversation"
      />,
    );

    expect(getByText('CS Azhar')).toBeTruthy();
    expect(getByText('from Acme')).toBeTruthy();
    expect(container.querySelector('.helpin-presence-dot--away')).toBeTruthy();
  });

  it('hides talk to human while AI is thinking', () => {
    const { container, getByRole, queryByText } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          features: {
            ...baseConfig.features,
            showTalkToHuman: true,
          },
        }}
        messages={[sampleMessage]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        onEscalateToHuman={() => {}}
        isAIThinking={true}
        initialView="conversation"
      />,
    );

    expect(queryByText('Talk to a human')).toBeNull();
    expect(getByRole('status').textContent).toContain('Looking into this…');
    expect(container.querySelector('.helpin-ai-thinking-icon svg')).toBeTruthy();
    expect(container.querySelectorAll('.helpin-ai-thinking-line')).toHaveLength(2);
    expect(queryByText('Thinking')).toBeNull();
  });

  it('hides talk to human while an agent is typing', () => {
    const { queryByText } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          features: {
            ...baseConfig.features,
            showTalkToHuman: true,
          },
        }}
        messages={[sampleMessage]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        onEscalateToHuman={() => {}}
        isTyping={true}
        initialView="conversation"
      />,
    );

    expect(queryByText('Talk to a human')).toBeNull();
  });

  it('shows a reconnecting banner while the widget is temporarily disconnected', () => {
    const { getByText, queryByText } = render(
      <ChatWindow
        config={baseConfig}
        messages={[sampleMessage]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        initialView="conversation"
        connectionStatus="disconnected"
      />,
    );

    expect(getByText('Connection lost. Reconnecting...')).toBeTruthy();
    expect(queryByText('Reconnect')).toBeNull();
  });

  it('shows a reconnect prompt and disables the composer after prolonged disconnection', () => {
    const handleRetry = vi.fn();
    const { getByText, container } = render(
      <ChatWindow
        config={baseConfig}
        messages={[sampleMessage]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        initialView="conversation"
        connectionStatus="failed"
        onRetryConnection={handleRetry}
      />,
    );

    expect(getByText("We've been offline for a while. We'll keep trying in the background, or reconnect now.")).toBeTruthy();

    fireEvent.click(getByText('Reconnect'));
    expect(handleRetry).toHaveBeenCalledTimes(1);

    const textarea = container.querySelector('.helpin-compose-input') as HTMLTextAreaElement;
    expect(textarea.disabled).toBe(true);
    expect(textarea.placeholder).toBe('Offline. Reconnecting in the background...');
  });

  it('hides talk to human after a human teammate has already replied', () => {
    const { queryByText } = render(
      <ChatWindow
        config={{
          ...baseConfig,
          features: {
            ...baseConfig.features,
            showTalkToHuman: true,
          },
        }}
        messages={[
          sampleMessage,
          {
            id: 'msg-2',
            conversationId: 'conv-1',
            role: 'agent' as const,
            content: 'I can help with that.',
            senderName: 'CS Azhar',
            isInternal: false,
            createdAt: new Date().toISOString(),
          },
        ]}
        isOpen={true}
        onClose={() => {}}
        onSendMessage={() => {}}
        onQuickReply={() => {}}
        showPreChatForm={false}
        onPreChatSubmit={() => {}}
        onEscalateToHuman={() => {}}
        initialView="conversation"
      />,
    );

    expect(queryByText('Talk to a human')).toBeNull();
  });
});

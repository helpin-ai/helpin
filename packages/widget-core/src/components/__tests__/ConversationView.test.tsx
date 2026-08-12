import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/preact';
import { ConversationView } from '../ConversationView';
import type { Conversation, Message, WidgetConfig } from '../../types';

const baseConfig: WidgetConfig = {
  workspaceId: 'ws_123',
  workspaceName: 'Acme',
  branding: {
    primaryColor: '#6366f1',
    welcomeMessage: 'How can we help?',
    widgetPosition: 'bottom-right',
    showBranding: true,
    colorScheme: 'light',
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

interface RenderOverrides {
  conversation?: Partial<Conversation>;
  messages?: Message[];
  transcriptEmail?: string;
}

function renderConversationView(overrides: RenderOverrides = {}) {
  const { conversation: conversationOverrides, messages, transcriptEmail } = overrides;
  const conversation: Conversation | undefined = conversationOverrides
    ? ({ subject: 'Help request', ...conversationOverrides } as Conversation)
    : undefined;

  return render(
    <ConversationView
      config={baseConfig}
      conversation={conversation}
      messages={messages ?? []}
      onSendMessage={() => {}}
      onBack={() => {}}
      showHumanAvailability={true}
      transcriptEmail={transcriptEmail}
      onRequestTranscript={async () => ({ success: true, message: 'Transcript sent' })}
    />,
  );
}

describe('ConversationView attribution', () => {
  it('identifies the workspace and composer placement with consistent UTMs', () => {
    const { container } = renderConversationView();
    const attribution = container.querySelector('a.helpin-compose-footer');
    const attributionUrl = new URL(attribution?.getAttribute('href') ?? '');

    expect(attributionUrl.searchParams.get('utm_source')).toBe('acme-ws-123');
    expect(attributionUrl.searchParams.get('utm_medium')).toBe('referral');
    expect(attributionUrl.searchParams.get('utm_campaign')).toBe('powered_by_helpin');
    expect(attributionUrl.searchParams.get('utm_content')).toBe('chat_widget_composer');
    expect(attribution?.getAttribute('href')).not.toContain('amp;');
  });
});

describe('ConversationView escalation email capture', () => {
  it('shows email-capture card for anonymous visitor when busy', () => {
    const { getByText, getByPlaceholderText } = renderConversationView({
      conversation: { id: 'c1', status: 'open', aiState: 'escalated', handoffState: 'busy' },
      transcriptEmail: undefined,
    });
    expect(getByText(/reply there too/i)).toBeTruthy();
    expect(getByPlaceholderText(/you@/i)).toBeTruthy();
  });

  it('hides email-capture card when visitor email is known', () => {
    const { queryByText } = renderConversationView({
      conversation: { id: 'c1', status: 'open', aiState: 'escalated', handoffState: 'after_hours' },
      transcriptEmail: 'known@example.com',
    });
    expect(queryByText(/reply there too/i)).toBeNull();
  });

  it('does not show email-capture card in live state', () => {
    const { queryByText } = renderConversationView({
      conversation: { id: 'c1', status: 'open', aiState: 'escalated', handoffState: 'live' },
      transcriptEmail: undefined,
    });
    expect(queryByText(/reply there too/i)).toBeNull();
  });
});

describe('ConversationView teammate_joined system line', () => {
  it('renders teammate_joined as a system line', () => {
    const { getByText } = renderConversationView({
      conversation: { id: 'c1', status: 'open' },
      messages: [
        {
          id: 'm1',
          conversationId: 'c1',
          role: 'system',
          systemEventType: 'teammate_joined',
          content: 'Sara joined the conversation',
          isInternal: false,
          createdAt: new Date().toISOString(),
        },
      ],
    });
    expect(getByText(/joined the conversation/i)).toBeTruthy();
  });
});

describe('ConversationView AI progress', () => {
  it('keeps the shimmer while announcing the current safe progress stage', () => {
    const { container, getByRole } = render(
      <ConversationView
        config={baseConfig}
        messages={[]}
        onSendMessage={() => {}}
        onBack={() => {}}
        isAIThinking={true}
        aiProgressLabel="Checking the details…"
      />,
    );

    expect(getByRole('status').textContent).toContain('Checking the details…');
    expect(container.querySelector('.helpin-ai-thinking-shimmer')).toBeTruthy();
    expect(container.querySelectorAll('.helpin-ai-thinking-line')).toHaveLength(2);
  });
});

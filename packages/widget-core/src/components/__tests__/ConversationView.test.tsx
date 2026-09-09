import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/preact';
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
  config?: WidgetConfig;
  conversation?: Partial<Conversation>;
  messages?: Message[];
  transcriptEmail?: string;
}

function renderConversationView(overrides: RenderOverrides = {}) {
  const { config = baseConfig, conversation: conversationOverrides, messages, transcriptEmail } = overrides;
  const conversation: Conversation | undefined = conversationOverrides
    ? ({ subject: 'Help request', ...conversationOverrides } as Conversation)
    : undefined;

  return render(
    <ConversationView
      config={config}
      conversation={conversation}
      messages={messages ?? []}
      onSendMessage={() => {}}
      onBack={() => {}}
      showHumanAvailability={true}
      transcriptEmail={transcriptEmail}
      onCaptureEmail={async () => {}}
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

describe('ConversationView branding', () => {
  it('places a transparent workspace logo on the configured brand color', () => {
    const config: WidgetConfig = {
      ...baseConfig,
      branding: {
        ...baseConfig.branding,
        logoUrl: 'https://cdn.example.com/logo.svg',
        primaryColor: '#0068e5',
      },
    };

    const { container } = renderConversationView({ config });
    const surface = container.querySelector('.helpin-conversation-logo--brand');
    const logo = surface?.querySelector('.helpin-conversation-brand-logo');
    const messageAvatar = container.querySelector('.helpin-message-avatar--brand');
    const messageLogo = messageAvatar?.querySelector('.helpin-message-brand-logo');

    expect(surface).toBeTruthy();
    expect((surface as HTMLElement | null)?.style.backgroundColor).toBe('rgb(0, 104, 229)');
    expect(logo?.getAttribute('alt')).toBe('Acme');
    expect((messageAvatar as HTMLElement | null)?.style.backgroundColor).toBe('rgb(0, 104, 229)');
    expect(messageLogo?.getAttribute('alt')).toBe('Acme');
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

  it('respects a completed contact choice during a busy handoff', () => {
    const conversation: Conversation = {
      id: 'c1',
      subject: 'Help request',
      status: 'open',
      aiState: 'escalated',
      handoffState: 'busy',
    };
    const { queryByText } = render(
      <ConversationView
        config={baseConfig}
        conversation={conversation}
        messages={[]}
        onSendMessage={() => {}}
        onBack={() => {}}
        showHumanAvailability={true}
        contactCaptureCompleted={true}
        onRequestTranscript={async () => ({ success: true, message: 'Transcript sent' })}
      />,
    );

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

describe('ConversationView CSAT', () => {
  const exchange: Message[] = [
    {
      id: 'customer-1', conversationId: 'c1', role: 'customer', content: 'Can you help?',
      isInternal: false, createdAt: new Date().toISOString(),
    },
    {
      id: 'ai-1', conversationId: 'c1', role: 'ai', content: 'Yes, this is now resolved.',
      isInternal: false, createdAt: new Date().toISOString(),
    },
  ];
  const csatConfig: WidgetConfig = {
    ...baseConfig,
    features: { ...baseConfig.features, csatRating: true },
  };

  it('shows CSAT after a qualifying conversation is resolved', () => {
    const onCsatSubmit = vi.fn();
    const rendered = render(
      <ConversationView
        config={csatConfig}
        conversation={{ id: 'c1', subject: 'Help', status: 'resolved' }}
        messages={exchange}
        onSendMessage={() => {}}
        onBack={() => {}}
        onCsatSubmit={onCsatSubmit}
      />,
    );

    expect(rendered.getByText('How was your support experience?')).toBeTruthy();
    fireEvent.click(rendered.getByRole('radio', { name: 'Very satisfied' }));
    fireEvent.click(rendered.getByRole('button', { name: 'Send feedback' }));
    expect(onCsatSubmit).toHaveBeenCalledWith(5, '');
  });

  it('does not show CSAT for open, one-sided, disabled, or previously rated conversations', () => {
    const common = {
      onSendMessage: () => {},
      onBack: () => {},
      onCsatSubmit: () => {},
    };
    const open = render(<ConversationView {...common} config={csatConfig} conversation={{ id: 'c1', subject: 'Help', status: 'open' }} messages={exchange} />);
    const oneSided = render(<ConversationView {...common} config={csatConfig} conversation={{ id: 'c1', subject: 'Help', status: 'resolved' }} messages={[exchange[0]]} />);
    const disabled = render(<ConversationView {...common} config={baseConfig} conversation={{ id: 'c1', subject: 'Help', status: 'resolved' }} messages={exchange} />);
    const submitted = render(<ConversationView {...common} config={csatConfig} conversation={{ id: 'c1', subject: 'Help', status: 'resolved' }} messages={exchange} csatSubmitted />);

    expect(open.queryByText('How was your support experience?')).toBeNull();
    expect(oneSided.queryByText('How was your support experience?')).toBeNull();
    expect(disabled.queryByText('How was your support experience?')).toBeNull();
    expect(submitted.queryByText('How was your support experience?')).toBeNull();
  });
});


describe('delayed team reply contact capture', () => {
  const delayed: Message = { id: 'delay', conversationId: 'c1', role: 'ai', content: 'Leave your email for a reply.', delayedTeamReply: true, captureEmail: true, isInternal: false, createdAt: new Date().toISOString() };
  it('reuses one form during a busy handoff and offers it after an earlier skip', () => {
    const { container } = render(<ConversationView config={baseConfig} conversation={{ id: 'c1', subject: '', status: 'open', handoffState: 'busy' }} messages={[delayed]} contactCaptureCompleted onCaptureEmail={async () => {}} onSendMessage={() => {}} onBack={() => {}} />);
    expect(container.querySelectorAll('input[type="email"]')).toHaveLength(1);
  });
  it('replaces joining and reply-time promises with a neutral waiting status after the delay', () => {
    const { getByText, queryByText, rerender } = render(<ConversationView config={baseConfig} conversation={{ id: 'c1', subject: '', status: 'open', flowState: 'assigned_to_human' }} messages={[{ ...delayed, captureEmail: false }]} transcriptEmail="known@example.com" onSendMessage={() => {}} onBack={() => {}} />);
    expect(getByText('Waiting for a teammate')).toBeTruthy();
    expect(queryByText(/is joining|shortly|typically reply/)).toBeNull();
    const human: Message = { ...delayed, id: 'human', role: 'agent', delayedTeamReply: false, captureEmail: false };
    rerender(<ConversationView config={baseConfig} conversation={{ id: 'c1', subject: '', status: 'open', flowState: 'assigned_to_human' }} messages={[delayed, human]} onSendMessage={() => {}} onBack={() => {}} />);
    expect(queryByText('Waiting for a teammate')).toBeNull();
  });
  it('offers capture for a new handoff after an earlier human reply, but stops after a new reply', () => {
    const human: Message = { ...delayed, id: 'human', role: 'agent', delayedTeamReply: false, captureEmail: false };
    const props = { config: baseConfig, conversation: { id: 'c1', subject: '', status: 'open', handoffState: 'live' as const }, onCaptureEmail: async () => {}, onSendMessage: () => {}, onBack: () => {} };
    const { getByRole, queryByRole, rerender } = render(<ConversationView {...props} messages={[human, delayed]} />);
    expect(getByRole('textbox', { name: 'Email for reply notifications' })).toBeTruthy();
    rerender(<ConversationView {...props} messages={[delayed, human]} />);
    expect(queryByRole('textbox', { name: 'Email for reply notifications' })).toBeNull();
    rerender(<ConversationView {...props} messages={[delayed]} transcriptEmail="known@example.com" />);
    expect(queryByRole('textbox', { name: 'Email for reply notifications' })).toBeNull();
  });
  it('waits for saved identity and never sends a transcript or escalates again', async () => {
    let confirm!: () => void;
    const save = vi.fn(() => new Promise<void>((resolve) => { confirm = resolve; }));
    const transcript = vi.fn();
    const escalate = vi.fn();
    const { getByRole, queryByText } = render(<ConversationView config={baseConfig} conversation={{ id: 'c1', subject: '', status: 'open', handoffState: 'live' }} messages={[delayed]} onCaptureEmail={save} onRequestTranscript={transcript} onEscalateToHuman={escalate} onSendMessage={() => {}} onBack={() => {}} />);
    fireEvent.input(getByRole('textbox', { name: 'Email for reply notifications' }), { target: { value: 'visitor@example.com' } });
    fireEvent.submit(getByRole('button', { name: 'Notify me by email' }).closest('form')!);
    expect(save).toHaveBeenCalledWith('visitor@example.com');
    expect(queryByText(/You’re all set/)).toBeNull();
    confirm();
    await waitFor(() => expect(queryByText(/You’re all set/)).not.toBeNull());
    expect(transcript).not.toHaveBeenCalled();
    expect(escalate).not.toHaveBeenCalled();
  });
  it('retains the form and shows an error if identity save fails', async () => {
    const { getByRole } = render(<ConversationView config={baseConfig} conversation={{ id: 'c1', subject: '', status: 'open', handoffState: 'live' }} messages={[delayed]} onCaptureEmail={async () => { throw new Error('failed'); }} onSendMessage={() => {}} onBack={() => {}} />);
    fireEvent.input(getByRole('textbox', { name: 'Email for reply notifications' }), { target: { value: 'visitor@example.com' } });
    fireEvent.submit(getByRole('button', { name: 'Notify me by email' }).closest('form')!);
    await waitFor(() => expect(getByRole('alert').textContent).toContain('try again'));
    expect(getByRole('textbox', { name: 'Email for reply notifications' })).toBeTruthy();
  });
});

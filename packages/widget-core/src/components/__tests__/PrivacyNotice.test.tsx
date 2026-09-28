import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/preact';
import { ConversationView } from '../ConversationView';
import { ChatWindow } from '../ChatWindow';
import type { Message, WidgetConfig } from '../../types';

const config: WidgetConfig = {
  workspaceId: 'privacy-test',
  branding: { primaryColor: '#6366f1', welcomeMessage: 'Hello', widgetPosition: 'bottom-right', showBranding: false },
  features: { aiEnabled: true, aiFirst: true, showTalkToHuman: false, fileUploads: false, preChatForm: false, requirePhone: false, csatRating: false, forceIdentify: false },
  availability: { isOnline: true, statusText: 'Online', replyTimeText: '' },
  privacyNotice: { enabled: true, policyUrl: 'https://example.com/privacy', text: 'By chatting with us, you agree to our' },
};
const message: Message = { id: 'reply', conversationId: 'one', role: 'customer', content: 'Hello', isInternal: false, createdAt: '2026-09-28T12:00:00Z' };
const view = (messages: Message[] = [], id?: string, customConfig = config) => (
  <ConversationView config={customConfig} conversation={id ? { id, subject: 'Help', status: 'open' } : undefined} messages={messages} onSendMessage={vi.fn()} onBack={vi.fn()} />
);

beforeEach(() => localStorage.clear());

describe('Conversation privacy notice', () => {
  it('shows above the composer independently of Helpin branding and opens the policy safely', () => {
    const { container, getByRole } = render(view());
    const notice = getByRole('complementary', { name: 'Chat privacy notice' });
    expect(notice.parentElement?.className).toBe('helpin-compose-wrapper');
    expect(container.querySelector('.helpin-compose-footer')).toBeNull();
    const link = getByRole('link', { name: 'Privacy Policy' });
    expect(link.getAttribute('href')).toBe('https://example.com/privacy');
    expect(link.getAttribute('target')).toBe('_blank');
    expect(link.getAttribute('rel')).toContain('noopener');
  });

  it('waits for a confirmed customer message, including attachment-only messages', () => {
    const { queryByRole, rerender } = render(view([{ ...message, deliveryStatus: 'sending' }]));
    expect(queryByRole('complementary')).toBeTruthy();
    rerender(view([{ ...message, content: '', attachments: [{ fileKey: 'file', fileName: 'photo.png', fileType: 'image/png', fileSize: 10 }] }]));
    expect(queryByRole('complementary')).toBeNull();
  });

  it('keeps dismissal for the same conversation but shows it for a new one', () => {
    const greeting = { ...message, role: 'ai' as const };
    const first = render(view([greeting], 'one'));
    fireEvent.click(first.getByRole('button', { name: 'Dismiss privacy notice' }));
    expect(first.queryByRole('complementary')).toBeNull();
    fireEvent.click(first.getByRole('button', { name: 'More options' }));
    expect(first.getByRole('link', { name: 'Privacy Policy' }).getAttribute('href')).toBe('https://example.com/privacy');
    first.unmount();
    const second = render(view([greeting], 'one'));
    expect(second.queryByRole('complementary')).toBeNull();
    second.rerender(view([greeting], 'two'));
    expect(second.queryByRole('complementary')).toBeTruthy();
  });

  it('does not show while an existing conversation is loading or after a past customer reply', () => {
    const { queryByRole, rerender } = render(view([], 'one'));
    expect(queryByRole('complementary')).toBeNull();
    rerender(view([message], 'one'));
    expect(queryByRole('complementary')).toBeNull();
    rerender(view());
    expect(queryByRole('complementary')).toBeTruthy();
  });

  it('keeps a dismissed draft hidden when the widget closes and reopens', async () => {
    const props = { config, messages: [], onClose: vi.fn(), onSendMessage: vi.fn(), onQuickReply: vi.fn(), showPreChatForm: false, onPreChatSubmit: vi.fn(), initialView: 'conversation' as const };
    const { getByRole, queryByRole, rerender } = render(<ChatWindow {...props} isOpen />);
    fireEvent.click(getByRole('button', { name: 'Dismiss privacy notice' }));
    rerender(<ChatWindow {...props} isOpen={false} />);
    await waitFor(() => expect(queryByRole('button', { name: 'More options' })).toBeNull());
    rerender(<ChatWindow {...props} isOpen />);
    await waitFor(() => expect(getByRole('button', { name: 'More options' })).toBeTruthy());
    expect(queryByRole('complementary')).toBeNull();
    rerender(<ChatWindow {...props} isOpen activeConversation={{ id: 'created', subject: 'Help', status: 'open' }} messages={[{ ...message, deliveryStatus: 'sending' }]} />);
    expect(queryByRole('complementary')).toBeNull();
  });

  it('still dismisses when browser storage is blocked', () => {
    const storage = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked'); });
    try {
      const { getByRole, queryByRole } = render(view([{ ...message, role: 'ai' }], 'one'));
      fireEvent.click(getByRole('button', { name: 'Dismiss privacy notice' }));
      expect(queryByRole('complementary')).toBeNull();
    } finally { storage.mockRestore(); }
  });

  it.each([undefined, { ...config.privacyNotice!, enabled: false }, { ...config.privacyNotice!, policyUrl: 'javascript:alert(1)' }, { ...config.privacyNotice!, policyUrl: 'https:example.com' }])('omits unconfigured, disabled and unsafe policies', (privacyNotice) => {
    const { queryByRole, getByRole } = render(view([], undefined, { ...config, privacyNotice }));
    expect(queryByRole('complementary')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'More options' }));
    expect(queryByRole('link', { name: 'Privacy Policy' })).toBeNull();
  });
});

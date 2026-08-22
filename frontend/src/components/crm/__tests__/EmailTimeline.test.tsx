// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { EmailTimeline } from '../EmailTimeline';

const emailState = vi.hoisted(() => ({
  threads: [] as Array<Record<string, unknown>>,
  detail: undefined as Record<string, unknown> | undefined,
}));

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
}));

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: () => ({ currentWorkspace: { slug: 'test' } }),
}));

vi.mock('@/stores/authStore', () => ({
  useAuthStore: (selector: (state: { user: { id: string } }) => unknown) => selector({ user: { id: 'member-1' } }),
}));

vi.mock('@/hooks/useAccessibleTeams', () => ({
  useAccessibleTeams: () => ({ teams: [] }),
}));

vi.mock('@/hooks/queries', () => ({
  useCreateTask: () => ({ mutateAsync: vi.fn() }),
}));

vi.mock('@/hooks/queries/useCRM', () => {
  const emailQuery = {
    get data() { return { pages: [{ data: emailState.threads }] }; },
    isLoading: false,
    isError: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
    refetch: vi.fn(),
  };

  return {
    useInfiniteEmailThreads: () => emailQuery,
    useEmailThread: () => ({ data: emailState.detail, isLoading: false }),
    useReplyToEmailThread: () => ({ mutateAsync: vi.fn(), isPending: false }),
    useSetEmailThreadDismissed: () => ({ mutateAsync: vi.fn() }),
    useLinkEmailThreadDeal: () => ({ mutateAsync: vi.fn() }),
    useEmailAccounts: () => ({
      data: [{
        id: 'account-1',
        member_id: 'member-1',
        email_address: 'owner@example.com',
        provider: 'gmail',
        is_active: true,
        status: 'connected',
        can_send: true,
      }],
    }),
  };
});

vi.mock('../CRMEmailComposerDialog', () => ({
  CRMEmailComposerDialog: ({ draft }: { draft?: { to?: string[] } }) => (
    <div data-testid="crm-email-composer">{draft?.to?.join(', ')}</div>
  ),
}));

vi.mock('@/components/pm/CreateTaskModal', () => ({
  CreateTaskModal: () => null,
}));

vi.mock('@/components/support/EmailBodyRenderer', () => ({
  EmailBodyRenderer: ({ html }: { html: string }) => <iframe title="Email body" srcDoc={html} />,
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  emailState.threads = [];
  emailState.detail = undefined;
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.clearAllMocks();
});

describe('EmailTimeline', () => {
  it('opens the composer from an empty timeline and prefills the contact email', () => {
    act(() => {
      root.render(
        <EmailTimeline
          workspaceId="workspace-1"
          contactId="contact-1"
          defaultRecipient="buyer@example.com"
        />,
      );
    });

    const composeButton = [...container.querySelectorAll('button')]
      .find((button) => button.textContent?.includes('Compose'));

    expect(composeButton).toBeDefined();
    act(() => composeButton?.click());

    const composer = container.querySelector('[data-testid="crm-email-composer"]');
    expect(composer?.textContent).toBe('buyer@example.com');
  });

  it('supports the shared company email view without requiring a default recipient', () => {
    act(() => {
      root.render(
        <EmailTimeline
          workspaceId="workspace-1"
          companyId="company-1"
        />,
      );
    });

    expect(container.textContent).toContain('No conversations found');
    expect(container.textContent).toContain('Compose');
  });

  it('keeps another member\'s mailbox readable but does not render reply controls', () => {
    emailState.threads = [{
      id: 'thread-1',
      workspace_id: 'workspace-1',
      email_account_id: 'account-2',
      thread_external_id: 'external-1',
      subject: 'Renewal question',
      last_message_at: '2026-08-21T10:00:00Z',
      message_count: 1,
      contact_ids: ['contact-1'],
      can_reply: false,
      needs_reply: true,
      mailbox_email: 'colleague@example.com',
      mailbox_provider: 'gmail',
      latest_message: {
        id: 'message-1',
        from_address: 'buyer@example.com',
        to_addresses: ['colleague@example.com'],
        cc_addresses: [],
        direction: 'inbound',
        sent_at: '2026-08-21T10:00:00Z',
        subject: 'Renewal question',
        body_text: 'Can we discuss renewal?',
        body_html: '<p>Can we discuss renewal?</p>',
      },
    }];
    emailState.detail = {
      thread: emailState.threads[0],
      participants: [{ email: 'buyer@example.com', role: 'from', contact_id: 'contact-1', contact_name: 'Buyer' }],
      messages: [(emailState.threads[0] as { latest_message: unknown }).latest_message],
    };

    act(() => {
      root.render(<EmailTimeline workspaceId="workspace-1" contactId="contact-1" />);
    });

    expect(container.textContent).toContain('Renewal question');
    expect(container.textContent).toContain('Read-only conversation');
    expect(container.textContent).toContain('only the person who connected colleague@example.com can reply');
    expect(container.querySelector('iframe[title="Email body"]')).not.toBeNull();
    expect([...container.querySelectorAll('button')].some((button) => button.textContent?.trim() === 'Send reply')).toBe(false);
  });

  it('keeps the shared PM divider editor in a bottom-pinned reply region', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/components/crm/EmailTimeline.tsx'), 'utf8');

    expect(source).toContain('id="crm-thread-reply"');
    expect(source).toContain('shrink-0 overflow-y-auto border-t');
    expect(source).toContain('variant="divider"');
    expect(source).toContain('contentVariant="pm"');
  });
});

// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { EmailTimeline } from '../EmailTimeline';

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
}));

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: () => ({ currentWorkspace: { slug: 'test' } }),
}));

vi.mock('@/hooks/queries/useCRM', () => {
  const emptyEmailQuery = {
    data: { pages: [{ data: [] }] },
    isLoading: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  };

  return {
    useInfiniteContactEmails: () => emptyEmailQuery,
    useInfiniteDealEmails: () => emptyEmailQuery,
    useEmailAccounts: () => ({
      data: [{
        id: 'account-1',
        email_address: 'owner@example.com',
        is_active: true,
        status: 'connected',
      }],
    }),
  };
});

vi.mock('../CRMEmailComposerDialog', () => ({
  CRMEmailComposerDialog: ({ draft }: { draft?: { to?: string[] } }) => (
    <div data-testid="crm-email-composer">{draft?.to?.join(', ')}</div>
  ),
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
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
      .find((button) => button.textContent?.includes('Compose email'));

    expect(composeButton).toBeDefined();
    act(() => composeButton?.click());

    const composer = container.querySelector('[data-testid="crm-email-composer"]');
    expect(composer?.textContent).toBe('buyer@example.com');
  });
});

// @vitest-environment jsdom

import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ActivityTimeline } from '../ActivityTimeline';

const { createActivity } = vi.hoisted(() => ({
  createActivity: vi.fn(),
}));

vi.mock('@/hooks/queries/useCRM', () => ({
  useCreateCRMActivity: () => ({ mutateAsync: createActivity, isPending: false }),
  useUpdateCRMActivity: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteCRMActivity: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useEmailAccounts: () => ({ data: [] }),
}));

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('../CRMEmailComposerDialog', () => ({
  CRMEmailComposerDialog: ({ open, draft }: { open: boolean; draft?: { to?: string[] } }) => open
    ? <div data-testid="email-composer">{draft?.to?.join(', ')}</div>
    : null,
}));

vi.mock('../EmailTimeline', () => ({
  EmailTimeline: ({ contactId, companyId }: { contactId?: string; companyId?: string }) => (
    <div data-testid="rich-email-timeline">{contactId ?? companyId}</div>
  ),
}));

vi.mock('@/components/ui/dialog', () => ({
  Dialog: ({
    open,
    onOpenChange,
    children,
  }: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    children: ReactNode;
  }) => open ? (
    <div data-testid="activity-dialog">
      {children}
      <button type="button" data-testid="dismiss-dialog" onClick={() => onOpenChange(false)}>
        Dismiss
      </button>
    </div>
  ) : null,
  DialogContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DialogDescription: ({ children }: { children: ReactNode }) => <p>{children}</p>,
  DialogFooter: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: ReactNode }) => <h2>{children}</h2>,
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

function clickButton(label: string) {
  const button = [...container.querySelectorAll('button')]
    .find((candidate) => candidate.textContent?.trim() === label);
  expect(button).toBeDefined();
  act(() => button?.click());
}

function setControlValue(control: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const prototype = control instanceof HTMLInputElement ? HTMLInputElement.prototype : HTMLTextAreaElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(prototype, 'value')?.set;
  act(() => {
    setter?.call(control, value);
    control.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

beforeEach(() => {
  createActivity.mockResolvedValue({ id: 'activity-1' });
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => {
    root.render(<ActivityTimeline activities={[]} workspaceId="workspace-1" companyId="company-1" />);
  });
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.clearAllMocks();
});

describe('CRM ActivityTimeline activity dialog', () => {
  it('offers the shared email composer when the record has an email address', () => {
    act(() => {
      root.render(
        <ActivityTimeline
          timelineItems={[]}
          workspaceId="workspace-1"
          contactId="contact-1"
          emailRecipient="buyer@example.com"
          presentation="borderless"
        />,
      );
    });

    clickButton('Email');
    expect(container.querySelector('[data-testid="email-composer"]')?.textContent).toBe('buyer@example.com');
  });

  it('matches the contact-detail tab selector for borderless activity filters', () => {
    act(() => {
      root.render(
        <ActivityTimeline
          activities={[]}
          workspaceId="workspace-1"
          companyId="company-1"
          presentation="borderless"
        />,
      );
    });

    const tabs = container.querySelectorAll('[role="tab"]');
    expect(tabs).toHaveLength(5);
    expect(tabs[0]?.className).toContain('text-[13px]');
    expect(tabs[0]?.className).toContain('border-primary');
    expect(tabs[0]?.getAttribute('aria-selected')).toBe('true');
  });

  it('shows the canonical CRM timeline filters and a paginated system event', () => {
    act(() => {
      root.render(
        <ActivityTimeline
          timelineItems={[{
            id: 'task:event-1',
            kind: 'task',
            event_type: 'task.completed',
            source_type: 'pm_activity',
            source_id: 'event-1',
            title: 'Prepare renewal plan',
            description: 'moved this task to Done',
            occurred_at: new Date().toISOString(),
            actor: { type: 'actor', id: 'user-1', name: 'Amad' },
            entity: { type: 'task', id: 'task-1', name: 'Prepare renewal plan', display_id: 'HLP-42' },
            can_edit: false,
            can_delete: false,
          }]}
          timelineFilter="all"
          workspaceId="workspace-1"
          companyId="company-1"
          presentation="borderless"
          hasNextPage
        />,
      );
    });

    expect([...container.querySelectorAll('[role="tab"]')].map((tab) => tab.textContent?.trim()))
      .toEqual(['All', 'Notes', 'Emails', 'Calls', 'Meetings', 'Tasks', 'Deals', 'Support']);
    expect(container.textContent).toContain('Prepare renewal plan');
    expect(container.textContent).toContain('HLP-42');
    expect(container.textContent).toContain('Task HLP-42 · Prepare renewal plan moved to Done');
    expect(container.textContent).toContain('by Amad');
    expect(container.textContent).toContain('Load more');
  });

  it('uses the rich email timeline for the Emails filter on contacts and companies', () => {
    act(() => {
      root.render(
        <ActivityTimeline
          timelineItems={[]}
          timelineFilter="email"
          workspaceId="workspace-1"
          companyId="company-1"
          presentation="borderless"
        />,
      );
    });

    expect(container.querySelector('[data-testid="rich-email-timeline"]')?.textContent).toBe('company-1');
    expect(container.textContent).not.toContain('No activities yet');
  });

  it('renders authored activity content with its entity icon and existing actions', () => {
    act(() => {
      root.render(
        <ActivityTimeline
          timelineItems={[{
            id: 'activity:note-1',
            kind: 'note',
            event_type: 'activity.note',
            source_type: 'crm_activity',
            source_id: 'note-1',
            title: 'Renewal discussion',
            description: 'Send the revised pricing on Monday.',
            occurred_at: new Date().toISOString(),
            actor: { type: 'actor', id: 'member-1', name: 'Waqar' },
            can_edit: true,
            can_delete: true,
          }]}
          timelineFilter="all"
          workspaceId="workspace-1"
          companyId="company-1"
          presentation="borderless"
        />,
      );
    });

    expect(container.textContent).toContain('Note added');
    expect(container.textContent).toContain('by Waqar');
    expect(container.textContent).toContain('Renewal discussion');
    expect(container.textContent).toContain('Send the revised pricing on Monday.');
    expect(container.querySelector('[aria-label="Edit activity"]')).not.toBeNull();
    expect(container.querySelector('[aria-label="Delete activity"]')).not.toBeNull();
  });

  it('expands and collapses overflowing activity content inline', () => {
    const scrollHeight = vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(160);
    const clientHeight = vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(80);

    act(() => {
      root.render(
        <ActivityTimeline
          timelineItems={[{
            id: 'activity:note-long',
            kind: 'note',
            event_type: 'activity.note',
            source_type: 'crm_activity',
            source_id: 'note-long',
            title: 'Detailed account plan',
            description: 'A long note with several decisions, risks, and next steps.',
            occurred_at: new Date().toISOString(),
            actor: { type: 'actor', id: 'member-1', name: 'Waqar' },
            can_edit: true,
            can_delete: true,
          }]}
          timelineFilter="all"
          workspaceId="workspace-1"
          companyId="company-1"
        />,
      );
    });

    const toggle = [...container.querySelectorAll('button')]
      .find((button) => button.textContent?.trim() === 'Show more');
    expect(toggle?.getAttribute('aria-expanded')).toBe('false');

    act(() => toggle?.click());
    expect(toggle?.textContent?.trim()).toBe('Show less');
    expect(toggle?.getAttribute('aria-expanded')).toBe('true');

    scrollHeight.mockRestore();
    clientHeight.mockRestore();
  });

  it('starts email descriptions at two lines and retains the show-more control', () => {
    const scrollHeight = vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(100);
    const clientHeight = vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(40);

    act(() => {
      root.render(
        <ActivityTimeline
          timelineItems={[{
            id: 'email:message-1',
            kind: 'email',
            event_type: 'email.inbound',
            source_type: 'crm_email_message',
            source_id: 'message-1',
            title: 'Renewal details',
            description: 'A longer email preview that initially occupies no more than two lines in the activity feed.',
            occurred_at: new Date().toISOString(),
            entity: { type: 'email_thread', id: 'thread-1', name: 'Renewal details' },
            can_edit: false,
            can_delete: false,
          }]}
          timelineFilter="all"
          workspaceId="workspace-1"
          contactId="contact-1"
          presentation="borderless"
        />,
      );
    });

    const description = [...container.querySelectorAll('p')]
      .find((paragraph) => paragraph.textContent?.startsWith('A longer email preview'));
    expect(description?.parentElement?.className).toContain('max-h-10');

    const toggle = [...container.querySelectorAll('button')]
      .find((button) => button.textContent?.trim() === 'Show more');
    expect(toggle).toBeDefined();
    act(() => toggle?.click());
    expect(description?.parentElement?.className).not.toContain('max-h-10');
    expect(toggle?.textContent?.trim()).toBe('Show less');

    scrollHeight.mockRestore();
    clientHeight.mockRestore();
  });

  it('retains the draft when dismissed or switched and clears it only on request', () => {
    clickButton('Note');

    const subject = container.querySelector<HTMLInputElement>('input[placeholder="What happened?"]');
    const details = container.querySelector<HTMLTextAreaElement>('textarea[placeholder^="Add context"]');
    expect(subject).not.toBeNull();
    expect(details).not.toBeNull();

    setControlValue(subject!, 'Discussed renewal');
    setControlValue(details!, 'Send pricing on Monday');
    clickButton('Call');

    expect(subject?.value).toBe('Discussed renewal');
    expect(details?.value).toBe('Send pricing on Monday');
    expect([...container.querySelectorAll('[role="radio"]')]
      .find((control) => control.textContent?.trim() === 'Call')
      ?.getAttribute('aria-checked')).toBe('true');

    const dismiss = container.querySelector<HTMLButtonElement>('[data-testid="dismiss-dialog"]');
    act(() => dismiss?.click());
    expect(container.querySelector('[data-testid="activity-dialog"]')).toBeNull();

    clickButton('Call');
    expect(container.querySelector<HTMLInputElement>('input[placeholder="What happened?"]')?.value)
      .toBe('Discussed renewal');
    expect(container.querySelector<HTMLTextAreaElement>('textarea[placeholder^="Add context"]')?.value)
      .toBe('Send pricing on Monday');

    clickButton('Clear draft');
    expect(container.querySelector<HTMLInputElement>('input[placeholder="What happened?"]')?.value).toBe('');
    expect(container.querySelector<HTMLTextAreaElement>('textarea[placeholder^="Add context"]')?.value).toBe('');
  });

  it('submits the selected type and resets the draft after a successful save', async () => {
    clickButton('Meeting');
    const subject = container.querySelector<HTMLInputElement>('input[placeholder="What happened?"]');
    setControlValue(subject!, 'Quarterly planning');

    const save = [...container.querySelectorAll('button')]
      .find((button) => button.textContent?.trim() === 'Save meeting');
    await act(async () => save?.click());

    expect(createActivity).toHaveBeenCalledWith(expect.objectContaining({
      activity_type: 'meeting',
      subject: 'Quarterly planning',
      company_id: 'company-1',
    }));
    expect(container.querySelector('[data-testid="activity-dialog"]')).toBeNull();

    clickButton('Meeting');
    expect(container.querySelector<HTMLInputElement>('input[placeholder="What happened?"]')?.value).toBe('');
  });
});

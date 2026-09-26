// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import { queryKeys } from '@/lib/queryKeys';
import { SetupEmailStep } from '../SetupEmailStep';
import { button, capability, click, renderWithQuery, type Rendered } from './setupTestUtils';

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }));
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children }: { to: string; children: React.ReactNode }) => <a href={to}>{children}</a>,
}));

let rendered: Rendered | undefined;
afterEach(async () => {
  await rendered?.unmount();
  rendered = undefined;
  vi.mocked(api.post).mockReset();
});

const testable = capability({ key: 'email_outbound', status: 'unable_to_verify', action: { kind: 'send_test_email', label: 'Send a test email' } });

async function renderStep(canManage = true, cap = testable) {
  rendered = await renderWithQuery(<SetupEmailStep capability={cap} workspaceId="ws-1" slug="acme" canManage={canManage} />);
  return rendered;
}

function status(container: HTMLElement) {
  return container.querySelector('[role="status"]')?.textContent ?? '';
}

describe('SetupEmailStep', () => {
  it('sends a test email, announces the recipient and refreshes capabilities', async () => {
    vi.mocked(api.post).mockResolvedValue({ data: { ok: true, recipient: 'owner@example.com' }, error: null, status: 200 } as never);
    const { container, client } = await renderStep();
    const invalidate = vi.spyOn(client, 'invalidateQueries');

    await click(button(container, 'Send test email'));

    expect(api.post).toHaveBeenCalledWith('/workspaces/ws-1/email/test');
    expect(status(container)).toBe('Test email sent to owner@example.com. Check that it arrived.');
    expect(container.querySelector('[role="status"]')?.getAttribute('aria-live')).toBe('polite');
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.workspaces.capabilities('ws-1') });
  });

  it('explains rate limiting instead of failing silently', async () => {
    vi.mocked(api.post).mockResolvedValue({ data: null, error: 'too many test emails; wait a minute and try again', status: 429 } as never);
    const { container } = await renderStep();

    await click(button(container, 'Send test email'));

    expect(status(container)).toBe('Too many test emails; wait a minute and try again.');
  });

  it('reports a delivery failure from the mail server', async () => {
    vi.mocked(api.post).mockResolvedValue({ data: { ok: false, error: 'SMTP authentication failed' }, error: null, status: 200 } as never);
    const { container } = await renderStep();

    await click(button(container, 'Send test email'));

    expect(status(container)).toBe('The test email failed: SMTP authentication failed');
  });

  it('shows server configuration instead of a button when mail is not configured', async () => {
    const { container } = await renderStep(true, capability({ key: 'email_outbound', action: { kind: 'server_config', label: 'Set SMTP_HOST and SMTP_FROM on the server' } }));

    expect(button(container, 'Send test email')).toBeUndefined();
    expect(container.querySelector('code')?.textContent).toBe('Set SMTP_HOST and SMTP_FROM on the server');
  });

  it('hides the test for members without workspace settings access', async () => {
    const { container } = await renderStep(false);

    expect(button(container, 'Send test email')).toBeUndefined();
    expect(container.textContent).toContain('A workspace admin can finish this step.');
  });
});

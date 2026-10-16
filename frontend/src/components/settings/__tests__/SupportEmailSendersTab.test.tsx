// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { SupportEmailSendersTab } from '../SupportEmailSendersTab';
import { SupportEmailSenderDialog } from '../SupportEmailSenderDialog';
import type { SupportEmailSender, SupportEmailSenderDomain } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const api = vi.hoisted(() => ({ create: vi.fn(), createDomain: vi.fn(), update: vi.fn(), verify: vi.fn(), verifySender: vi.fn() }));
const data = vi.hoisted(() => ({ senders: [] as SupportEmailSender[], domains: [] as SupportEmailSenderDomain[] }));
vi.mock('@/hooks/queries/useSupport', () => ({
  useSupportEmailSenders: () => ({ data: data.senders, isLoading: false }),
  useSupportEmailSenderDomains: () => ({ data: data.domains, isLoading: false }),
  useSupportMailboxes: () => ({ data: [{ id: 'billing', name: 'Billing', active: true }, { id: 'sales', name: 'Sales', active: true }] }),
  useCreateSupportEmailSender: () => ({ mutateAsync: api.create }),
  useCreateSupportEmailSenderDomain: () => ({ mutateAsync: api.createDomain }),
  useUpdateSupportEmailSender: () => ({ mutateAsync: api.update }),
  useVerifySupportEmailSenderDomain: () => ({ mutateAsync: api.verify }),
  useVerifySupportEmailSender: () => ({ mutateAsync: api.verifySender }),
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function sender(overrides: Partial<SupportEmailSender> = {}): SupportEmailSender {
  return { id: 'sender', workspace_id: 'workspace', email: 'support@example.com', domain: 'example.com', display_name: 'Support', local_part: 'support', active: false, default_scope: 'none', dkim_verified: true, return_path_domain_verified: true, ...overrides } as SupportEmailSender;
}
function domain(overrides: Partial<SupportEmailSenderDomain> = {}): SupportEmailSenderDomain {
  return { id: 'domain', domain: 'example.com', dkim_verified: true, return_path_domain_verified: true, dkim_host: 'key._domainkey', dkim_text_value: 'public-key', return_path_domain: 'pm-bounces', return_path_domain_cname_value: 'pm.example.net', ...overrides } as SupportEmailSenderDomain;
}
let root: Root;
let container: HTMLDivElement;
function render(canManage = true) {
  act(() => root.render(<TooltipProvider><SupportEmailSendersTab workspaceId="workspace" workspaceName="Example" workspaceWebsiteUrl="https://example.com" canManage={canManage} /></TooltipProvider>));
}
function button(text: string) {
  const found = Array.from(document.querySelectorAll('button')).find(b => b.textContent?.trim() === text);
  expect(found, `button: ${text}`).toBeDefined();
  return found!;
}
async function click(text: string) { await act(async () => button(text).click()); }
async function submit() { await act(async () => document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))); }
async function choose(id: string, text: string) {
  await act(async () => document.getElementById(id)!.click());
  await act(async () => {
    const option = Array.from(document.querySelectorAll<HTMLElement>('[role="option"]')).find(row => row.textContent?.trim() === text);
    expect(option, `option: ${text}`).toBeDefined(); option!.click();
  });
}
function pendingSetup() {
  data.domains = [domain({ dkim_verified: false, return_path_domain_verified: false })];
  api.create.mockImplementation(async payload => sender({ email: payload.email, display_name: payload.display_name, dkim_verified: false, return_path_domain_verified: false }));
}
function fill(id: string, value: string) {
  act(() => { const input = document.getElementById(id)!; Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value); input.dispatchEvent(new Event('input', { bubbles: true })); });
}
beforeEach(() => {
  vi.clearAllMocks();
  data.senders = []; data.domains = [domain()];
  api.create.mockReset().mockImplementation(async payload => sender({ email: payload.email, display_name: payload.display_name }));
  api.createDomain.mockReset().mockResolvedValue(domain());
  api.update.mockReset().mockImplementation(async ({ payload }) => sender({ ...payload, active: payload.default_scope !== 'none' }));
  api.verify.mockReset().mockResolvedValue(domain());
  api.verifySender.mockReset().mockResolvedValue(sender());
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  HTMLElement.prototype.scrollIntoView = vi.fn();
  container = document.createElement('div'); document.body.appendChild(container); root = createRoot(container);
});
afterEach(() => { act(() => root.unmount()); container.remove(); vi.unstubAllGlobals(); });

describe('sender address settings', () => {
  it('shows existing addresses directly, preserving assigned and unused senders without writing', () => {
    data.senders = [sender({ active: true, default_scope: 'workspace' }), sender({ id: 'unused', email: 'unused@example.com' })];
    render();
    expect(container.querySelectorAll('tbody tr')).toHaveLength(2);
    expect(container.textContent).toContain('Used for inboxes');
    expect(container.textContent).toContain('Workspace default');
    expect(container.textContent).toContain('Not in use');
    expect(api.update).not.toHaveBeenCalled();
  });
  it('adds an address on a verified domain and applies usage without recreating or rechecking the domain', async () => {
    render(); await click('Add address'); fill('sender-email', 'hello@example.com'); await submit();
    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({ email: 'hello@example.com' }));
    expect(api.createDomain).not.toHaveBeenCalled(); expect(api.verify).not.toHaveBeenCalled();
    expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ payload: expect.objectContaining({ default_scope: 'workspace' }) }));
    expect(document.querySelector('[role="dialog"]')).toBeNull();
  });
  it('does not offer editing or creation without support administration permission', () => {
    data.senders = [sender()]; render(false);
    expect(Array.from(container.querySelectorAll('button')).some(b => b.textContent === 'Add address')).toBe(false);
    expect(container.querySelector('[aria-label^="More options"]')).toBeNull();
  });
  it('keeps DNS pending and provider failures actionable without activating the sender', async () => {
    pendingSetup(); render(); await click('Add address'); await submit();
    api.verify.mockResolvedValueOnce(domain({ return_path_domain_verified: false }));
    await click('Check verification');
    expect(document.body.textContent).toContain('Not verified yet');
    expect(api.update).not.toHaveBeenCalled();
    api.verify.mockResolvedValueOnce(domain({ last_error: 'Provider unavailable' }));
    await click('Check again');
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('Provider unavailable');
    expect(api.update).not.toHaveBeenCalled();
    await click('Check again');
    expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ payload: expect.objectContaining({ default_scope: 'workspace' }) }));
    expect(button('Done')).toBeDefined();
  });
  it('shows a real pending state, blocks repeat checks, and never applies usage after closing', async () => {
    pendingSetup(); render(); await click('Add address'); await submit();
    let resolve!: (value: SupportEmailSenderDomain) => void;
    api.verify.mockImplementation(() => new Promise<SupportEmailSenderDomain>(done => { resolve = done; }));
    await click('Check verification');
    expect(document.body.textContent).toContain('Checking DNS records…');
    expect(button('Checking…').disabled).toBe(true);
    await click('Finish later');
    await act(async () => resolve(domain()));
    expect(api.update).not.toHaveBeenCalled();
    expect(document.querySelector('[role="dialog"]')).toBeNull();
  });
  it('retries just the inbox save when DNS verification succeeded', async () => {
    pendingSetup(); render(); await click('Add address'); await submit();
    api.update.mockRejectedValueOnce(new Error('Save unavailable'));
    await click('Check verification');
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('Address verified, but inbox selection could not be saved');
    expect(document.body.textContent).not.toContain('Done');
    await click('Retry save');
    expect(api.verify).toHaveBeenCalledTimes(1); expect(api.create).toHaveBeenCalledTimes(1);
    expect(button('Done')).toBeDefined();
  });
  it('reuses a domain saved before address creation failed', async () => {
    data.domains = []; api.create.mockRejectedValueOnce(new Error('Try again'));
    render(); await click('Add address'); await submit();
    expect(document.querySelector('[role="alert"]')?.textContent).toBe('Try again');
    await submit();
    expect(api.createDomain).toHaveBeenCalledTimes(1); expect(api.create).toHaveBeenCalledTimes(2);
  });
  it('does not recreate a saved address when its first assignment fails', async () => {
    api.update.mockRejectedValueOnce(new Error('Save unavailable'));
    render(); await click('Add address'); await submit();
    expect((document.getElementById('sender-email') as HTMLInputElement).disabled).toBe(true);
    await submit(); expect(api.create).toHaveBeenCalledTimes(1); expect(api.update).toHaveBeenCalledTimes(2);
  });
  it('keeps Set up only unused after verification', async () => {
    pendingSetup(); render(); await click('Add address'); await choose('sender-usage', 'Set up only'); await submit(); await click('Check verification');
    expect(api.update).not.toHaveBeenCalled(); expect(button('Done')).toBeDefined();
  });
  it.each([['Billing'], ['Billing', 'Sales']])('saves selected inboxes %j with their stable IDs', async (...names) => {
    render(); await click('Add address'); await choose('sender-usage', 'Selected inboxes'); await click('Select inboxes');
    for (const name of names) {
      await act(async () => Array.from(document.querySelectorAll<HTMLElement>('[role="option"]')).find(row => row.textContent?.trim() === name)!.click());
    }
    // Close the multiple-selection popover before submitting the form.
    await act(async () => document.querySelector('[aria-label="Select inboxes"]')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })));
    await submit();
    expect(api.update).toHaveBeenCalledWith({ senderId: 'sender', payload: { default_scope: 'mailbox', mailbox_id: 'billing', mailbox_ids: names.map(name => name.toLowerCase()) } });
  });
  it('preserves saved assignments when only the label changes, and allows disabling later', async () => {
    data.senders = [sender({ active: true, default_scope: 'mailbox', mailbox_ids: ['billing', 'sales'], mailbox_id: 'billing' })];
    render(); await click('support@example.com'); fill('sender-label', 'Customer care'); await submit();
    expect(api.update).toHaveBeenCalledWith({ senderId: 'sender', payload: { display_name: 'Customer care' } });
    await click('support@example.com'); await choose('sender-usage', 'Do not use'); await submit();
    expect(api.update).toHaveBeenLastCalledWith({ senderId: 'sender', payload: { default_scope: 'none', mailbox_id: null, mailbox_ids: [] } });
  });
  it('resumes an unfinished address without guessing usage from its related inbox', async () => {
    pendingSetup(); data.senders = [sender({ dkim_verified: false, mailbox_id: 'billing' })];
    render(); await click('Finish setup');
    expect(document.getElementById('sender-usage')?.textContent).toContain('Do not use');
    await submit(); await click('Finish later');
    expect(api.create).not.toHaveBeenCalled(); expect(api.update).not.toHaveBeenCalled();
    await click('Finish setup'); expect(document.querySelector('form')).not.toBeNull();
  });
  it('can recheck a sender with no separate domain row without changing its assignments', async () => {
    data.domains = []; const existing = sender({ active: true, default_scope: 'workspace' });
    act(() => root.render(<TooltipProvider><SupportEmailSenderDialog workspaceId="workspace" sender={existing} manageDNS domains={[]} senders={[existing]} mailboxes={[]} onClose={vi.fn()} /></TooltipProvider>));
    await click('Recheck DNS');
    expect(api.verifySender).toHaveBeenCalledWith('sender'); expect(api.verify).not.toHaveBeenCalled();
    expect(api.update).not.toHaveBeenCalled();
    expect(document.body.textContent).toContain('DNS verification refreshed');
  });
  it('returns a failed request to the records with a retry action', async () => {
    pendingSetup(); api.verify.mockRejectedValueOnce(new Error('Request timed out'));
    render(); await click('Add address'); await submit(); await click('Check verification');
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('Request timed out');
    expect(button('Check again').disabled).toBe(false); expect(api.update).not.toHaveBeenCalled();
  });
  it('validates the address and selected inboxes before creating any records', async () => {
    render(); await click('Add address'); fill('sender-email', 'invalid'); await submit();
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('valid email');
    fill('sender-email', 'hello@example.com'); await choose('sender-usage', 'Selected inboxes'); await submit();
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('Select one or more available inboxes');
    expect(api.create).not.toHaveBeenCalled(); expect(api.createDomain).not.toHaveBeenCalled();
  });
});

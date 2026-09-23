// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import type { AppEmailSettings } from '@/lib/instanceTypes';

const service = vi.hoisted(() => ({
  emailSettings: vi.fn(),
  updateEmailSettings: vi.fn(),
  clearEmailSettings: vi.fn(),
  sendTestEmail: vi.fn(),
}));
vi.mock('@/lib/services/instanceService', () => ({ instanceService: service }));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { AppEmailSettingsCard } from '../AppEmailSettingsCard';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

const saved: AppEmailSettings = {
  source: 'database', provider: 'smtp', editable: true, host: 'smtp.example.com', port: 587,
  username: 'mailer', from: 'helpin@example.com', tls_mode: 'starttls', password_set: true,
};

beforeEach(() => {
  service.emailSettings.mockResolvedValue({ data: saved, error: null, status: 200 });
  service.updateEmailSettings.mockImplementation(async (data) => ({ data: { ...saved, host: data.host }, error: null, status: 200 }));
  service.sendTestEmail.mockResolvedValue({ data: { ok: true, recipient: 'admin@example.com' }, error: null, status: 200 });
});

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  container = null;
  root = null;
  vi.clearAllMocks();
});

async function render() {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  await act(async () => {
    root?.render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><TooltipProvider><AppEmailSettingsCard /></TooltipProvider></QueryClientProvider>);
  });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  return container;
}

function type(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!;
  act(() => {
    setter.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

async function submit(page: HTMLElement) {
  await act(async () => {
    page.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  });
}

describe('AppEmailSettingsCard', () => {
  it('never shows the saved password and keeps it when the field is left empty', async () => {
    const page = await render();
    const password = page.querySelector<HTMLInputElement>('#smtp-password')!;
    expect(password.value).toBe('');
    expect(password.placeholder).toContain('Saved');
    expect(page.querySelector<HTMLInputElement>('#smtp-host')!.value).toBe('smtp.example.com');

    type(page.querySelector<HTMLInputElement>('#smtp-host')!, 'mail.example.org');
    await submit(page);
    expect(service.updateEmailSettings).toHaveBeenCalledWith(expect.objectContaining({ host: 'mail.example.org', password: null, port: 587, tls_mode: 'starttls' }));

    // Saving re-renders the form from the saved settings; the password stays blank.
    const refreshed = page.querySelector<HTMLInputElement>('#smtp-password')!;
    expect(refreshed.value).toBe('');
    type(refreshed, 'new-secret');
    await submit(page);
    expect(service.updateEmailSettings).toHaveBeenLastCalledWith(expect.objectContaining({ password: 'new-secret' }));
  });

  it('sends a test email once email is configured', async () => {
    const page = await render();
    const button = Array.from(page.querySelectorAll('button')).find((item) => item.textContent?.includes('Send test email'))!;
    await act(async () => button.click());
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
    expect(service.sendTestEmail).toHaveBeenCalled();
    expect(page.textContent).toContain('Test email sent to admin@example.com');
  });

  it('shows settings from the server configuration read-only', async () => {
    service.emailSettings.mockResolvedValue({ data: { ...saved, source: 'env', editable: false }, error: null, status: 200 });
    const page = await render();
    expect(page.textContent).toContain('Set by the server’s configuration');
    expect(page.querySelector('fieldset')?.disabled).toBe(true);
    expect(Array.from(page.querySelectorAll('button')).some((item) => item.textContent === 'Save')).toBe(false);
  });

  it('offers setup when nothing is configured yet', async () => {
    service.emailSettings.mockResolvedValue({ data: { ...saved, source: 'none', host: '', username: '', from: '', password_set: false }, error: null, status: 200 });
    const page = await render();
    expect(page.textContent).toContain('invitations are shared as links');
    expect(page.querySelector<HTMLInputElement>('#smtp-password')!.placeholder).toBe('SMTP password');
    expect(Array.from(page.querySelectorAll('button')).some((item) => item.textContent?.includes('Send test email'))).toBe(false);
  });
});

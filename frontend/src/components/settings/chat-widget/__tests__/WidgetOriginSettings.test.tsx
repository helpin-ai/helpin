// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { parseWidgetOrigins, WidgetOriginSettings } from '../WidgetOriginSettings';
import type { SupportInstallationResponse } from '@/lib/pmTypes';

const mocks = vi.hoisted(() => ({ save: vi.fn() }));
vi.mock('@/hooks/queries/useSupport', () => ({ useUpdateChatSettings: () => ({ mutateAsync: mocks.save, isPending: false }) }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
afterEach(() => vi.clearAllMocks());

describe('widget origin settings', () => {
  it('normalizes exact origins and rejects paths, credentials and wildcards', () => {
    expect(parseWidgetOrigins('https://EXAMPLE.com:443\n\nhttps://example.com\nhttp://localhost:3000')).toEqual(['http://localhost:3000', 'https://example.com']);
    for (const input of ['*', 'https://*.example.com', 'https://example.com/path', 'https://example.com//', 'https://user@example.com', 'https://example.com?', 'https://example.com#', 'null']) {
      expect(() => parseWidgetOrigins(input)).toThrow();
    }
    expect(parseWidgetOrigins('')).toEqual([]);
    expect(parseWidgetOrigins(' https://example.com/\nhttp://localhost:3000/')).toEqual(['http://localhost:3000', 'https://example.com']);
  });
  it('preserves Tauri custom origins and keeps platform variants distinct', () => {
    expect(parseWidgetOrigins('tauri://LOCALHOST/\nTAURI://localhost\nhttp://tauri.localhost\nhttps://tauri.localhost')).toEqual([
      'http://tauri.localhost', 'https://tauri.localhost', 'tauri://localhost',
    ]);
    for (const input of ['tauri://', 'tauri://other', 'tauri://localhost.evil', 'tauri://localhost:1420', 'tauri://user@localhost', 'tauri://localhost/path', 'tauri://localhost//', 'tauri://localhost?', 'tauri://localhost#', 'tauri://*', 'app://localhost', 'file:///app', 'null']) {
      expect(() => parseWidgetOrigins(input)).toThrow();
    }
  });
  it('shows the origin-first setup state and saves an explicit allowlist', async () => {
    const installation = { allowed_origins: [], identity_verification_mode: 'report_only' } as unknown as SupportInstallationResponse;
    const container = document.createElement('div'); document.body.append(container);
    const root = createRoot(container);
    try {
      await act(async () => root.render(<WidgetOriginSettings workspaceId="ws" installation={installation} />));
      expect(container.querySelector('[role="status"]')?.textContent).toContain('Add your website or desktop app origin');
      const input = container.querySelector('textarea')!;
      await act(async () => {
        Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(input, 'https://site.example\ntauri://localhost');
        input.dispatchEvent(new Event('input', { bubbles: true }));
      });
      const save = Array.from(container.querySelectorAll('button')).find(button => button.textContent === 'Save origin settings')!;
      await act(async () => save.click());
      expect(mocks.save).toHaveBeenCalledWith({ allowed_origins: ['https://site.example', 'tauri://localhost'], identity_verification_mode: 'report_only' });
    } finally { await act(async () => root.unmount()); container.remove(); }
  });
  it('saves allow-all and restores restrictions when unchecked', async () => {
    const installation = { allowed_origins: ['https://site.example'], identity_verification_mode: 'enforced' } as unknown as SupportInstallationResponse;
    const container = document.createElement('div'); document.body.append(container);
    const root = createRoot(container);
    try {
      await act(async () => root.render(<WidgetOriginSettings workspaceId="ws" installation={installation} />));
      const checkbox = container.querySelector<HTMLButtonElement>('[role="checkbox"]')!;
      const textarea = container.querySelector('textarea')!;
      const save = () => Array.from(container.querySelectorAll('button')).find(button => button.textContent === 'Save origin settings')!;
      await act(async () => checkbox.click());
      expect(textarea.disabled).toBe(true);
      await act(async () => save().click());
      expect(mocks.save).toHaveBeenLastCalledWith({ allowed_origins: ['*', 'https://site.example'], identity_verification_mode: 'enforced' });
      const updated = { ...installation, allowed_origins: ['*', 'https://site.example'] };
      await act(async () => root.render(<WidgetOriginSettings workspaceId="ws" installation={updated} />));
      expect(checkbox.getAttribute('aria-checked')).toBe('true');
      expect(textarea.value).toBe('https://site.example');
      await act(async () => checkbox.click());
      expect(textarea.disabled).toBe(false);
      await act(async () => save().click());
      expect(mocks.save).toHaveBeenLastCalledWith({ allowed_origins: ['https://site.example'], identity_verification_mode: 'enforced' });
    } finally { await act(async () => root.unmount()); container.remove(); }
  });

});

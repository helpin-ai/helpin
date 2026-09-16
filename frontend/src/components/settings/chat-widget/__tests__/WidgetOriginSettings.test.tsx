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

describe('widget website settings', () => {
  it('normalizes exact origins and rejects paths, credentials and wildcards', () => {
    expect(parseWidgetOrigins('https://EXAMPLE.com:443\n\nhttps://example.com\nhttp://localhost:3000')).toEqual(['http://localhost:3000', 'https://example.com']);
    for (const input of ['*', 'https://*.example.com', 'https://example.com/path', 'https://example.com//', 'https://user@example.com', 'https://example.com?', 'https://example.com#', 'null']) {
      expect(() => parseWidgetOrigins(input)).toThrow();
    }
    expect(parseWidgetOrigins('')).toEqual([]);
    expect(parseWidgetOrigins(' https://example.com/\nhttp://localhost:3000/')).toEqual(['http://localhost:3000', 'https://example.com']);
  });
  it('shows the origin-first setup state and saves an explicit allowlist', async () => {
    const installation = { allowed_origins: [], identity_verification_mode: 'report_only' } as unknown as SupportInstallationResponse;
    const container = document.createElement('div'); document.body.append(container);
    const root = createRoot(container);
    try {
      await act(async () => root.render(<WidgetOriginSettings workspaceId="ws" installation={installation} />));
      expect(container.querySelector('[role="status"]')?.textContent).toContain('Add your website origin');
      const input = container.querySelector('textarea')!;
      await act(async () => {
        Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(input, 'https://site.example');
        input.dispatchEvent(new Event('input', { bubbles: true }));
      });
      const save = Array.from(container.querySelectorAll('button')).find(button => button.textContent === 'Save website settings')!;
      await act(async () => save.click());
      expect(mocks.save).toHaveBeenCalledWith({ allowed_origins: ['https://site.example'], identity_verification_mode: 'report_only' });
    } finally { await act(async () => root.unmount()); container.remove(); }
  });
});

// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { WidgetOriginSettings } from '../WidgetOriginSettings';
import { normalizeWidgetOrigin } from '../widgetOrigins';
import type { SupportInstallationResponse } from '@/lib/pmTypes';

const mocks = vi.hoisted(() => ({ save: vi.fn() }));
vi.mock('@/hooks/queries/useSupport', () => ({
  useUpdateChatSettings: () => ({ mutateAsync: mocks.save, isPending: false }),
  useRevealWidgetSigningSecret: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRotateWidgetSigningSecret: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.clearAllMocks();
});

async function render(installation: Partial<SupportInstallationResponse>) {
  await act(async () => root.render(
    <TooltipProvider>
      <WidgetOriginSettings workspaceId="ws" installation={{ identity_verification_mode: 'report_only', ...installation } as SupportInstallationResponse} />
    </TooltipProvider>,
  ));
}

const rows = () => Array.from(container.querySelectorAll<HTMLInputElement>('input[aria-label^="Website "]'));
const button = (label: string) => Array.from(container.querySelectorAll('button')).find(candidate =>
  candidate.textContent === label || candidate.getAttribute('aria-label') === label)!;

async function type(input: HTMLInputElement, value: string) {
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

describe('normalizeWidgetOrigin', () => {
  it('accepts what people type and stores exact origins or subdomain patterns', () => {
    for (const [input, want] of [
      ['example.com', 'https://example.com'],
      [' https://EXAMPLE.com:443/pricing?plan=pro#top ', 'https://example.com'],
      ['https://example.com//', 'https://example.com'],
      ['http://example.com:8080', 'http://example.com:8080'],
      ['localhost:3000', 'http://localhost:3000'],
      ['127.0.0.1:5173', 'http://127.0.0.1:5173'],
      ['[::1]:3000', 'http://[::1]:3000'],
      ['*.example.com', 'https://*.example.com'],
      ['https://*.Example.com/', 'https://*.example.com'],
      ['https://*.example.com:8443', 'https://*.example.com:8443'],
    ]) {
      expect(normalizeWidgetOrigin(input)).toBe(want);
    }
  });

  it('keeps the supported Tauri origins distinct', () => {
    expect(normalizeWidgetOrigin('tauri://LOCALHOST/')).toBe('tauri://localhost');
    expect(normalizeWidgetOrigin('http://tauri.localhost')).toBe('http://tauri.localhost');
    expect(normalizeWidgetOrigin('https://tauri.localhost')).toBe('https://tauri.localhost');
    for (const input of ['tauri://', 'tauri://other', 'tauri://localhost.evil', 'tauri://localhost:1420', 'tauri://user@localhost', 'tauri://localhost/path', 'tauri://*', 'app://localhost', 'file:///app']) {
      expect(() => normalizeWidgetOrigin(input)).toThrow();
    }
  });

  it('rejects allow-all, credentials, typos and unsafe patterns with a plain message', () => {
    expect(() => normalizeWidgetOrigin('*')).toThrow('Allow any website or app');
    expect(() => normalizeWidgetOrigin('http://*.example.com')).toThrow('must use https');
    for (const input of ['*.localhost', 'https://*.127.0.0.1', 'https://*.*.example.com', 'https://app.*.example.com', 'https://user@example.com', 'null', 'examplecom', 'https://']) {
      expect(() => normalizeWidgetOrigin(input)).toThrow();
    }
  });
});

describe('widget origin settings', () => {
  it('asks for a website while the widget is blocked and saves the typed rows', async () => {
    await render({ allowed_origins: [] });
    expect(container.querySelector('[role="status"]')?.textContent).toContain('Add at least one website');
    expect(button('Save').disabled).toBe(true);
    await type(rows()[0], 'site.example/contact');
    await act(async () => button('Add website').click());
    await type(rows()[1], 'tauri://localhost');
    await act(async () => button('Save').click());
    expect(mocks.save).toHaveBeenCalledWith({ allowed_origins: ['https://site.example', 'tauri://localhost'], identity_verification_mode: 'report_only' });
  });

  it('fills in the workspace website suggestion and saves it in one click', async () => {
    await render({ allowed_origins: [], suggested_allowed_origins: ['https://site.example', 'https://*.site.example'] });
    expect(rows().map(row => row.value)).toEqual(['https://site.example', 'https://*.site.example']);
    expect(container.querySelector('[role="status"]')?.textContent).toContain('Filled in from your workspace website');
    await act(async () => button('Save').click());
    expect(mocks.save).toHaveBeenCalledWith({ allowed_origins: ['https://*.site.example', 'https://site.example'], identity_verification_mode: 'report_only' });
  });

  it('ignores the suggestion once origins are saved', async () => {
    await render({ allowed_origins: ['https://shop.example'], suggested_allowed_origins: ['https://site.example'] });
    expect(rows().map(row => row.value)).toEqual(['https://shop.example']);
    expect(container.querySelector('[role="status"]')).toBeNull();
    expect(button('Save').disabled).toBe(true);
  });

  it('shows the problem on the row instead of saving', async () => {
    await render({ allowed_origins: ['https://site.example'] });
    await act(async () => button('Add website').click());
    await type(rows()[1], 'http://*.site.example');
    await act(async () => button('Save').click());
    expect(mocks.save).not.toHaveBeenCalled();
    expect(rows()[1].getAttribute('aria-invalid')).toBe('true');
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('must use https');
    await act(async () => button('Remove website 2').click());
    expect(rows()).toHaveLength(1);
    expect(container.querySelector('[role="alert"]')).toBeNull();
    await act(async () => button('Save').click());
    expect(mocks.save).toHaveBeenCalledWith({ allowed_origins: ['https://site.example'], identity_verification_mode: 'report_only' });
  });

  it('saves allow-all and restores restrictions when unchecked', async () => {
    await render({ allowed_origins: ['https://site.example'], identity_verification_mode: 'enforced' });
    const checkbox = container.querySelector<HTMLButtonElement>('[role="checkbox"]')!;
    await act(async () => checkbox.click());
    expect(rows()[0].disabled).toBe(true);
    expect(container.textContent).toContain('Any site can load your widget');
    await act(async () => button('Save').click());
    expect(mocks.save).toHaveBeenLastCalledWith({ allowed_origins: ['*', 'https://site.example'], identity_verification_mode: 'enforced' });
    await render({ allowed_origins: ['*', 'https://site.example'], identity_verification_mode: 'enforced' });
    expect(checkbox.getAttribute('aria-checked')).toBe('true');
    expect(rows()[0].value).toBe('https://site.example');
    await act(async () => checkbox.click());
    expect(rows()[0].disabled).toBe(false);
    await act(async () => button('Save').click());
    expect(mocks.save).toHaveBeenLastCalledWith({ allowed_origins: ['https://site.example'], identity_verification_mode: 'enforced' });
  });
});

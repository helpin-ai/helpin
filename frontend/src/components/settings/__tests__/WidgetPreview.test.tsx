// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { WidgetPreview } from '../WidgetPreview';

const { mountWidget } = vi.hoisted(() => ({ mountWidget: vi.fn() }));
vi.mock('@helpin-ai/widget-core', () => ({ mountWidget, unmountWidget: vi.fn() }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('WidgetPreview configuration', () => {
  let root: Root;
  let container: HTMLDivElement;
  beforeEach(() => {
    mountWidget.mockClear();
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });
  afterEach(() => { act(() => root.unmount()); container.remove(); });

  it.each([true, false])('passes the effective AI-first setting (%s) and lets the widget resolve an empty greeting', (aiFirst) => {
    act(() => root.render(<WidgetPreview brandColor="#6366f1" showBranding launcherPosition="bottom_right" launcherIcon="chat_bubble" welcomeMessage="" aiFirst={aiFirst} />));
    const { config } = mountWidget.mock.lastCall![1];
    expect(config.features.aiFirst).toBe(aiFirst);
    expect(config.branding.welcomeMessage).toBe('');
  });

  it('updates the assistant identity and keeps the identity avatar separate from the logo', () => {
    const props = { brandColor: '#123456', showBranding: true, launcherPosition: 'bottom_right', launcherIcon: 'chat_bubble', welcomeMessage: '', aiFirst: true };
    act(() => root.render(<WidgetPreview {...props} aiDisplayName="Demo assistant" avatarUrl="https://example.com/avatar.png" logoUrl="https://example.com/logo.png" />));
    expect(mountWidget.mock.lastCall![1].config).toMatchObject({
      aiDisplayName: 'Demo assistant',
      branding: { avatarUrl: 'https://example.com/avatar.png', logoUrl: 'https://example.com/logo.png' },
    });
    act(() => root.render(<WidgetPreview {...props} aiDisplayName="Updated assistant" avatarUrl="https://example.com/updated.png" />));
    expect(mountWidget.mock.lastCall![1].config).toMatchObject({
      aiDisplayName: 'Updated assistant', branding: { avatarUrl: 'https://example.com/updated.png' },
    });
  });
});

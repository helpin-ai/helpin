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
});

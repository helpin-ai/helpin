// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { SupportAskAgentsButton } from '../SupportAskAgentsButton';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('SupportAskAgentsButton', () => {
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
  });

  it('opens the agent sidebar and exposes its expanded state', () => {
    const onOpen = vi.fn();
    act(() => {
      root.render(<TooltipProvider><SupportAskAgentsButton open={false} onOpen={onOpen} /></TooltipProvider>);
    });
    const button = container.querySelector<HTMLButtonElement>('button[aria-label="Ask agents about this conversation"]');
    expect(button?.getAttribute('aria-expanded')).toBe('false');
    act(() => button?.click());
    expect(onOpen).toHaveBeenCalledOnce();
  });
});

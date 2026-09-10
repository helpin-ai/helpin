// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it, vi } from 'vitest';
import { ForwardingSetupTransition } from '../ForwardingSetupTransition';
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const container = document.createElement('div');
const root = createRoot(container);
afterEach(() => vi.useRealTimers());
it('collapses verified instructions, removes them after the transition, and restores them for new setup', () => {
  vi.useFakeTimers();
  const render = (verified: boolean) => act(() => root.render(<ForwardingSetupTransition verified={verified}><button>Send test</button></ForwardingSetupTransition>));
  render(false);
  expect(container.querySelector('button')).not.toBeNull();
  render(true);
  expect(container.querySelector('[inert]')).not.toBeNull();
  expect(container.querySelector('[role="status"]')?.textContent).toContain('Forwarding verified');
  act(() => vi.advanceTimersByTime(350));
  expect(container.querySelector('button')).toBeNull();
  render(false);
  expect(container.querySelector('button')).not.toBeNull();
  act(() => root.unmount());
});

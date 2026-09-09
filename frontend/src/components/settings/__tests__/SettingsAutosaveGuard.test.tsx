// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SettingsAutosaveGuard } from '../SettingsAutosaveGuard';
const mocks = vi.hoisted(() => ({ proceed: vi.fn(), reset: vi.fn(), options: {} as any }));
vi.mock('@tanstack/react-router', () => ({ useBlocker: (options: any) => {
  mocks.options = options;
  return { status: 'blocked', proceed: mocks.proceed, reset: mocks.reset };
} }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const container = document.createElement('div');
let root: ReturnType<typeof createRoot>;
afterEach(() => { act(() => root.unmount()); vi.clearAllMocks(); });
function render(isDirty: boolean, error?: string, onRetry = vi.fn()) {
  act(() => root.render(<SettingsAutosaveGuard isDirty={isDirty} error={error} onRetry={onRetry} />));
}
describe('autosave navigation protection', () => {
  it('blocks navigation and tab closing while dirty, then continues after persistence', () => {
    root = createRoot(container);
    render(true);
    expect(mocks.options.shouldBlockFn()).toBe(true);
    expect(mocks.options.enableBeforeUnload).toBe(true);
    expect(mocks.proceed).not.toHaveBeenCalled();
    render(false);
    expect(mocks.proceed).toHaveBeenCalledOnce();
    expect(mocks.options.enableBeforeUnload).toBe(false);
  });
  it('keeps failed edits on the page and lets the user retry or explicitly discard', () => {
    root = createRoot(container);
    const retry = vi.fn();
    render(true, 'Network error', retry);
    expect(mocks.proceed).not.toHaveBeenCalled();
    const buttons = Array.from(document.querySelectorAll('button'));
    act(() => buttons.find(button => button.textContent === 'Retry')!.click());
    expect(retry).toHaveBeenCalledOnce();
    act(() => buttons.find(button => button.textContent === 'Leave without saving')!.click());
    expect(mocks.proceed).toHaveBeenCalledOnce();
  });
});

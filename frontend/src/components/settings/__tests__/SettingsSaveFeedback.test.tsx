// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { describe, it, expect, vi } from 'vitest';
import { SettingsSaveBar } from '../SettingsSaveBar';
import { SettingsSaveStatus } from '../SettingsSaveStatus';
import { Toaster } from '@/components/ui/sonner';

vi.mock('next-themes', () => ({ useTheme: () => ({ theme: 'dark' }) }));
vi.mock('sonner', () => ({ Toaster: ({ position, theme }: { position: string; theme: string }) => <div data-position={position} data-theme={theme} /> }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

function setup() {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  return { container, render: (element: React.ReactNode) => act(() => root.render(element)), cleanup: () => { act(() => root.unmount()); container.remove(); } };
}

describe('settings save feedback', () => {
  it('announces save state without announcing a successful save while idle', () => {
    const view = setup();
    view.render(<SettingsSaveStatus status="idle" />);
    expect(view.container.querySelector('[role="status"]')?.textContent).toBe('');
    view.render(<SettingsSaveStatus status="saving" />);
    expect(view.container.querySelector('[role="status"]')?.textContent).toBe('Saving…');
    view.render(<SettingsSaveStatus status="saved" />);
    expect(view.container.querySelector('[role="status"]')?.textContent).toBe('Saved');
    view.cleanup();
  });
  it('keeps the failure and retry available without submitting its parent form', () => {
    const retry = vi.fn();
    const submit = vi.fn((event: React.FormEvent) => event.preventDefault());
    const view = setup();
    view.render(<form onSubmit={submit}><SettingsSaveStatus status="error" error="Connection lost. Changes are not saved." onRetry={retry} /></form>);
    expect(view.container.querySelector('[role="alert"]')?.textContent).toContain('Changes are not saved');
    act(() => view.container.querySelector('button')!.click());
    expect(retry).toHaveBeenCalledOnce();
    expect(submit).not.toHaveBeenCalled();
    expect(view.container.querySelector('[role="alert"]')).not.toBeNull();
    view.cleanup();
  });
  it('removes hidden save actions from keyboard navigation and preserves form submission when visible', () => {
    const view = setup();
    const submit = vi.fn((event: React.FormEvent) => event.preventDefault());
    view.render(<form onSubmit={submit}><SettingsSaveBar visible={false}><button type="submit">Save</button></SettingsSaveBar></form>);
    expect(view.container.querySelector('button')).toBeNull();
    view.render(<form onSubmit={submit}><SettingsSaveBar><button type="submit">Save</button></SettingsSaveBar></form>);
    act(() => view.container.querySelector('button')!.click());
    expect(submit).toHaveBeenCalledOnce();
    view.cleanup();
  });
  it('defaults notifications to top-center while preserving explicit position and theme', () => {
    const view = setup();
    view.render(<Toaster />);
    expect(view.container.firstElementChild?.getAttribute('data-position')).toBe('top-center');
    expect(view.container.firstElementChild?.getAttribute('data-theme')).toBe('dark');
    view.render(<Toaster position="top-left" theme="light" />);
    expect(view.container.firstElementChild?.getAttribute('data-position')).toBe('top-left');
    expect(view.container.firstElementChild?.getAttribute('data-theme')).toBe('light');
    view.cleanup();
  });
});

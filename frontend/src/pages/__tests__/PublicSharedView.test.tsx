// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it, vi } from 'vitest';

import { workspacesService } from '@/lib/services/workspacesService';
import { dockChatService } from '@/lib/services/dockChatService';
import { useAuthStore } from '@/stores/authStore';
import { PublicSharedView, PublicSharedContent } from '../PublicSharedView';

globalThis.IS_REACT_ACT_ENVIRONMENT = true;
globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };

const container = document.createElement('div');
document.body.appendChild(container);
const root = createRoot(container);

afterEach(() => {
	act(() => root.render(<></>));
	useAuthStore.setState({ user: null });
	vi.restoreAllMocks();
});

it('renders a shared Ask conversation as a read-only transcript', async () => {
	await act(async () => {
		root.render(<PublicSharedContent resource={{
			resource_type: 'dock_chat',
			dock_chat: {
				title: 'Growth ideas',
				updated_at: '2026-08-18T00:00:00Z',
				messages: [
					{ id: 'm1', role: 'user', content: 'How can we grow?', created_at: '2026-08-18T00:00:00Z' },
					{ id: 'm2', role: 'assistant', content: '**Improve onboarding.**', created_at: '2026-08-18T00:00:01Z' },
				] as never,
			},
		}} />);
	});
	expect(container.textContent).toContain('Growth ideas');
	expect(container.textContent).toContain('How can we grow?');
	expect(container.textContent).toContain('Improve onboarding.');
	expect(container.querySelector('textarea')).toBeNull();
});


it('puts Open in Helpin after Sign in in the header, without repeating it by the title', async () => {
  useAuthStore.setState({ user: { id: 'viewer' } as never });
  vi.spyOn(workspacesService, 'getBySlug').mockResolvedValue({ data: { id: 'ws-1' } as never, error: null });
  vi.spyOn(dockChatService, 'getChat').mockResolvedValue({ data: { chat: { id: 'shared-chat-123' } } as never, error: null });
  const openPath = '/w/acme?ask_chat=shared-chat-123';
  vi.spyOn(dockChatService, 'getPublicSharedResource').mockResolvedValue({
    data: { resource_type: 'dock_chat', dock_chat: { title: 'Shared discussion', open_path: openPath, messages: [], updated_at: '2026-09-16T00:00:00Z' } },
    error: null,
  });
  await act(async () => { root.render(<PublicSharedView />); });
  const links = Array.from(container.querySelectorAll('header a'));
  expect(links.slice(-2).map(link => link.textContent)).toEqual(['Sign in', 'Open in Helpin']);
  expect(links.at(-1)?.getAttribute('href')).toBe(openPath);
  expect(links.at(-1)?.getAttribute('target')).toBe('_blank');
  expect(links.at(-1)?.getAttribute('rel')).toContain('noopener');
  expect(container.querySelector('main')?.textContent).not.toContain('Open in Helpin');
});


it.each([403, 404, 500])('disables internal navigation when the access check returns %s', async (status) => {
  useAuthStore.setState({ user: { id: 'viewer' } as never });
  vi.spyOn(workspacesService, 'getBySlug').mockResolvedValue({ data: { id: 'ws-1' } as never, error: null });
  vi.spyOn(dockChatService, 'getChat').mockResolvedValue({ data: null, error: 'failed', status });
  vi.spyOn(dockChatService, 'getPublicSharedResource').mockResolvedValue({
    data: { resource_type: 'dock_chat', dock_chat: { title: 'Shared discussion', open_path: '/w/acme?ask_chat=shared-chat-123', messages: [], updated_at: '' } }, error: null,
  });
  await act(async () => { root.render(<PublicSharedView />); });
  const button = container.querySelector<HTMLButtonElement>('button[aria-disabled="true"]');
  expect(button?.textContent).toBe('Open in Helpin');
  expect(container.querySelector('a[target="_blank"]')).toBeNull();
  await act(async () => { button?.focus(); });
  expect(document.body.querySelector('[role="tooltip"]')?.textContent).toContain(status === 500 ? 'Could not check access' : 'Ask the owner');
  expect(container.textContent).toContain('Shared discussion');
});

it('keeps Open in Helpin disabled for signed-out visitors without checking private resources', async () => {
  const check = vi.spyOn(workspacesService, 'getBySlug');
  vi.spyOn(dockChatService, 'getPublicSharedResource').mockResolvedValue({
    data: { resource_type: 'dock_chat', dock_chat: { title: 'Shared discussion', open_path: '/w/acme?ask_chat=shared-chat-123', messages: [], updated_at: '' } }, error: null,
  });
  await act(async () => { root.render(<PublicSharedView />); });
  const button = container.querySelector<HTMLButtonElement>('button[aria-disabled="true"]');
  expect(button).not.toBeNull();
  await act(async () => { button?.focus(); });
  expect(document.body.querySelector('[role="tooltip"]')?.textContent).toContain('Sign in');
  expect(check).not.toHaveBeenCalled();
});


it('keeps navigation disabled while checking and clears access when the viewer changes', async () => {
  useAuthStore.setState({ user: { id: 'viewer' } as never });
  let resolveWorkspace!: (value: never) => void;
  vi.spyOn(workspacesService, 'getBySlug').mockImplementation(() => new Promise(resolve => { resolveWorkspace = resolve; }));
  vi.spyOn(dockChatService, 'getChat').mockResolvedValue({ data: { chat: { id: 'chat-1' } } as never, error: null });
  vi.spyOn(dockChatService, 'getPublicSharedResource').mockResolvedValue({
    data: { resource_type: 'dock_chat', dock_chat: { title: 'Discussion', open_path: '/w/acme?ask_chat=chat-1', messages: [], updated_at: '' } }, error: null,
  });
  await act(async () => { root.render(<PublicSharedView />); });
  expect(container.querySelector('button[aria-disabled="true"]')).not.toBeNull();
  expect(container.querySelector('a[target="_blank"]')).toBeNull();
  await act(async () => { resolveWorkspace({ data: { id: 'ws-1' }, error: null } as never); });
  expect(container.querySelector('a[target="_blank"]')).not.toBeNull();
  await act(async () => { useAuthStore.setState({ user: { id: 'other-viewer' } as never }); });
  expect(container.querySelector('a[target="_blank"]')).toBeNull();
  expect(container.querySelector('button[aria-disabled="true"]')).not.toBeNull();
});

it('checks access to the exact shared agent run', async () => {
  useAuthStore.setState({ user: { id: 'viewer' } as never });
  vi.spyOn(workspacesService, 'getBySlug').mockResolvedValue({ data: { id: 'ws-1' } as never, error: null });
  const check = vi.spyOn(dockChatService, 'getRunSnapshot').mockResolvedValue({ data: { id: 'run-1' } as never, error: null });
  vi.spyOn(dockChatService, 'getPublicSharedResource').mockResolvedValue({
    data: { resource_type: 'agent_run', agent_run: { title: 'Run', open_path: '/w/acme/pm/coding-sessions/run-1', events: [], interactions: [], artifacts: [] } } as never, error: null,
  });
  await act(async () => { root.render(<PublicSharedView />); });
  expect(check).toHaveBeenCalledWith('ws-1', 'run-1', expect.any(AbortSignal));
  expect(container.querySelector('a[target="_blank"]')?.getAttribute('href')).toBe('/w/acme/pm/coding-sessions/run-1');
});

it('rechecks workspace access when returning to the tab', async () => {
  useAuthStore.setState({ user: { id: 'viewer' } as never });
  const workspace = vi.spyOn(workspacesService, 'getBySlug').mockResolvedValue({ data: null, error: 'forbidden', status: 403 });
  const chat = vi.spyOn(dockChatService, 'getChat').mockResolvedValue({ data: { chat: { id: 'chat-1' } } as never, error: null });
  vi.spyOn(dockChatService, 'getPublicSharedResource').mockResolvedValue({
    data: { resource_type: 'dock_chat', dock_chat: { title: 'Discussion', open_path: '/w/acme?ask_chat=chat-1', messages: [], updated_at: '' } }, error: null,
  });
  await act(async () => { root.render(<PublicSharedView />); });
  expect(container.querySelector('a[target="_blank"]')).toBeNull();
  expect(chat).not.toHaveBeenCalled();
  workspace.mockResolvedValue({ data: { id: 'ws-1' } as never, error: null });
  await act(async () => { window.dispatchEvent(new Event('focus')); });
  expect(container.querySelector('a[target="_blank"]')).not.toBeNull();
});

// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it, vi } from 'vitest';

import { dockChatService } from '@/lib/services/dockChatService';
import { useAuthStore } from '@/stores/authStore';
import { PublicSharedView, PublicSharedContent } from '../PublicSharedView';

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

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

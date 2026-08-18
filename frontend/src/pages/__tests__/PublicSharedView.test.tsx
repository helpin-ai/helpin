// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it } from 'vitest';

import { PublicSharedContent } from '../PublicSharedView';

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

const container = document.createElement('div');
document.body.appendChild(container);
const root = createRoot(container);

afterEach(() => {
	act(() => root.render(<></>));
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

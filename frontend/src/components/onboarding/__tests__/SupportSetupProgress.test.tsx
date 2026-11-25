// @vitest-environment jsdom

import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

const state = vi.hoisted(() => ({ status: vi.fn() }));

vi.mock('@tanstack/react-router', () => ({ Link: ({ children }: { children: ReactNode }) => <a>{children}</a> }));
vi.mock('@/lib/services/supportQuickstartService', () => ({ supportQuickstartService: { status: state.status } }));
vi.mock('@/hooks/queries', () => ({ useChatSettings: () => ({ data: undefined }) }));

import { SupportSetupProgress } from '../SupportSetupProgress';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

async function render(status: Record<string, unknown>) {
  state.status.mockResolvedValue({ data: { started: true, sources: [], agent_ready: false, done: false, ai_mode: 'answer', ...status }, error: null });
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  act(() => root?.render(<QueryClientProvider client={client}><SupportSetupProgress workspaceId="ws" slug="acme" /></QueryClientProvider>));
  for (let i = 0; i < 4; i++) {
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  }
}

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  container = null;
  root = null;
});

describe('SupportSetupProgress', () => {
  it('shows how far the copy is and the article it is on', async () => {
    await render({ articles: { status: 'importing', imported: 100, total: 400, failed: 0, current: 'How refunds work' } });

    expect(container?.textContent).toContain('Copying your help articles');
    expect(container?.textContent).toContain('100 of 400');
    expect(container?.textContent).toContain('How refunds work');
    expect(container?.querySelector('[data-slot="progress-indicator"]')?.getAttribute('style')).toContain('-75%');
  });

  it('does not claim a site was unreadable when nothing from it could be used', async () => {
    await render({
      done: true,
      articles: { status: 'done', imported: 400, total: 400, failed: 0 },
      sources: [{ name: 'Acme', url: 'https://acme.com', status: 'failed', indexed_pages: 0 }],
    });

    expect(container?.textContent).toContain('Couldn’t learn from acme.com');
    expect(container?.textContent).toContain('Your agent still answers from your help articles.');
    expect(container?.textContent).not.toContain('Couldn’t read');
    expect(container?.querySelector('[data-slot="progress"]')).toBeNull();
  });
});

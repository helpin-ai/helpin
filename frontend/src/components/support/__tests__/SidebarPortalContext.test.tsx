// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SupportConversation } from '@/lib/pmTypes';

const mocks = vi.hoisted(() => ({ context: vi.fn() }));

vi.mock('@/hooks/queries/useSupport', () => ({
  useConversationPortalContext: (_workspaceId: string, _conversationId: string, enabled: boolean) =>
    (enabled ? mocks.context() : { data: undefined }),
}));

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (select: (state: { currentWorkspace: { slug: string } }) => unknown) => select({ currentWorkspace: { slug: 'acme' } }),
}));

import { SidebarPortalContext } from '../SidebarPortalContext';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

let root: Root;
let container: HTMLDivElement;

beforeEach(() => {
  mocks.context.mockReset();
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

const portalRequest = { id: 'conv-1', channel: 'portal', source: 'portal', crm_company_id: 'acme' } as SupportConversation;
const render = (conversation: SupportConversation) =>
  act(async () => root.render(<SidebarPortalContext workspaceId="ws" conversation={conversation} />));

describe('SidebarPortalContext', () => {
  it('tells teammates which colleagues can read the conversation', async () => {
    mocks.context.mockReturnValue({ data: { company: { company_id: 'acme', company_name: 'Acme', members: 3 }, intake: null } });
    await render(portalRequest);
    expect(container.textContent).toContain('Besides the requester, 3 colleagues at Acme can read and reply');
  });

  it('uses the singular for one colleague', async () => {
    mocks.context.mockReturnValue({ data: { company: { company_id: 'acme', company_name: 'Acme', members: 1 }, intake: null } });
    await render(portalRequest);
    expect(container.textContent).toContain('1 colleague at Acme');
  });

  it('lists the articles the customer opened before writing in', async () => {
    mocks.context.mockReturnValue({ data: { company: null, intake: { suggested_count: 3, opened_articles: [{ id: 'a', title: 'Exporting reports', url: 'https://help.example/export' }] } } });
    await render(portalRequest);
    expect(container.textContent).toContain('Before writing in, the customer opened:');
    const link = container.querySelector<HTMLAnchorElement>('a[href="https://help.example/export"]');
    expect(link?.textContent).toBe('Exporting reports');
    expect(link?.target).toBe('_blank');
    expect(container.textContent).not.toContain('colleague');
  });

  it('opens an article for signed-in customers in Docs', async () => {
    mocks.context.mockReturnValue({ data: { company: null, intake: { suggested_count: 1, opened_articles: [{ id: 'doc-1', title: 'Renewal pricing', portal_article_id: 'hc_renewal' }] } } });
    await render(portalRequest);
    const link = container.querySelector<HTMLAnchorElement>('a');
    expect(link?.getAttribute('href')).toBe('/w/acme/docs/documents/doc-1');
    expect(link?.title).toBe('Renewal pricing (signed-in customers)');
  });

  it('stays hidden when there is nothing to tell', async () => {
    mocks.context.mockReturnValue({ data: { company: { company_id: 'acme', company_name: 'Acme', members: 0 }, intake: { suggested_count: 2, opened_articles: [] } } });
    await render(portalRequest);
    expect(container.textContent).toBe('');
  });

  it('does not ask the server about chats without a company', async () => {
    await render({ ...portalRequest, channel: 'widget', source: 'widget', crm_company_id: null });
    expect(mocks.context).not.toHaveBeenCalled();
    expect(container.textContent).toBe('');
  });
});

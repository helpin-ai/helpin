// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SupportConversation } from '@/lib/pmTypes';

const mocks = vi.hoisted(() => ({ audience: vi.fn() }));

vi.mock('@/hooks/queries/useSupport', () => ({
  useConversationPortalAudience: (_workspaceId: string, _conversationId: string, enabled: boolean) =>
    (enabled ? mocks.audience() : { data: undefined }),
}));

import { SidebarPortalAudience } from '../SidebarPortalAudience';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

let root: Root;
let container: HTMLDivElement;

beforeEach(() => {
  mocks.audience.mockReset();
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

const linked = { id: 'conv-1', crm_company_id: 'acme' } as SupportConversation;

describe('SidebarPortalAudience', () => {
  it('tells teammates which colleagues can read the conversation', async () => {
    mocks.audience.mockReturnValue({ data: { company_id: 'acme', company_name: 'Acme', members: 3 } });
    await act(async () => root.render(<SidebarPortalAudience workspaceId="ws" conversation={linked} />));
    expect(container.textContent).toContain('Besides the requester, 3 colleagues at Acme can read and reply');
  });

  it('uses the singular for one colleague', async () => {
    mocks.audience.mockReturnValue({ data: { company_id: 'acme', company_name: 'Acme', members: 1 } });
    await act(async () => root.render(<SidebarPortalAudience workspaceId="ws" conversation={linked} />));
    expect(container.textContent).toContain('1 colleague at Acme');
  });

  it('stays hidden when nobody else can read it', async () => {
    mocks.audience.mockReturnValue({ data: { company_id: 'acme', company_name: 'Acme', members: 0 } });
    await act(async () => root.render(<SidebarPortalAudience workspaceId="ws" conversation={linked} />));
    expect(container.textContent).toBe('');
    mocks.audience.mockReturnValue({ data: null });
    await act(async () => root.render(<SidebarPortalAudience workspaceId="ws" conversation={{ ...linked, id: 'conv-2' }} />));
    expect(container.textContent).toBe('');
  });

  it('does not ask the server about conversations without a company', async () => {
    await act(async () => root.render(<SidebarPortalAudience workspaceId="ws" conversation={{ ...linked, crm_company_id: null }} />));
    expect(mocks.audience).not.toHaveBeenCalled();
    expect(container.textContent).toBe('');
  });
});

// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SidebarAssociations } from '../SidebarAssociations';
import type { SupportSharedItem } from '@/lib/pmTypes';

const sharedItems: SupportSharedItem[] = [
  { kind: 'doc', id: 'doc', title: 'Export invoices', url: 'https://help.example.com/export-invoices-ab12cd34', message_id: 'm1', shared_at: '2026-10-25T10:00:00Z' },
  { kind: 'file', id: 'pdf', title: 'invoice.pdf', url: 'https://storage.example.com/invoice.pdf', file_type: 'application/pdf', file_size: 2048, message_id: 'm1', shared_at: '2026-10-25T10:00:00Z' },
  { kind: 'file', id: 'video', title: 'steps.mp4', url: 'https://storage.example.com/steps.mp4', file_type: 'video/mp4', file_size: 4096, message_id: 'm1', shared_at: '2026-10-25T10:00:00Z' },
  { kind: 'file', id: 'sheet', title: 'template.xlsx', url: 'https://storage.example.com/template.xlsx', file_type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', file_size: 1024, message_id: 'm2', shared_at: '2026-10-25T11:00:00Z' },
];

vi.mock('@/lib/api', () => ({ API_BASE: 'https://api.example.com/api', fetchWithSessionAuth: vi.fn() }));
vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn(), useLocation: () => ({ pathname: '/' }) }));
vi.mock('@/hooks/queries', () => ({
  useConversationAssociations: () => ({ isLoading: false, data: { tasks: [], crm_records: [] } }),
  useCreatePMAssociation: () => ({ mutateAsync: vi.fn() }),
  useWorkflows: () => ({ data: [] }),
  useWorkspaceAccess: () => ({ data: { modules: [] } }),
}));
vi.mock('@/hooks/queries/useSupport', () => ({ useConversationSharedItems: () => ({ data: sharedItems }) }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('sidebar docs and files', () => {
  let container: HTMLDivElement;
  let root: ReturnType<typeof createRoot>;

  beforeEach(async () => {
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
    await act(async () => root.render(<SidebarAssociations workspaceId="workspace" conversationId="conversation" />));
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
  });

  const lightbox = () => document.body.querySelector('[data-testid="support-attachment-lightbox"]');
  const row = (title: string) => Array.from(container.querySelectorAll('a, button')).find((element) => element.textContent?.includes(title))!;

  it('opens articles and other files in a new tab', () => {
    for (const title of ['Export invoices', 'template.xlsx']) {
      const link = row(title);
      expect(link.tagName).toBe('A');
      expect(link.getAttribute('target')).toBe('_blank');
    }
  });

  it('opens PDFs and videos in the lightbox and moves between them', async () => {
    await act(async () => (row('invoice.pdf') as HTMLButtonElement).click());
    expect(lightbox()?.querySelector('iframe')?.getAttribute('src')).toBe('https://storage.example.com/invoice.pdf');
    expect(lightbox()?.textContent).toContain('1 / 2');

    await act(async () => lightbox()!.querySelector<HTMLButtonElement>('button[aria-label="Next attachment"]')!.click());
    expect(lightbox()?.querySelector('video source')?.getAttribute('src')).toBe('https://storage.example.com/steps.mp4');
    expect(lightbox()?.querySelector('button[aria-label="Download attachment"]')).not.toBeNull();

    await act(async () => window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })));
    expect(lightbox()).toBeNull();
  });
});

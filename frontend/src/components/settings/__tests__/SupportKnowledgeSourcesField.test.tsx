// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { DocsSpace } from '@/lib/docsTypes';
import { SupportKnowledgeSourcesField } from '../SupportKnowledgeSourcesField';

const internalSpace: DocsSpace = {
  id: 'internal-space',
  workspace_id: 'workspace-1',
  team_ids: [],
  name: 'Sales guidance',
  slug: 'sales-guidance',
  visibility: 'workspace_wide',
  type: 'internal',
  is_system: false,
  position: 0,
  created_by: 'user-1',
  created_at: '2026-07-17T00:00:00Z',
  updated_at: '2026-07-17T00:00:00Z',
};

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('SupportKnowledgeSourcesField internal docs', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  it('offers internal spaces as private AI guidance', async () => {
    await act(async () => {
      root.render(
        <SupportKnowledgeSourcesField
          agentId="agent-1"
          spaces={[internalSpace]}
          knowledgeSources={[]}
          onToggle={vi.fn()}
          onReindex={vi.fn()}
          kind="internal"
        />,
      );
    });

    expect(container.textContent).toContain('Sales guidance');
    expect(container.textContent).toContain('Internal');
    expect(container.textContent).toContain('never shown as customer citations');
  });

  it('links an empty internal section back to Docs', async () => {
    await act(async () => {
      root.render(
        <SupportKnowledgeSourcesField
          agentId="agent-1"
          spaces={[]}
          knowledgeSources={[]}
          onToggle={vi.fn()}
          onReindex={vi.fn()}
          onOpenDocs={vi.fn()}
          kind="internal"
        />,
      );
    });

    expect(container.textContent).toContain('No internal docs spaces yet');
    expect(container.textContent).toContain('Create an Internal docs space');
    expect(container.textContent).toContain('Open docs');
  });
});

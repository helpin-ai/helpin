// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ToolRow } from '../ToolCatalog';
import type { ToolCatalogEntry } from '@/lib/pmTypes';

vi.mock('@/hooks/queries', () => ({
  useAutomationToolCatalog: vi.fn(),
}));
vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: vi.fn(),
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

afterEach(() => {
  act(() => root?.unmount());
  root = null;
  container?.remove();
  container = null;
});

function render(node: React.ReactNode) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(node));
}

const tool: ToolCatalogEntry = {
  name: 'repository_search',
  description: 'Search repository files and return matching paths.',
  category: 'Workspace',
  presets: ['code_builder'],
  input_schema: {
    type: 'object',
    properties: {
      query: { type: 'string', description: 'Text to search for.' },
    },
    required: ['query'],
  },
};

describe('ToolRow', () => {
  it('uses a divider row and expands its parameters inline', () => {
    render(<ToolRow tool={tool} />);

    const wrapper = container?.firstElementChild;
    const trigger = container?.querySelector('button');
    expect(wrapper?.className).toContain('border-b');
    expect(wrapper?.className).not.toContain('rounded');
    expect(wrapper?.className).not.toContain('bg-card');
    expect(trigger?.getAttribute('aria-expanded')).toBe('false');

    act(() => trigger?.dispatchEvent(new MouseEvent('click', { bubbles: true })));

    expect(trigger?.getAttribute('aria-expanded')).toBe('true');
    expect(container?.querySelector('[role="region"]')?.textContent).toContain('query');
    expect(container?.querySelector('[role="region"]')?.textContent).toContain('required');
  });

  it('keeps tools without parameters noninteractive', () => {
    render(
      <ToolRow
        tool={{
          ...tool,
          name: 'list_workspaces',
          input_schema: { type: 'object', properties: {} },
        }}
      />,
    );

    expect(container?.querySelector('button')).toBeNull();
    expect(container?.textContent).toContain('list_workspaces');
    expect(container?.querySelector('[role="region"]')).toBeNull();
  });
});

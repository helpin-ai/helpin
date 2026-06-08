// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';

import { ActivityTimeline } from '@/components/pm/ActivityTimeline';
import type { ActivityLogEntry } from '@/lib/pmTypes';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeAll(() => {
  globalThis.IS_REACT_ACT_ENVIRONMENT = true;
});

afterEach(() => {
  if (root) {
    act(() => root?.unmount());
  }
  root = null;
  container?.remove();
  container = null;
});

function renderTimeline(activity: ActivityLogEntry[]) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => {
    root?.render(<ActivityTimeline activity={activity} showAll onShowAll={() => {}} entityLabel="task" />);
  });
  return container;
}

function agentActivity(newValue: string, metadata: Record<string, unknown> = {}): ActivityLogEntry {
  return {
    activity: {
      id: `activity-${newValue}`,
      workspace_id: 'workspace-1',
      entity_type: 'task',
      entity_id: 'task-1',
      actor_id: 'user-1',
      action: 'updated',
      field_name: 'agent_run',
      new_value: newValue,
      metadata: { agent_name: 'Atlas', ...metadata },
      created_at: '2026-06-08T12:00:00.000Z',
    },
    actor: {
      id: 'user-1',
      email: 'amad@example.com',
      full_name: 'Amad Ali',
      created_at: '2026-06-01T00:00:00.000Z',
      updated_at: '2026-06-01T00:00:00.000Z',
    },
  };
}

describe('ActivityTimeline agent run activity', () => {
  it('renders readable user action labels and note snippets', () => {
    const node = renderTimeline([
      agentActivity('approved'),
      agentActivity('changes_requested'),
      agentActivity('note_added', { note_snippet: 'Please keep this focused.' }),
      agentActivity('cancelled'),
    ]);

    expect(node.textContent).toContain('approved agent run');
    expect(node.textContent).toContain('requested changes on agent run');
    expect(node.textContent).toContain('added note to agent run');
    expect(node.textContent).toContain('Please keep this focused.');
    expect(node.textContent).toContain('cancelled agent run');
  });
});

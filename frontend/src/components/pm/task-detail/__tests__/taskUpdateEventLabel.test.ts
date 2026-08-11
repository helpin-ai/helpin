import { describe, expect, it } from 'vitest';
import type { TaskUpdateEntry } from '@/lib/pmTypes';
import { taskUpdateEventLabel } from '../taskUpdateEventLabel';

function noteAddedEntry(overrides: Partial<TaskUpdateEntry> = {}): TaskUpdateEntry {
  return {
    id: 'activity:activity-1',
    kind: 'change',
    occurred_at: '2026-08-11T12:00:00Z',
    actor: {
      id: 'user-1',
      email: 'alex@example.com',
      full_name: 'Alex',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
    activity: {
      id: 'activity-1',
      workspace_id: 'workspace-1',
      entity_type: 'task',
      entity_id: 'task-1',
      action: 'updated',
      field_name: 'agent_run',
      new_value: 'note_added',
      metadata: {
        agent_name: 'Forge',
        note_snippet: 'Please keep the implementation focused.',
      },
      created_at: '2026-08-11T12:00:00Z',
    },
    ...overrides,
  };
}

describe('taskUpdateEventLabel', () => {
  it('renders an agent run note as a readable audit sentence', () => {
    expect(taskUpdateEventLabel(noteAddedEntry())).toBe(
      'Alex added a note to the Forge run · “Please keep the implementation focused.”',
    );
  });

  it('uses a neutral sentence when the note actor is unavailable', () => {
    expect(taskUpdateEventLabel(noteAddedEntry({ actor: undefined }))).toBe(
      'A note was added to the Forge run · “Please keep the implementation focused.”',
    );
  });
});

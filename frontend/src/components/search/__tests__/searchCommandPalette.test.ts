import { describe, expect, it } from 'vitest';

import { buildTaskCommandValue } from '@/components/search/searchCommandPalette';

describe('buildTaskCommandValue', () => {
  it('includes task key, numeric display id, and task name for cmdk matching', () => {
    expect(
      buildTaskCommandValue({
        id: 'task-123',
        task_key: 'HLP-123',
        display_id: 123,
        name: 'Fix command palette search',
      }),
    ).toBe('task task-123 HLP-123 123 Fix command palette search');
  });

  it('falls back cleanly when the task key is missing', () => {
    expect(
      buildTaskCommandValue({
        id: 'task-42',
        display_id: 42,
        name: 'Unkeyed task',
      }),
    ).toBe('task task-42 42 Unkeyed task');
  });
});

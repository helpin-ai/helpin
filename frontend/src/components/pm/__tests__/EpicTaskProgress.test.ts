import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('epic task progress', () => {
  it('places the shared inline progress in the Tasks heading instead of the metadata pane', () => {
    const source = readFileSync(resolve(__dirname, '../../../pages/pm/EpicDetail.tsx'), 'utf8');
    const tasksHeadingIndex = source.indexOf('title={`Tasks (${tasks.length})`}');
    const progressIndex = source.indexOf('<InlineCompletionProgress', tasksHeadingIndex);
    const taskListIndex = source.indexOf('<TaskListView', tasksHeadingIndex);

    expect(tasksHeadingIndex).toBeGreaterThan(-1);
    expect(progressIndex).toBeGreaterThan(tasksHeadingIndex);
    expect(taskListIndex).toBeGreaterThan(progressIndex);
    expect(source).toContain('testIdPrefix="epic-tasks"');
    expect(source).toContain('className="ml-auto pl-6">{renderTaskHeaderAddButton()}</div>');
    expect(source).not.toContain('<h3 className="text-xs font-semibold text-foreground">Progress</h3>');
  });
});

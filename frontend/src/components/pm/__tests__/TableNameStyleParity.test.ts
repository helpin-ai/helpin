import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { TABLE_NAME_TEXT } from '@/lib/tableStyles';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('table name style parity', () => {
  const epicSource = readFileSync(resolve(__dirname, '../../../pages/pm/Epics.tsx'), 'utf8');
  const taskSource = readFileSync(resolve(__dirname, '../TaskListView.tsx'), 'utf8');
  const objectivePickerSource = readFileSync(resolve(__dirname, '../ObjectivePicker.tsx'), 'utf8');

  it('uses the shared soft foreground name style in epic and task tables', () => {
    expect(TABLE_NAME_TEXT).toBe('text-sm text-foreground/90');
    expect(epicSource).toContain('${TABLE_NAME_TEXT}');
    expect(taskSource).toContain('${TABLE_NAME_TEXT}');
  });

  it('standardizes epic table cell components on the text-ui token', () => {
    const componentStart = epicSource.indexOf('const MemoEpicGroupRow');
    const componentEnd = epicSource.indexOf('// ── Main page', componentStart);
    const columnStart = epicSource.indexOf('  const columns = useMemo', componentEnd);
    const columnEnd = epicSource.indexOf('\n    ],\n    [', columnStart);
    const componentSource = epicSource.slice(componentStart, componentEnd);
    const columnSource = epicSource.slice(columnStart, columnEnd);

    expect(componentSource).toContain('text-ui');
    expect(componentSource).not.toContain('text-xs');
    expect(columnSource).toContain('text-ui');
    expect(columnSource).not.toContain('text-xs');
    expect(objectivePickerSource).toContain('text-ui');
    expect(objectivePickerSource).not.toContain('text-xs');
  });
});

import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('TaskDetailPanel description actions', () => {
  it('aligns the edit and done description actions on the left', () => {
    const source = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const doneIndex = source.indexOf('Done');
    const editIndex = source.indexOf('Edit description');
    const doneBlock = source.slice(source.lastIndexOf('<div', doneIndex), doneIndex);
    const editBlock = source.slice(source.lastIndexOf('<div', editIndex), editIndex);

    expect(doneIndex).toBeGreaterThan(-1);
    expect(editIndex).toBeGreaterThan(-1);
    expect(doneBlock).toContain('flex justify-start');
    expect(editBlock).toContain('flex justify-start');
    expect(editBlock).toContain('mt-3');
    expect(doneBlock).not.toContain('justify-end');
    expect(editBlock).not.toContain('justify-end');
  });
});

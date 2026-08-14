import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('TaskDetailPanel description actions', () => {
  it('keeps edit on the left and uses the shared pinned save actions', () => {
    const source = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const actionsSource = readFileSync(resolve(__dirname, '../../DetailDescriptionEditorActions.tsx'), 'utf8');
    const editIndex = source.indexOf('Edit description');
    const editBlock = source.slice(source.lastIndexOf('<div', editIndex), editIndex);

    expect(editIndex).toBeGreaterThan(-1);
    expect(editBlock).toContain('flex justify-start');
    expect(editBlock).toContain('mt-3');
    expect(editBlock).not.toContain('justify-end');
    expect(source).toContain('<DetailDescriptionEditorActions');
    expect(source).toContain('onCancel={cancelDescriptionEditing}');
    expect(actionsSource).toContain('sticky bottom-0');
    expect(actionsSource).toContain('justify-end');
    expect(actionsSource).toContain('Cancel');
    expect(actionsSource).toContain('Done');
  });
});

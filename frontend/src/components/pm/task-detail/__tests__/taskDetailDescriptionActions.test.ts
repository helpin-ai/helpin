import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('TaskDetailPanel description actions', () => {
  it('uses section-pinned edit and save actions', () => {
    const source = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const actionsSource = readFileSync(resolve(__dirname, '../../DetailDescriptionEditorActions.tsx'), 'utf8');
    const editButtonSource = readFileSync(resolve(__dirname, '../../DetailDescriptionEditButton.tsx'), 'utf8');

    expect(source).toContain('<DetailDescriptionEditButton');
    expect(editButtonSource).toContain('absolute inset-y-0 right-0');
    expect(editButtonSource).toContain('sticky top-3');
    expect(editButtonSource).toContain('h-9 w-9');
    expect(editButtonSource).toContain('md:group-hover/desc:opacity-100');
    expect(editButtonSource).toContain("label = 'Edit description'");
    expect(source).toContain('<DetailDescriptionEditorActions');
    expect(source).toContain('onCancel={cancelDescriptionEditing}');
    expect(actionsSource).toContain('sticky bottom-0');
    expect(actionsSource).toContain('justify-end');
    expect(actionsSource).toContain('Cancel');
    expect(actionsSource).toContain('Done');
  });
});

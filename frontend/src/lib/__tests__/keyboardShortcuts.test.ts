// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';

import { isEditableShortcutTarget } from '@/lib/keyboardShortcuts';

describe('keyboardShortcuts', () => {
  it('detects native editable fields', () => {
    expect(isEditableShortcutTarget(document.createElement('input'))).toBe(true);
    expect(isEditableShortcutTarget(document.createElement('textarea'))).toBe(true);
    expect(isEditableShortcutTarget(document.createElement('select'))).toBe(true);
  });

  it('detects nested rich text editor targets', () => {
    const editor = document.createElement('div');
    editor.setAttribute('contenteditable', 'true');
    const paragraph = document.createElement('p');
    const bold = document.createElement('strong');

    paragraph.appendChild(bold);
    editor.appendChild(paragraph);

    expect(isEditableShortcutTarget(bold)).toBe(true);
  });

  it('ignores non-editable targets', () => {
    expect(isEditableShortcutTarget(document.createElement('button'))).toBe(false);
    expect(isEditableShortcutTarget(null)).toBe(false);
  });
});

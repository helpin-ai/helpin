import { describe, expect, it } from 'vitest';

import type { Workspace } from '@/lib/types';
import {
  flattenGroupedWorkspaces,
  isEditableShortcutTarget,
  workspaceShortcutLabel,
} from '@/components/layout/workspaceSwitcherShortcuts';

function makeWorkspace(id: string, name: string): Workspace {
  const now = new Date().toISOString();
  return {
    id,
    organization_id: 'org-1',
    name,
    slug: name.toLowerCase(),
    workspace_key: name.slice(0, 3).toUpperCase(),
    owner_id: 'user-1',
    timezone: 'UTC',
    created_at: now,
    updated_at: now,
  };
}

describe('workspaceSwitcherShortcuts', () => {
  it('flattens grouped workspaces in render order', () => {
    const groups = [
      { workspaces: [makeWorkspace('1', 'Alpha'), makeWorkspace('2', 'Beta')] },
      { workspaces: [makeWorkspace('3', 'Gamma')] },
    ];

    expect(flattenGroupedWorkspaces(groups).map((workspace) => workspace.id)).toEqual(['1', '2', '3']);
  });

  it('formats shortcut labels for the first nine workspaces', () => {
    expect(workspaceShortcutLabel(0, true)).toBe('⌘1');
    expect(workspaceShortcutLabel(4, false)).toBe('Ctrl+5');
    expect(workspaceShortcutLabel(8, false)).toBe('Ctrl+9');
    expect(workspaceShortcutLabel(9, true)).toBeNull();
    expect(workspaceShortcutLabel(-1, false)).toBeNull();
  });

  it('detects editable shortcut targets', () => {
    class FakeHTMLElement extends EventTarget {
      tagName: string;
      isContentEditable: boolean;

      constructor(tagName: string, isContentEditable = false) {
        super();
        this.tagName = tagName;
        this.isContentEditable = isContentEditable;
      }
    }

    const originalHTMLElement = globalThis.HTMLElement;
    Object.defineProperty(globalThis, 'HTMLElement', {
      configurable: true,
      value: FakeHTMLElement,
    });

    try {
      const input = new FakeHTMLElement('INPUT');
      const textarea = new FakeHTMLElement('TEXTAREA');
      const editable = new FakeHTMLElement('DIV', true);
      const plain = new FakeHTMLElement('BUTTON');

      expect(isEditableShortcutTarget(input)).toBe(true);
      expect(isEditableShortcutTarget(textarea)).toBe(true);
      expect(isEditableShortcutTarget(editable)).toBe(true);
      expect(isEditableShortcutTarget(plain)).toBe(false);
      expect(isEditableShortcutTarget(null)).toBe(false);
    } finally {
      Object.defineProperty(globalThis, 'HTMLElement', {
        configurable: true,
        value: originalHTMLElement,
      });
    }
  });
});

import type { Workspace } from '@/lib/types';
export { isEditableShortcutTarget } from '@/lib/keyboardShortcuts';

export function isMacPlatform(): boolean {
  if (typeof navigator === 'undefined') return false;

  return /Mac|iPhone|iPad|iPod/.test(navigator.platform) || /Mac|iPhone|iPad|iPod/.test(navigator.userAgent);
}

export function workspaceShortcutLabel(index: number, isMac: boolean): string | null {
  if (index < 0 || index > 8) return null;

  return `${isMac ? '⌘' : 'Ctrl+'}${index + 1}`;
}

export function flattenGroupedWorkspaces(groups: { workspaces: Workspace[] }[]): Workspace[] {
  return groups.flatMap((group) => group.workspaces);
}

// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { EditSkillDialog } from '../SkillCatalog';
import type { SkillCatalogEntry } from '@/lib/pmTypes';

const mocks = vi.hoisted(() => ({ save: vi.fn(), toastError: vi.fn() }));
vi.mock('@/hooks/queries', () => ({
  useUpdateWorkspaceSkill: () => ({ mutateAsync: mocks.save, isPending: false }),
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: mocks.toastError } }));
vi.mock('@/components/pm/CodingSession/MarkdownContent', () => ({ MarkdownContent: () => null }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root;
let container: HTMLDivElement;
const defaults = { title: 'Support', description: 'Default description', instructions: 'Default instructions' };
const skill: SkillCatalogEntry = {
  key: 'support_triage_response', source_kind: 'built_in', title: 'Support',
  description: 'Current description', instructions: 'Current instructions',
  version_key: 'current-version', default_content: defaults, is_customized: true,
};

beforeEach(() => {
  mocks.save.mockReset().mockResolvedValue({});
  mocks.toastError.mockReset();
  container = document.createElement('div'); document.body.append(container);
  root = createRoot(container);
});
afterEach(() => { act(() => root.unmount()); container.remove(); });
function button(label: string) {
  const el = Array.from(document.querySelectorAll('button')).find(b => b.textContent === label);
  if (!el) throw new Error(`Missing ${label}`);
  return el;
}
function instructions() { return (document.getElementById('edit-skill-instructions') as HTMLTextAreaElement).value; }

describe('skill editing', () => {
  it('prefills built-in instructions and stages restore until Save', async () => {
    const close = vi.fn();
    act(() => root.render(<EditSkillDialog workspaceId="ws" skill={skill} open onOpenChange={close} />));
    expect(instructions()).toBe('Current instructions');
    act(() => button('Restore to default').click());
    expect(instructions()).toBe('Default instructions');
    expect(mocks.save).not.toHaveBeenCalled();
    await act(async () => button('Save').click());
    expect(mocks.save).toHaveBeenCalledWith({ skillId: skill.key, data: { ...defaults, restore_default: true, expected_version: 'current-version' } });
    expect(close).toHaveBeenCalledWith(false);
  });
  it('saves an imported skill by ID and keeps the editor open on failure', async () => {
    mocks.save.mockRejectedValue(new Error('Skill changed; reload'));
    const close = vi.fn();
    act(() => root.render(<EditSkillDialog workspaceId="ws" skill={{ ...skill, id: 'imported-id', source_kind: 'imported' }} open onOpenChange={close} />));
    await act(async () => button('Save').click());
    expect(mocks.save).toHaveBeenCalledWith({ skillId: 'imported-id', data: { title: skill.title, description: skill.description, instructions: skill.instructions, expected_version: 'current-version' } });
    expect(close).not.toHaveBeenCalled();
    expect(mocks.toastError).toHaveBeenCalledWith('Skill changed; reload');
    expect(instructions()).toBe('Current instructions');
  });
  it('does not save a restore when cancelled', () => {
    act(() => root.render(<EditSkillDialog workspaceId="ws" skill={skill} open onOpenChange={vi.fn()} />));
    act(() => button('Restore to default').click());
    act(() => button('Cancel').click());
    expect(mocks.save).not.toHaveBeenCalled();
  });
});

import { describe, expect, it } from 'vitest';
import { isCreateTaskModalDirty } from '../CreateTaskModal';

const baseForm = {
  name: '',
  task_type: 'feature' as const,
  description: '',
  priority: 'medium' as const,
  severity: 'none' as const,
  estimate: '',
  epic_id: '',
  sprint_id: '',
  team_id: '',
  owner_member_ids: [] as string[],
  requester_member_id: '',
  deadline: '',
  label_ids: [] as string[],
  checklist_items: [] as { text: string; position?: number }[],
  external_links: [] as { url: string; title?: string }[],
};

describe('isCreateTaskModalDirty', () => {
  it('does not mark an unchanged edited template as dirty just because it has a title', () => {
    const templateForm = {
      ...baseForm,
      name: 'Test story 3',
      description: '<p>Template description</p>',
      team_id: 'team-1',
      owner_member_ids: ['member-1', 'member-2'],
      label_ids: ['label-1'],
    };

    expect(
      isCreateTaskModalDirty({
        mode: 'template',
        editingTemplate: true,
        form: templateForm,
        baselineForm: templateForm,
        currentDescription: templateForm.description,
        baselineDescription: templateForm.description,
        stateId: 'state-1',
        baselineStateId: 'state-1',
      }),
    ).toBe(false);
  });

  it('marks an edited template as dirty when a saved field changes', () => {
    const baselineForm = {
      ...baseForm,
      name: 'Test story 3',
      team_id: 'team-1',
      label_ids: ['label-1'],
    };
    const currentForm = {
      ...baselineForm,
      label_ids: ['label-1', 'label-2'],
    };

    expect(
      isCreateTaskModalDirty({
        mode: 'template',
        editingTemplate: true,
        form: currentForm,
        baselineForm,
        currentDescription: currentForm.description,
        baselineDescription: baselineForm.description,
        stateId: 'state-1',
        baselineStateId: 'state-1',
      }),
    ).toBe(true);
  });
});

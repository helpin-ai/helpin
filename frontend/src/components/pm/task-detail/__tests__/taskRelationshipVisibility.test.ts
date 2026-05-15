import { describe, expect, it } from 'vitest';

import { hasVisibleTaskAssociations } from '../taskRelationshipVisibility';
import type { GroupedAssociations } from '@/lib/pmTypes';

function associations(overrides: Partial<GroupedAssociations> = {}): GroupedAssociations {
  return {
    task_relationships: {
      blocked_by: [],
      blocking: [],
      relates_to: [],
      related_by: [],
      duplicates: [],
      duplicated_by: [],
    },
    tasks: [],
    support_conversations: [],
    crm_records: [],
    docs: [],
    ...overrides,
  };
}

describe('hasVisibleTaskAssociations', () => {
  it('returns true when a task has relationships or linked docs', () => {
    expect(hasVisibleTaskAssociations(associations({
      task_relationships: {
        blocked_by: [{ relationship_id: 'rel-1', link_type: 'is_blocked_by', is_active: true }],
        blocking: [],
        relates_to: [],
        related_by: [],
        duplicates: [],
        duplicated_by: [],
      },
    }))).toBe(true);

    expect(hasVisibleTaskAssociations(associations({
      docs: [{ object_type: 'document', object_id: 'doc-1', title: 'Spec' }],
    }))).toBe(true);
  });

  it('returns false when relationship and doc association groups are empty', () => {
    expect(hasVisibleTaskAssociations(associations())).toBe(false);
    expect(hasVisibleTaskAssociations(null)).toBe(false);
  });
});

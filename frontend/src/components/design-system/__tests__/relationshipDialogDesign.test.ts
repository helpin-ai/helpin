import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const source = (path: string) => readFileSync(resolve(__dirname, path), 'utf8');

const quietSource = source('../quiet.tsx');
const relationshipPickerSources = [
  source('../../crm/AssociationsList.tsx'),
  source('../../crm/CompanyDetailCollections.tsx'),
  source('../../crm/EmailTimeline.tsx'),
  source('../../crm/LinkedTasksPanel.tsx'),
  source('../../crm/deal-detail/DealRelationships.tsx'),
  source('../../pm/AssociationsPanel.tsx'),
  source('../../pm/TaskRelationshipsSection.tsx'),
  source('../../support/SidebarAssociations.tsx'),
  source('../../../pages/crm/ContactDetail.tsx'),
];

describe('relationship picker dialog design', () => {
  it('grows responsively while containing long result content', () => {
    expect(quietSource).toContain('sm:max-w-xl lg:max-w-2xl xl:max-w-3xl');
    expect(quietSource).toContain('min-w-0 overflow-hidden');
    expect(quietSource).toContain("quietRelationshipResultRowClassName = 'min-w-0 overflow-hidden'");

    for (const pickerSource of relationshipPickerSources) {
      expect(pickerSource).toContain('<QuietRelationshipDialogContent');
      expect(pickerSource).toContain('<QuietRelationshipResults');
      expect(pickerSource).toContain('quietRelationshipResultRowClassName');
    }
  });
});

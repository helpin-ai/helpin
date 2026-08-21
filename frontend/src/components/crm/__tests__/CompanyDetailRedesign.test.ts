import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const companyDetailSource = readFileSync(
  resolve(__dirname, '../../../pages/crm/CompanyDetail.tsx'),
  'utf8',
);
const createDealSource = readFileSync(
  resolve(__dirname, '../CreateDealDialog.tsx'),
  'utf8',
);
const enrichmentSource = readFileSync(
  resolve(__dirname, '../contact-detail/EnrichmentRailCard.tsx'),
  'utf8',
);

describe('Company detail divider redesign', () => {
  it('uses the task-detail rail scale and scoped borderless surfaces', () => {
    expect(companyDetailSource).toContain('lg:grid-cols-[minmax(0,1fr)_300px]');
    expect(companyDetailSource).toContain('grid-cols-[16px_72px_1fr]');
    expect(companyDetailSource).toContain('<RichTextMentionContent');
    expect(companyDetailSource).toContain('<DetailDescriptionEditorActions');
    expect(companyDetailSource).toContain('presentation="borderless"');
  });

  it('retains the existing association rail presentation', () => {
    const associationUsage = companyDetailSource.slice(
      companyDetailSource.indexOf('<AssociationsList'),
      companyDetailSource.indexOf('</aside>'),
    );

    expect(associationUsage).not.toContain('presentation=');
    expect(associationUsage).toContain('currentObjectType="company"');
  });

  it('links deals created from company context back to the company', () => {
    expect(companyDetailSource).toContain('companyContext={{ id: companyId');
    expect(createDealSource).toContain("from_object_type: 'deal'");
    expect(createDealSource).toContain("to_object_type: 'company'");
    expect(createDealSource).toContain('Deal created, but it could not be linked');
  });

  it('keeps the new enrichment treatment opt-in', () => {
    expect(enrichmentSource).toContain("presentation?: 'default' | 'borderless'");
    expect(enrichmentSource).toContain("presentation={presentation}");
  });
});

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const dealSource = readFileSync(resolve(__dirname, '../DealDetail.tsx'), 'utf8');
const relationshipsSource = readFileSync(resolve(__dirname, '../../../components/crm/deal-detail/DealRelationships.tsx'), 'utf8');
const associationsSource = readFileSync(resolve(__dirname, '../../../components/crm/AssociationsList.tsx'), 'utf8');
const createSource = readFileSync(resolve(__dirname, '../../../components/crm/CreateDealDialog.tsx'), 'utf8');

describe('CRM deal customer and Quiet detail composition', () => {
  it('uses the standard editable detail header without repeating the deal name', () => {
    expect(dealSource).toContain('<QuietDetailHeader');
    expect(dealSource).toContain('<QuietBreadcrumbs');
    expect(dealSource).toContain('<QuietTitleInput');
    expect(dealSource).toContain('presentation="header"');
    expect(dealSource).toContain('<SaveIndicator saving={saving} error={saveError} presentation="quiet"');
		expect(dealSource).not.toContain('<QuietStatusText');
		expect(dealSource).toContain('<DealStagePath');
    expect(dealSource).toContain('<QuietDetailLayout');
  });

  it('keeps the rail at the Task detail density while giving record names text-sm hierarchy', () => {
    expect(dealSource).toContain('grid-cols-[16px_72px_1fr]');
    expect(dealSource).toContain('gap-y-2.5');
    expect(dealSource).toContain('text-[12px]');
    expect(relationshipsSource).toContain('text-xs font-semibold uppercase tracking-wide text-foreground/70');
    expect(relationshipsSource).toContain('truncate text-sm font-medium');
    expect(associationsSource).toContain("cn('truncate', quietRelatedItemTitleClassName)");
  });

  it('separates the canonical customer from removable participants', () => {
    expect(relationshipsSource).toContain("association_label === 'deal_customer'");
    expect(relationshipsSource).toContain("association_label !== 'deal_customer'");
    expect(relationshipsSource).toContain('A company-backed deal does not require a contact.');
    expect(relationshipsSource).toContain("setPicker('customer')");
    expect(relationshipsSource).toContain("setPicker('participant')");
  });

  it('requires one searched customer during creation and makes company contact optional', () => {
    expect(createSource).toContain('Every deal belongs to one company or independent contact.');
    expect(createSource).toContain('Search contacts or companies');
    expect(createSource).toContain('No primary contact');
		expect(createSource).toContain('!customer || !effectivePipelineId || !effectiveStageId');
  });
});

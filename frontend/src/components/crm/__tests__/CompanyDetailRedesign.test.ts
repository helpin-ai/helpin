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
const linkedTasksSource = readFileSync(
  resolve(__dirname, '../LinkedTasksPanel.tsx'),
  'utf8',
);
const companyTasksWorkspaceSource = readFileSync(
  resolve(__dirname, '../CompanyTasksWorkspace.tsx'),
  'utf8',
);

describe('Company detail divider redesign', () => {
  it('uses the task-detail rail scale and scoped borderless surfaces', () => {
    expect(companyDetailSource).toContain('lg:grid-cols-[minmax(0,1fr)_300px]');
    expect(companyDetailSource).toContain('grid-cols-[16px_72px_1fr]');
    expect(companyDetailSource).toContain('<RichTextMentionContent');
    expect(companyDetailSource).toContain('<DetailDescriptionEditorActions');
    expect(companyDetailSource).toContain('<DetailDescriptionEditButton');
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

  it('uses the unified cursor-paginated company activity timeline', () => {
    expect(companyDetailSource).toContain('useCompanyTimeline');
    expect(companyDetailSource).toContain('timeline.data?.pages.flatMap');
    expect(companyDetailSource).toContain('onTimelineFilterChange={setActivityFilter}');
    expect(companyDetailSource).toContain('onLoadMore={() => void timeline.fetchNextPage()}');
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

  it('always chooses a team from an add-task action matching link existing', () => {
    expect(linkedTasksSource).toContain('CheckListIcon');
    expect(linkedTasksSource).not.toContain('CheckmarkSquare02Icon');
    expect(linkedTasksSource).toContain('<SidebarPopoverSelect');
    expect(linkedTasksSource).not.toContain('triggerVariant="underline"');
    expect(linkedTasksSource).toContain('showChevron');
    expect(linkedTasksSource).toContain('<PlusSignIcon');
    expect(linkedTasksSource).toContain('handleStartCreate(teamId)');
    expect(linkedTasksSource).toContain("getOptionalSectionActionClass(canEdit ? 'available' : 'locked', 'borderless')");
    expect(linkedTasksSource).toContain("getOptionalSectionActionClass(canCreateTask && !teamsLoading && !openingCreate ? 'available' : 'locked', 'borderless')");
    expect(linkedTasksSource).not.toContain('<DropdownMenu>');
  });

  it('embeds a bounded customer-scoped task table and minimal board', () => {
    expect(linkedTasksSource).toContain('<CompanyTasksWorkspace');
    expect(linkedTasksSource).toContain("associationTarget.type === 'company'");
    expect(companyTasksWorkspaceSource).toContain('company_id: companyId');
    expect(companyTasksWorkspaceSource).toContain('max-h-[470px]');
    expect(companyTasksWorkspaceSource).toContain('fitContent');
    expect(companyTasksWorkspaceSource).toContain('<TaskListView');
    expect(companyTasksWorkspaceSource).toContain('<CompanyTaskBoard');
    expect(companyTasksWorkspaceSource).toContain("stateType: 'backlog'");
    expect(companyTasksWorkspaceSource).toContain("stateType: 'done'");
    expect(companyTasksWorkspaceSource).not.toContain('rounded-md border border-border/60 bg-muted/20');
  });
});

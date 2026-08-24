import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('TaskDetailPanel optional section spacing', () => {
  it('keeps overview essentials visible and consolidates linked context in Related', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const overviewIndex = panelSource.indexOf("activeView === 'overview'");
    const attachmentsIndex = panelSource.indexOf('id="attachments-section"');
    const checklistIndex = panelSource.indexOf('<ChecklistItems');
    const deliverySettingsIndex = panelSource.indexOf('<TaskDeliveryRailSection');
    const relatedIndex = panelSource.indexOf('<details id="task-related-section"');
    const relationshipsIndex = panelSource.indexOf('<TaskRelationshipsSection');
    const docsIndex = panelSource.indexOf('section="docs"');
    const externalLinksIndex = panelSource.indexOf('<ExternalLinks', relationshipsIndex);
    const supportIndex = panelSource.indexOf('section="support"');
    const crmIndex = panelSource.indexOf('section="crm"');

    expect(overviewIndex).toBeGreaterThan(-1);
    expect(panelSource).not.toContain('<TaskStandingBriefCard');
    expect(panelSource).not.toContain('handleBriefSuggestion');
    expect(panelSource).toContain('className="mt-6 border-t border-border/60 pt-6" data-testid="checklist-section"');
    expect(attachmentsIndex).toBeGreaterThan(overviewIndex);
    expect(checklistIndex).toBeGreaterThan(attachmentsIndex);
    expect(deliverySettingsIndex).toBeGreaterThan(checklistIndex);
    expect(relatedIndex).toBeGreaterThan(deliverySettingsIndex);
    expect(relatedIndex).toBeGreaterThan(checklistIndex);
    expect(relationshipsIndex).toBeGreaterThan(relatedIndex);
    expect(docsIndex).toBeGreaterThan(relationshipsIndex);
    expect(externalLinksIndex).toBeGreaterThan(docsIndex);
    expect(supportIndex).toBeGreaterThan(externalLinksIndex);
    expect(crmIndex).toBeGreaterThan(supportIndex);
    expect(panelSource).not.toContain('<TabsList variant="line"');
    expect(panelSource).toContain('<div className="mt-3 space-y-0">');
    expect(panelSource).toContain('<div className="my-2 h-px bg-border/60" />');
    expect(panelSource).toContain('hideDocs');
    expect(panelSource).toContain('flat');
    expect(panelSource).toContain('excludeDocs');
    expect(panelSource).toContain('const [deliverySectionOpen, setDeliverySectionOpen] = useState(false)');
    expect(panelSource).toContain('if (panelOpen) setDeliverySectionOpen(false)');
    expect(panelSource).toContain('data-testid="task-delivery-settings"');
    expect(panelSource).toContain('label="Repository"');
    expect(panelSource).toContain('label="Base branch"');
    expect(panelSource).toContain('label="Task branch"');
    expect(panelSource).toContain('label="Source"');
    expect(panelSource).toContain('Use default');
    expect(panelSource).toContain('Use epic branch');
    expect(panelSource).toContain('ACTIVE_RUN_STATUSES.has(status)');
  });

  it('uses the two task views and existing shared task components', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const agentRunSource = readFileSync(resolve(__dirname, '../../AgentRunPanel.tsx'), 'utf8');

    expect(panelSource).toContain("(['overview', 'delivery'] as TaskDetailView[])");
    expect(panelSource).toContain('<TaskUpdatesView');
    expect(panelSource).toContain('<AgentRunPanel');
    expect(panelSource).not.toContain('<TaskGitPanel');
    expect(panelSource).toContain('showDevelopmentHistory={hasGitIntegration && fieldVis.dev_history}');
    expect(panelSource).toContain('onEditDeliveryContext={openDeliveryContext}');
    expect(panelSource).toContain('<ChecklistItems');
    expect(panelSource).toContain('<Attachments');
    expect(panelSource).toContain('showAddAction');
    expect(panelSource).toContain('emptyPresentation="inline-action"');
    expect(agentRunSource).toContain('Execution context');
    expect(agentRunSource).toContain('<TaskDeliveryTimeline');
    expect(agentRunSource).toContain('Working branch');
    expect(agentRunSource).not.toContain('aria-label="Edit execution context"');
    expect(agentRunSource).not.toContain('<RepositoryBranchPicker');
  });

  it('keeps updates in Overview while delivery remains separate', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const overviewStart = panelSource.indexOf("activeView === 'overview'");
    const deliveryStart = panelSource.indexOf("activeView === 'delivery'");
    const overviewBlock = panelSource.slice(overviewStart, deliveryStart);

    expect(overviewBlock).toContain('<TaskDetailSectionHeading title="Updates"');
    expect(overviewBlock).toContain('<TaskUpdatesView');
    expect(overviewBlock).not.toContain('<AgentRunPanel');
    expect(panelSource.indexOf('<TaskUpdatesView')).toBeLessThan(deliveryStart);
  });
});

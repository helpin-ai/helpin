import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('TaskDetailPanel optional section spacing', () => {
  it('keeps overview essentials visible and consolidates linked context in Related', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const overviewIndex = panelSource.indexOf("activeView === 'overview'");
    const checklistIndex = panelSource.indexOf('<ChecklistItems');
    const relatedIndex = panelSource.indexOf('<details id="task-related-section"');
    const relationshipsIndex = panelSource.indexOf('<TaskRelationshipsSection');
    const externalLinksIndex = panelSource.indexOf('<ExternalLinks');
    const associationsIndex = panelSource.indexOf('<AssociationsPanel');

    expect(overviewIndex).toBeGreaterThan(-1);
    expect(panelSource).toContain('<TaskStandingBriefCard');
    expect(checklistIndex).toBeGreaterThan(overviewIndex);
    expect(relatedIndex).toBeGreaterThan(checklistIndex);
    expect(relationshipsIndex).toBeGreaterThan(relatedIndex);
    expect(externalLinksIndex).toBeGreaterThan(relationshipsIndex);
    expect(associationsIndex).toBeGreaterThan(externalLinksIndex);
    expect(panelSource).toContain('excludeDocs');
  });

  it('uses the three task views and existing shared task components', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');

    expect(panelSource).toContain("(['overview', 'updates', 'delivery'] as TaskDetailView[])");
    expect(panelSource).toContain('<TaskUpdatesView');
    expect(panelSource).toContain('<AgentRunPanel');
    expect(panelSource).toContain('<TaskGitPanel');
    expect(panelSource).toContain('<ChecklistItems');
    expect(panelSource).toContain('<Attachments');
  });

  it('keeps comments and history out of Overview', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const overviewStart = panelSource.indexOf("activeView === 'overview'");
    const deliveryStart = panelSource.indexOf("activeView === 'delivery'");
    const overviewBlock = panelSource.slice(overviewStart, deliveryStart);

    expect(overviewBlock).not.toContain('<TaskUpdatesView');
    expect(overviewBlock).not.toContain('<AgentRunPanel');
    expect(panelSource.indexOf('<TaskUpdatesView')).toBeGreaterThan(deliveryStart);
  });
});

import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('task detail optional section labels', () => {
  it('uses slightly darker text for task detail section labels', () => {
    const files = {
      checklist: readFileSync(resolve(__dirname, '../ChecklistItems.tsx'), 'utf8'),
      relationships: readFileSync(resolve(__dirname, '../TaskRelationshipsSection.tsx'), 'utf8'),
      externalLinks: readFileSync(resolve(__dirname, '../ExternalLinks.tsx'), 'utf8'),
      attachments: readFileSync(resolve(__dirname, '../Attachments.tsx'), 'utf8'),
      panel: readFileSync(resolve(__dirname, '../TaskDetailPanel.tsx'), 'utf8'),
      associations: readFileSync(resolve(__dirname, '../AssociationsPanel.tsx'), 'utf8'),
      agentRuns: readFileSync(resolve(__dirname, '../AgentRunPanel.tsx'), 'utf8'),
    };

    expect(files.checklist).toContain('className="text-xs font-semibold text-foreground/70 uppercase tracking-wide"');
    expect(files.relationships).toContain('className="text-[12.5px] font-semibold text-foreground/70 uppercase tracking-wide"');
    expect(files.externalLinks).toContain('className="text-[12.5px] font-semibold text-foreground/70 uppercase tracking-wide"');
    expect(files.attachments).toContain('className="text-xs font-semibold text-foreground/70 uppercase tracking-wide"');
    expect(files.panel).toContain('cursor-pointer select-none text-xs font-semibold uppercase tracking-wide text-foreground/70');
    expect(files.associations).toContain('className="text-[12.5px] font-semibold uppercase tracking-wide text-foreground/70"');
    expect(files.agentRuns).toContain('className="text-xs font-semibold uppercase tracking-wide text-foreground/70"');
  });

  it('keeps agent runs visible while gating Git activity inside the unified delivery view', () => {
    const panel = readFileSync(resolve(__dirname, '../TaskDetailPanel.tsx'), 'utf8');
    const agentRunPanelIndex = panel.indexOf('<AgentRunPanel');

    expect(agentRunPanelIndex).toBeGreaterThan(-1);
    expect(panel).toContain('showDevelopmentHistory={hasGitIntegration && fieldVis.dev_history}');
    expect(panel).not.toContain('<TaskGitPanel');
  });

  it('uses the shared related-item title typography across tasks and epics', () => {
    const quiet = readFileSync(resolve(__dirname, '../../design-system/quiet.tsx'), 'utf8');
    const relationships = readFileSync(resolve(__dirname, '../TaskRelationshipsSection.tsx'), 'utf8');
    const associations = readFileSync(resolve(__dirname, '../AssociationsPanel.tsx'), 'utf8');
    const externalLinks = readFileSync(resolve(__dirname, '../ExternalLinks.tsx'), 'utf8');
    const taskPanel = readFileSync(resolve(__dirname, '../TaskDetailPanel.tsx'), 'utf8');
    const epicDetail = readFileSync(resolve(__dirname, '../../../pages/pm/EpicDetail.tsx'), 'utf8');

    expect(quiet).toContain("'text-[12.5px] font-medium text-foreground/90'");
    expect(relationships).toContain('quietRelatedItemTitleClassName');
    expect(associations).toContain('quietRelatedItemTitleClassName');
    expect(externalLinks).toContain('quietRelatedItemTitleClassName');
    expect(taskPanel).toContain('<AssociationsPanel');
    expect(epicDetail).toContain('<AssociationsPanel');
  });
});

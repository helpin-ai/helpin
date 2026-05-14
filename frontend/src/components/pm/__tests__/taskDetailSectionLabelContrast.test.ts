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
    expect(files.relationships).toContain('className="text-xs font-semibold text-foreground/70 uppercase tracking-wide"');
    expect(files.externalLinks).toContain('className="text-xs font-semibold text-foreground/70 uppercase tracking-wide"');
    expect(files.attachments).toContain('className="text-xs font-semibold text-foreground/70 uppercase tracking-wide"');
    expect(files.panel).toContain('className="mb-3 text-xs font-semibold uppercase tracking-wide text-foreground/70"');
    expect(files.panel).toContain('className="text-xs font-semibold text-foreground/70 uppercase tracking-wide"');
    expect(files.associations).toContain('className="text-xs font-semibold uppercase tracking-wide text-foreground/70"');
    expect(files.agentRuns).toContain('className="text-xs font-semibold uppercase tracking-wide text-foreground/70"');
  });
});

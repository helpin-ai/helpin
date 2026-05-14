import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('TaskDetailSectionHeading', () => {
  it('uses a subtle separator under task detail section labels', () => {
    const source = readFileSync(resolve(__dirname, '../TaskDetailSectionHeading.tsx'), 'utf8');

    expect(source).toContain('border-b border-border/50 pb-2');
    expect(source).toContain('text-xs font-semibold uppercase tracking-wide text-foreground/70');
  });

  it('is used for dense task detail sections', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const relationshipsSource = readFileSync(resolve(__dirname, '../../TaskRelationshipsSection.tsx'), 'utf8');
    const checklistSource = readFileSync(resolve(__dirname, '../../ChecklistItems.tsx'), 'utf8');
    const externalLinksSource = readFileSync(resolve(__dirname, '../../ExternalLinks.tsx'), 'utf8');

    expect(panelSource).toContain('<TaskDetailSectionHeading title="Comments"');
    expect(panelSource).toContain('<TaskDetailSectionHeading title="Activity"');
    expect(relationshipsSource).toContain('<TaskDetailSectionHeading title="Task Relationships"');
    expect(checklistSource).toContain('<TaskDetailSectionHeading');
    expect(checklistSource).toContain('title="Checklist"');
    expect(externalLinksSource).toContain('<TaskDetailSectionHeading title="External Links"');
  });
});

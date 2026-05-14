import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('TaskDetailSectionHeading', () => {
  it('uses a subtle separator and one icon accent for task detail section labels', () => {
    const source = readFileSync(resolve(__dirname, '../TaskDetailSectionHeading.tsx'), 'utf8');

    expect(source).toContain('border-b border-border/50 pb-2 pt-1.5');
    expect(source).toContain('text-primary/80');
    expect(source).toContain('text-xs font-semibold uppercase tracking-wide text-foreground/75');
  });

  it('is used for dense task detail sections', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const relationshipsSource = readFileSync(resolve(__dirname, '../../TaskRelationshipsSection.tsx'), 'utf8');
    const checklistSource = readFileSync(resolve(__dirname, '../../ChecklistItems.tsx'), 'utf8');
    const externalLinksSource = readFileSync(resolve(__dirname, '../../ExternalLinks.tsx'), 'utf8');

    expect(panelSource).toContain('<TaskDetailSectionHeading title="Comments" icon={Message01Icon}');
    expect(panelSource).toContain('<TaskDetailSectionHeading title="Activity" icon={Activity01Icon}');
    expect(relationshipsSource).toContain('<TaskDetailSectionHeading title="Task Relationships"');
    expect(checklistSource).toContain('<TaskDetailSectionHeading');
    expect(checklistSource).toContain('title="Checklist"');
    expect(externalLinksSource).toContain('<TaskDetailSectionHeading title="External Links"');
  });

  it('keeps the add relationship action compact', () => {
    const relationshipsSource = readFileSync(resolve(__dirname, '../../TaskRelationshipsSection.tsx'), 'utf8');

    expect(relationshipsSource).toContain('px-2 py-1.5 text-[11px]');
    expect(relationshipsSource).toContain('<PlusSignIcon className="h-3 w-3" />');
  });

  it('keeps checklist and external link add actions visually aligned with relationship add', () => {
    const checklistSource = readFileSync(resolve(__dirname, '../../ChecklistItems.tsx'), 'utf8');
    const externalLinksSource = readFileSync(resolve(__dirname, '../../ExternalLinks.tsx'), 'utf8');

    expect(checklistSource).toContain('Add item');
    expect(checklistSource).toContain('h-6 shrink-0 gap-1 px-2 text-[11px]');
    expect(checklistSource).toContain('<PlusSignIcon className="h-3 w-3" />');
    expect(externalLinksSource).toContain('Add link');
    expect(externalLinksSource).toContain('h-6 shrink-0 gap-1 px-2 text-[11px]');
    expect(externalLinksSource).toContain('<PlusSignIcon className="h-3 w-3" />');
  });
});

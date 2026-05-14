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

    expect(relationshipsSource).toContain('variant="outline"');
    expect(relationshipsSource).toContain('size="xs"');
    expect(relationshipsSource).toContain('<PlusSignIcon />');
  });

  it('opens related task rows in a new tab', () => {
    const relationshipsSource = readFileSync(resolve(__dirname, '../../TaskRelationshipsSection.tsx'), 'utf8');

    expect(relationshipsSource).toContain('to="/w/$slug/pm/tasks/$taskId"');
    expect(relationshipsSource).toContain('target="_blank"');
    expect(relationshipsSource).toContain('rel="noopener noreferrer"');
  });

  it('keeps checklist and external link add actions visually aligned with relationship add', () => {
    const checklistSource = readFileSync(resolve(__dirname, '../../ChecklistItems.tsx'), 'utf8');
    const externalLinksSource = readFileSync(resolve(__dirname, '../../ExternalLinks.tsx'), 'utf8');

    expect(checklistSource).toContain('const [addingItem, setAddingItem] = useState(false);');
    expect(checklistSource).toContain('Add item');
    expect(checklistSource).toContain('setAddingItem(true)');
    expect(checklistSource).toContain('group flex items-center gap-2 py-0.5');
    expect(checklistSource).toContain('variant="outline"');
    expect(checklistSource).toContain('size="xs"');
    expect(checklistSource).toContain('<PlusSignIcon />');
    expect(externalLinksSource).toContain('const [addingLink, setAddingLink] = useState(false);');
    expect(externalLinksSource).toContain('Add link');
    expect(externalLinksSource).toContain('setAddingLink(true)');
    expect(externalLinksSource).toContain('variant="outline"');
    expect(externalLinksSource).toContain('size="xs"');
    expect(externalLinksSource).toContain('<PlusSignIcon />');
  });
});

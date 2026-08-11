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

  it('unifies comments and activity in Updates while compact sections retain their own headers', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const updatesSource = readFileSync(resolve(__dirname, '../TaskUpdatesView.tsx'), 'utf8');
    const relationshipsSource = readFileSync(resolve(__dirname, '../../TaskRelationshipsSection.tsx'), 'utf8');
    const checklistSource = readFileSync(resolve(__dirname, '../../ChecklistItems.tsx'), 'utf8');
    const externalLinksSource = readFileSync(resolve(__dirname, '../../ExternalLinks.tsx'), 'utf8');

    expect(panelSource).toContain('<TaskUpdatesView');
    expect(updatesSource).toContain('<CommentThread');
    expect(updatesSource).toContain("(['all', 'discussion', 'changes'] as TaskUpdateFilter[])");
    expect(panelSource).not.toContain('<TaskDetailSectionHeading title="Comments"');
    expect(panelSource).not.toContain('<TaskDetailSectionHeading title="Activity"');
    expect(relationshipsSource).toContain('Task Relationships');
    expect(checklistSource).toContain('Checklist');
    expect(externalLinksSource).toContain('External Links');
    expect(relationshipsSource).toContain('rounded-lg border border-border/60 bg-card');
    expect(checklistSource).toContain('rounded-lg border border-border/60 bg-card');
    expect(externalLinksSource).toContain('rounded-lg border border-border/60 bg-card');
  });

  it('keeps the add relationship action inline', () => {
    const relationshipsSource = readFileSync(resolve(__dirname, '../../TaskRelationshipsSection.tsx'), 'utf8');

    expect(relationshipsSource).toContain('Add relationship');
    expect(relationshipsSource).toContain('text-xs text-muted-foreground transition-colors hover:text-foreground');
    expect(relationshipsSource).toContain('<PlusSignIcon className="h-3 w-3" />');
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
    expect(checklistSource).toContain('text-xs text-muted-foreground transition-colors hover:text-foreground');
    expect(checklistSource).toContain('<PlusSignIcon className="h-3 w-3" />');
    expect(externalLinksSource).toContain('const [addingLink, setAddingLink] = useState(false);');
    expect(externalLinksSource).toContain('Add link');
    expect(externalLinksSource).toContain('setAddingLink(true)');
    expect(externalLinksSource).toContain('text-xs text-muted-foreground transition-colors hover:text-foreground');
    expect(externalLinksSource).toContain('<PlusSignIcon className="h-3 w-3" />');
  });
});

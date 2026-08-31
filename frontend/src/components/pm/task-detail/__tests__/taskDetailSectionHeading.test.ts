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
    const updateRowSource = readFileSync(resolve(__dirname, '../../UpdateActivityRow.tsx'), 'utf8');
    const relationshipsSource = readFileSync(resolve(__dirname, '../../TaskRelationshipsSection.tsx'), 'utf8');
    const checklistSource = readFileSync(resolve(__dirname, '../../ChecklistItems.tsx'), 'utf8');
    const externalLinksSource = readFileSync(resolve(__dirname, '../../ExternalLinks.tsx'), 'utf8');

    expect(panelSource).toContain('<TaskUpdatesView');
    expect(panelSource).toContain('<TaskDetailSectionHeading title="Updates" icon={Activity01Icon}');
    expect(updatesSource).toContain('<CommentThread');
    expect(updatesSource).toContain("from '@/components/ui/tabs'");
    expect(updatesSource).toContain('<Tabs value={filter}');
    expect(updatesSource).toContain('<TabsList variant="line" aria-label="Update type"');
    expect(updatesSource).toContain('<TabsTrigger');
    expect(updatesSource).toContain('<UpdateActivityRow');
    expect(updateRowSource).toContain('<AgentAvatar');
    expect(updateRowSource).toContain('<UserAvatar');
    expect(updatesSource).toContain('!agentActivity.automated');
    expect(updateRowSource).toContain('font-semibold text-foreground/90');
    expect(updatesSource).toContain('taskUpdateAgentPresentation');
    expect(updatesSource).toContain("{ value: 'all', label: 'All' }");
    expect(updatesSource).toContain("{ value: 'discussion', label: 'Discussion' }");
    expect(updatesSource).toContain("{ value: 'changes', label: 'Changes' }");
    expect(panelSource).not.toContain('<TaskDetailSectionHeading title="Comments"');
    expect(panelSource).not.toContain('<TaskDetailSectionHeading title="Activity"');
    expect(relationshipsSource).toContain('Task Relationships');
    expect(checklistSource).toContain('Checklist');
    expect(externalLinksSource).toContain('External Links');
    expect(relationshipsSource).toContain("cn(!flat && 'rounded-lg border border-border/60 bg-card')");
    expect(checklistSource).not.toContain('rounded-lg border border-border/60 bg-card');
    expect(externalLinksSource).toContain("flat ? undefined : 'rounded-lg border border-border/60 bg-card'");
  });

  it('keeps the add relationship action inline', () => {
    const relationshipsSource = readFileSync(resolve(__dirname, '../../TaskRelationshipsSection.tsx'), 'utf8');

    expect(relationshipsSource).toContain('Add relationship');
    expect(relationshipsSource).toContain('aria-label="Add task relationship"');
    expect(relationshipsSource).toContain('rounded-md p-0.5 text-muted-foreground');
  });

  it('opens related task rows in the existing task panel', () => {
    const relationshipsSource = readFileSync(resolve(__dirname, '../../TaskRelationshipsSection.tsx'), 'utf8');

    expect(relationshipsSource).toContain('openTaskRoute(');
    expect(relationshipsSource).not.toContain('target="_blank"');
  });

  it('keeps checklist and external link add actions visually aligned with relationship add', () => {
    const checklistSource = readFileSync(resolve(__dirname, '../../ChecklistItems.tsx'), 'utf8');
    const externalLinksSource = readFileSync(resolve(__dirname, '../../ExternalLinks.tsx'), 'utf8');

    expect(checklistSource).toContain('const [addingItem, setAddingItem] = useState(false);');
    expect(checklistSource).toContain('Add item');
    expect(checklistSource).toContain('setAddingItem(true)');
    expect(checklistSource).toContain('aria-label="Add checklist item"');
    expect(checklistSource).toContain('rounded-md p-0.5 text-muted-foreground');
    expect(externalLinksSource).toContain('const [addingLink, setAddingLink] = useState(false);');
    expect(externalLinksSource).toContain('Add link');
    expect(externalLinksSource).toContain('setAddingLink(true)');
    expect(externalLinksSource).toContain('aria-label="Add external link"');
    expect(externalLinksSource).toContain('rounded-md p-0.5 text-muted-foreground');
  });
});

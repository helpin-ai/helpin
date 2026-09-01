import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));
const source = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');

describe('TaskDetailPanel shared detail header', () => {
  it('keeps one editable task identity in the shared header', () => {
    expect(source).toContain('<QuietDetailHeader');
    expect(source).toContain('<QuietBreadcrumbs items={taskBreadcrumbItems} />');
    expect(source).toContain('presentation="header"');
    expect(source.match(/aria-label="Task title"/g)).toHaveLength(1);
    expect(source).toContain('<QuietMetaLine');
    expect(source).toContain('<QuietStatusText');
    expect(source).toContain('<QuietStatusText className="lg:hidden"');
    expect(source).toContain('presentation="quiet"');
    expect(source).not.toContain('ui-divider-bottom-fade');
  });

  it('preserves the complete optional planning breadcrumb chain', () => {
    const tasksIndex = source.indexOf("id: 'tasks'");
    const objectiveIndex = source.indexOf('id: `objective-');
    const epicIndex = source.indexOf('id: `epic-');
    const sprintIndex = source.indexOf('id: `sprint-');

    expect(tasksIndex).toBeGreaterThan(-1);
    expect(objectiveIndex).toBeGreaterThan(tasksIndex);
    expect(epicIndex).toBeGreaterThan(objectiveIndex);
    expect(sprintIndex).toBeGreaterThan(epicIndex);
    expect(source).toContain("to: '/w/$slug/pm/tasks'");
    expect(source).toContain('search: form.team_id ? { team: form.team_id } : {}');
    expect(source).toContain("to: '/w/$slug/pm/objectives/$objectiveId'");
    expect(source).toContain("to: '/w/$slug/pm/epics/$epicId'");
    expect(source).toContain("to: '/w/$slug/pm/sprints/$sprintId'");
    expect(source).toContain('icon: <Layers01Icon className="h-3.5 w-3.5 text-quiet-muted" />');
    expect(source).not.toContain('HexagonIcon');
  });

  it('keeps utilities in the header and run controls in Delivery', () => {
    const headerStart = source.indexOf('<QuietDetailHeader', source.indexOf('const taskBreadcrumbItems'));
    const headerEnd = source.indexOf('{duplicateNotice ?', headerStart);
    const header = source.slice(headerStart, headerEnd);

    expect(header).toContain("label={linkCopied ? 'Link copied' : 'Copy link'}");
    expect(header).toContain('label="Open in new tab"');
    expect(header).toContain('label="Close task"');
    expect(header).toContain('<DropdownMenu');
    expect(header).not.toContain('Run agent');
    expect(header).not.toContain('Open run');
    expect(source).toContain('<AgentRunPanel');
  });

  it('uses the same header skeleton while task data loads', () => {
    const loadingStart = source.indexOf(') : loading ? (');
    const loadingBlock = source.slice(loadingStart);

    expect(loadingBlock).toContain('<QuietDetailHeader');
    expect(loadingBlock).toContain("id: 'tasks', label: 'Tasks'");
    expect(loadingBlock).toContain('label="Close task"');
  });
});

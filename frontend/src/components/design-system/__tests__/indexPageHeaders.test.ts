import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const tasksSource = readFileSync(resolve(__dirname, '../../pm/KanbanBoard.tsx'), 'utf8');
const contactsSource = readFileSync(resolve(__dirname, '../../../pages/crm/Contacts.tsx'), 'utf8');
const companiesSource = readFileSync(resolve(__dirname, '../../../pages/crm/Companies.tsx'), 'utf8');
const dealsSource = readFileSync(resolve(__dirname, '../../../pages/crm/Deals.tsx'), 'utf8');
const epicsSource = readFileSync(resolve(__dirname, '../../../pages/pm/Epics.tsx'), 'utf8');
const sprintsSource = readFileSync(resolve(__dirname, '../../../pages/pm/Sprints.tsx'), 'utf8');
const roadmapSource = readFileSync(resolve(__dirname, '../../../pages/pm/Roadmap.tsx'), 'utf8');

describe('page-owned index headers', () => {
  it.each([
    ['Tasks', 'Add task', tasksSource],
    ['Contacts', 'Add contact', contactsSource],
    ['Companies', 'Add company', companiesSource],
    ['Deals', 'Add deal', dealsSource],
    ['Epics', 'Add epic', epicsSource],
    ['Sprints', 'Add sprint', sprintsSource],
    ['Roadmap', 'Add epic', roadmapSource],
  ])('keeps %s identity and its primary action in a compact Quiet header', (title, action, source) => {
    expect(source).toContain('<QuietPageHeader');
    expect(source).toContain('variant="shell"');
    expect(source).toContain(`title="${title}"`);
    expect(source).toContain(action);
  });

  it.each([
    ['Tasks', tasksSource],
    ['Epics', epicsSource],
    ['Sprints', sprintsSource],
  ])('keeps the active team as parenthetical context in the %s header', (_title, source) => {
    expect(source).toContain('context={team');
  });
});

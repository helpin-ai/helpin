import { describe, expect, it } from 'vitest';

import { buildTaskCopyUrl, buildTaskPath, buildTaskUrl } from '@/lib/pmTaskLinks';

describe('pmTaskLinks', () => {
  it('builds the canonical task path', () => {
    expect(buildTaskPath('acme-team', 'task-123')).toBe('/w/acme-team/pm/tasks/task-123');
  });

  it('builds the canonical task url when workspace context is available', () => {
    expect(
      buildTaskUrl({
        origin: 'https://stage.helpin.ai',
        slug: 'acme-team',
        taskId: 'task-123',
      }),
    ).toBe('https://stage.helpin.ai/w/acme-team/pm/tasks/task-123');
  });

  it('falls back to the current page task query when workspace context is unavailable', () => {
    expect(
      buildTaskCopyUrl({
        currentHref: 'https://stage.helpin.ai/w/acme-team/pm/epics/epic-7?tab=active',
        displayId: 42,
      }),
    ).toBe('https://stage.helpin.ai/w/acme-team/pm/epics/epic-7?tab=active&task=42');
  });

  it('uses task_key format in the URL when available', () => {
    expect(
      buildTaskCopyUrl({
        currentHref: 'https://stage.helpin.ai/w/acme-team/pm/epics/epic-7?tab=active',
        displayId: 42,
        taskKey: 'HLP-42',
      }),
    ).toBe('https://stage.helpin.ai/w/acme-team/pm/epics/epic-7?tab=active&task=HLP-42');
  });

  it('falls back to display_id when taskKey is null', () => {
    expect(
      buildTaskCopyUrl({
        currentHref: 'https://stage.helpin.ai/w/acme-team/pm/sprints/s-1',
        displayId: 99,
        taskKey: null,
      }),
    ).toBe('https://stage.helpin.ai/w/acme-team/pm/sprints/s-1?task=99');
  });
});

// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { TaskOwnerDistribution } from '@/components/pm/TaskOwnerDistribution';
import { buildTaskOwnerDistribution } from '@/components/pm/taskOwnerDistribution';
import { TooltipProvider } from '@/components/ui/tooltip';
import type { AssignableMember } from '@/lib/types';

vi.mock('@/components/pm/UserAvatar', () => ({
  UserAvatar: ({ name, avatarUrl }: { name?: string | null; avatarUrl?: string | null }) => (
    <span data-testid="owner-avatar" data-avatar-name={name} data-avatar-url={avatarUrl} />
  ),
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const members: AssignableMember[] = [
  {
    id: 'member-a',
    user_id: 'user-a',
    role: 'member',
    email: 'alice@example.com',
    display_name: 'Alice Johnson',
    avatar_url: 'https://cdn.example.com/alice.png',
    avatar_style: 'notionists',
    avatar_seed: 'alice',
    avatar_background_mode: 'solid',
    avatar_background_color: 'f0f0f0',
    status: 'active',
  },
  {
    id: 'member-b',
    role: 'member',
    email: 'bob@example.com',
    display_name: 'Bob Smith',
    status: 'active',
  },
];

describe('buildTaskOwnerDistribution', () => {
  it('counts task assignments, canonicalizes member ids, and keeps unassigned last', () => {
    const distribution = buildTaskOwnerDistribution(
      [
        { owner_member_ids: ['member-a', 'member-b'] },
        { owner_member_ids: ['user-a'] },
        { owner_member_ids: [] },
        { owner_member_ids: ['member-a', 'member-a'] },
      ],
      members,
    );

    expect(distribution.totalTasks).toBe(4);
    expect(distribution.totalAssignments).toBe(5);
    expect(distribution.ownerCount).toBe(2);
    expect(distribution.entries.map(({ id, taskCount, percentage }) => ({ id, taskCount, percentage }))).toEqual([
      { id: 'member-a', taskCount: 3, percentage: 60 },
      { id: 'member-b', taskCount: 1, percentage: 20 },
      { id: '__unassigned__', taskCount: 1, percentage: 20 },
    ]);
    expect(distribution.entries[0].member?.avatar_url).toBe('https://cdn.example.com/alice.png');
  });

  it('allocates rounding remainders so displayed percentages total 100', () => {
    const distribution = buildTaskOwnerDistribution(
      [
        { owner_member_ids: ['member-a'] },
        { owner_member_ids: ['member-b'] },
        { owner_member_ids: [] },
      ],
      members,
    );

    expect(distribution.entries.map((entry) => entry.percentage)).toEqual([34, 33, 33]);
    expect(distribution.entries.reduce((sum, entry) => sum + entry.percentage, 0)).toBe(100);
  });

  it('returns an empty distribution when the epic has no tasks', () => {
    expect(buildTaskOwnerDistribution([], members)).toEqual({
      entries: [],
      ownerCount: 0,
      totalAssignments: 0,
      totalTasks: 0,
    });
  });
});

describe('TaskOwnerDistribution', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  it('aligns labels and segments, renders real avatars, and keeps unassigned gray', () => {
    act(() => {
      root.render(
        <TooltipProvider>
          <TaskOwnerDistribution
            tasks={[
              { owner_member_ids: ['member-a'] },
              { owner_member_ids: ['member-a'] },
              { owner_member_ids: ['member-b'] },
              { owner_member_ids: [] },
            ]}
            members={members}
          />
        </TooltipProvider>,
      );
    });

    const labels = container.querySelector<HTMLElement>('[data-testid="task-owner-labels"]');
    const bar = container.querySelector<HTMLElement>('[data-testid="task-owner-bar"]');
    const avatar = container.querySelector<HTMLElement>('[data-testid="owner-avatar"]');
    const assigned = container.querySelector<HTMLElement>('[data-testid="task-owner-segment-member-a"]');
    const secondAssigned = container.querySelector<HTMLElement>('[data-testid="task-owner-segment-member-b"]');
    const unassigned = container.querySelector<HTMLElement>('[data-testid="task-owner-segment-__unassigned__"]');

    expect(labels?.style.gridTemplateColumns).toBe('2fr 1fr 1fr');
    expect(bar?.style.gridTemplateColumns).toBe(labels?.style.gridTemplateColumns);
    expect(bar?.getAttribute('aria-label')).toBe('Alice Johnson 50%, Bob Smith 25%, Unassigned 25%');
    expect(avatar?.dataset.avatarName).toBe('Alice Johnson');
    expect(avatar?.dataset.avatarUrl).toBe('https://cdn.example.com/alice.png');
    expect(assigned?.className).toMatch(/bg-\[#(?:1b7f4e|4a9bd8|c98a3e|b8614a|6f8f55|7b6f9b|3f8c85)\]/);
    expect(secondAssigned?.className).toMatch(/bg-\[#(?:1b7f4e|4a9bd8|c98a3e|b8614a|6f8f55|7b6f9b|3f8c85)\]/);
    expect(unassigned?.className).toContain('bg-muted-foreground/35');
    expect(container.textContent).toContain('2 tasks · 50%');
    expect(container.textContent).toContain('1 task · 25%');
  });

  it('shows a purposeful empty state when there are no tasks', () => {
    act(() => {
      root.render(
        <TooltipProvider>
          <TaskOwnerDistribution tasks={[]} members={members} />
        </TooltipProvider>,
      );
    });

    expect(container.textContent).toContain('Add tasks to see ownership.');
    expect(container.querySelector('[data-testid="task-owner-bar"]')).toBeNull();
  });
});

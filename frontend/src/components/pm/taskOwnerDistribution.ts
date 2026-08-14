import { findAssignableMember, formatAssignableMemberName } from '@/lib/assignableMembers';
import type { Task } from '@/lib/pmTypes';
import type { AssignableMember } from '@/lib/types';

export type TaskOwnerInput = Pick<Task, 'owner_member_ids'>;

export interface TaskOwnerDistributionEntry {
  id: string;
  name: string;
  taskCount: number;
  percentage: number;
  kind: 'member' | 'unknown' | 'unassigned';
  member?: AssignableMember;
}

export interface TaskOwnerDistributionData {
  entries: TaskOwnerDistributionEntry[];
  ownerCount: number;
  totalAssignments: number;
  totalTasks: number;
}

function allocateRoundedPercentages(counts: number[], total: number): number[] {
  if (total <= 0) return counts.map(() => 0);

  const exact = counts.map((count) => (count / total) * 100);
  const roundedDown = exact.map(Math.floor);
  let remaining = 100 - roundedDown.reduce((sum, value) => sum + value, 0);
  const remainderOrder = exact
    .map((value, index) => ({ index, remainder: value - roundedDown[index] }))
    .sort((a, b) => b.remainder - a.remainder || a.index - b.index);

  for (let index = 0; index < remainderOrder.length && remaining > 0; index += 1) {
    roundedDown[remainderOrder[index].index] += 1;
    remaining -= 1;
  }

  return roundedDown;
}

export function buildTaskOwnerDistribution(
  tasks: TaskOwnerInput[],
  members: AssignableMember[],
): TaskOwnerDistributionData {
  const ownerCounts = new Map<string, Omit<TaskOwnerDistributionEntry, 'percentage'>>();
  let unassignedTaskCount = 0;

  for (const task of tasks) {
    const canonicalOwnerIds = new Set<string>();

    for (const ownerKey of new Set(task.owner_member_ids ?? [])) {
      const member = findAssignableMember(members, ownerKey);
      const canonicalId = member?.id ?? `unknown:${ownerKey}`;
      if (canonicalOwnerIds.has(canonicalId)) continue;
      canonicalOwnerIds.add(canonicalId);

      const existing = ownerCounts.get(canonicalId);
      if (existing) {
        existing.taskCount += 1;
        continue;
      }

      ownerCounts.set(canonicalId, {
        id: canonicalId,
        name: member ? formatAssignableMemberName(member) : 'Unknown owner',
        taskCount: 1,
        kind: member ? 'member' : 'unknown',
        member,
      });
    }

    if (canonicalOwnerIds.size === 0) {
      unassignedTaskCount += 1;
    }
  }

  const owners = Array.from(ownerCounts.values())
    .sort((a, b) => b.taskCount - a.taskCount || a.name.localeCompare(b.name));
  const unassigned = unassignedTaskCount > 0
    ? [{
        id: '__unassigned__',
        name: 'Unassigned',
        taskCount: unassignedTaskCount,
        kind: 'unassigned' as const,
      }]
    : [];
  const entriesWithoutPercentages = [...owners, ...unassigned];
  const totalAssignments = entriesWithoutPercentages.reduce((sum, entry) => sum + entry.taskCount, 0);
  const percentages = allocateRoundedPercentages(
    entriesWithoutPercentages.map((entry) => entry.taskCount),
    totalAssignments,
  );

  return {
    entries: entriesWithoutPercentages.map((entry, index) => ({
      ...entry,
      percentage: percentages[index],
    })),
    ownerCount: owners.length,
    totalAssignments,
    totalTasks: tasks.length,
  };
}

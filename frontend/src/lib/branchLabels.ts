export const BASE_BRANCH_TOKEN = '{base_branch}';
export const TASK_BRANCH_TOKEN = '{task_branch}';

function trim(value?: string) {
  return value?.trim() ?? '';
}

export function repositoryDefaultBranchLabel(defaultBranch?: string) {
  const branch = trim(defaultBranch);
  return `Repository default branch${branch ? ` (${branch})` : ''}`;
}

export function taskBranchOptionLabel(taskBranch?: string) {
  const branch = trim(taskBranch);
  return branch ? `Task branch (${branch})` : '';
}

export function describeRunBranchOverrides(baseBranch?: string, taskBranch?: string) {
  const parts: string[] = [];
  const resolvedBaseBranch = trim(baseBranch);
  const resolvedTaskBranch = trim(taskBranch);
  if (resolvedBaseBranch) parts.push(`base ${resolvedBaseBranch}`);
  if (resolvedTaskBranch) parts.push(`task ${resolvedTaskBranch}`);
  return parts.length ? ` (${parts.join(' · ')})` : '';
}

export function describeMergeDestination(branch?: string) {
  const resolvedBranch = trim(branch);
  if (!resolvedBranch || resolvedBranch === BASE_BRANCH_TOKEN) {
    return "the task's base branch";
  }
  return resolvedBranch;
}

export function describeMergeInto(branch?: string) {
  return `Merge into ${describeMergeDestination(branch)}`;
}

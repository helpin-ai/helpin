import type { CapabilitiesResponse } from '@/lib/capabilityTypes';
import type { SetupTask } from '@/lib/setupTypes';
import { findCapability } from './capabilityPresentation';

/** Setup catalog tasks whose value depends on the workspace AI provider. */
export const AI_DEPENDENT_TASK_KEYS: ReadonlySet<string> = new Set([
  'support.ai_agent_activated',
  'support.coverage_fix_applied',
  'internal_docs.agent_connected',
  'internal_docs.agent_succeeded',
  'product.agent_result_used',
  'automation.first_assisted_value',
  'automation.triggered_value',
  'automation.reliable_unattended_value',
  'automation.custom_agent_succeeded',
]);

/** Setup catalog tasks that need the GitHub App to exist and be installed first. */
export const GITHUB_APP_TASK_KEYS: ReadonlySet<string> = new Set(['product.repository_ready']);

export type SetupTaskRequirementKind = 'ai' | 'github';

function actionable(task: SetupTask) {
  return task.status === 'available' || task.status === 'needs_attention' || task.status === 'unable_to_verify';
}

/**
 * Connecting AI is a workspace step only on Community; other editions provide
 * AI from the platform, so they never ask for it.
 */
export function aiNeedsConnection(capabilities: CapabilitiesResponse | undefined) {
  return capabilities?.edition === 'community' && findCapability(capabilities, 'ai_chat')?.status === 'needs_setup';
}

/**
 * GitHub steps the repository task cannot take on its own: creating or
 * installing the App. Selecting repositories is the task's own action.
 */
function gitHubAppNeeded(capabilities: CapabilitiesResponse | undefined) {
  const github = findCapability(capabilities, 'github');
  return github?.status === 'needs_setup' && github.action?.path !== 'settings/repositories';
}

/**
 * Which inline prerequisite, if any, each task in a journey shows. The AI
 * block appears once per journey, on the first actionable AI task, so the same
 * fix is not repeated down the list.
 */
export function journeyTaskRequirements(tasks: SetupTask[], capabilities: CapabilitiesResponse | undefined) {
  const requirements = new Map<string, SetupTaskRequirementKind>();
  if (aiNeedsConnection(capabilities)) {
    const first = tasks.find((task) => actionable(task) && AI_DEPENDENT_TASK_KEYS.has(task.key));
    if (first) requirements.set(first.key, 'ai');
  }
  if (gitHubAppNeeded(capabilities)) {
    for (const task of tasks) {
      if (actionable(task) && GITHUB_APP_TASK_KEYS.has(task.key)) requirements.set(task.key, 'github');
    }
  }
  return requirements;
}
